# アーキテクチャ

## 1. リポジトリ構成

spec §12 のモノレポ方針を踏襲し、Go とフロントエンドを同一 repo に置く。

```
fumi/
├── go.mod                       # Go module (module path: github.com/tkuramot/fumi)
├── go.sum
├── cmd/
│   ├── fumi/                    # fumi バイナリ (package main) — CLI 専用ロジック同居
│   │   ├── main.go              # urfave/cli/v2 App 組み立て
│   │   ├── setup.go             # fumi setup
│   │   ├── uninstall.go         # fumi uninstall
│   │   ├── doctor.go            # fumi doctor
│   │   ├── actions.go           # fumi actions ...
│   │   ├── scripts.go           # fumi scripts ...
│   │   ├── manifest.go          # Native Messaging manifest の生成・配置
│   │   └── constants.go         # ldflags で注入される Extension ID / host binary path
│   └── fumi-host/               # fumi-host バイナリ (package main) — dispatch 同居
│       ├── main.go              # stdin/stdout ロック + run() 呼び出し
│       ├── dispatch.go          # op → handler ルーティング
│       ├── actions_list.go     # actions/list ハンドラ
│       └── scripts_run.go      # scripts/run ハンドラ
├── internal/                    # 両バイナリで共有する Go コード
│   ├── config/                  # ~/.config/fumi/config.toml ローダ
│   ├── store/                   # ストアパス解決・frontmatter parser・セキュリティ検証
│   ├── protocol/                # Native Messaging 4B LE + JSON エンコーダ/デコーダ
│   └── runner/                  # scripts/run のコアロジック (host と `fumi scripts run` で共有)
├── extension/
│   ├── package.json             # dev deps: typescript, @types/chrome のみ
│   ├── tsconfig.json
│   ├── public/
│   │   ├── manifest.json        # Chrome Extension manifest (静的 JSON)
│   │   └── popup.html
│   └── src/
│       ├── background/          # Service Worker
│       ├── popup/               # Popup UI
│       ├── user-script/         # fumi.* プレリュード (import 禁止・単一ファイル)
│       └── shared/              # 型・メッセージスキーマ
├── samples/
│   ├── actions/
│   └── scripts/
├── docs/
│   ├── spec.md
│   └── design/
└── README.md
```

- `internal/` は Go のアクセス制御でモジュール内からのみ import 可能。外部への API 公開は行わない。**本当に共有されるコードのみ**をここに置く (`config` / `store` / `protocol` / `runner`)。
- バイナリ専用ロジックは `cmd/<binary>/` に `package main` として同居させる。`cli` 専用の Command factory や manifest 配置ロジックは `cmd/fumi/` 配下、host 専用の dispatch / handler は `cmd/fumi-host/` 配下。これにより「どこに置くか」が機械的に決まる (**1 バイナリ専用なら cmd 配下、複数で共有するなら internal**)。
- `cmd/fumi/` は `github.com/urfave/cli/v2` で `App.Commands` + `Subcommands` により `fumi <resource> <verb>` を表現。サブコマンドごとに `cmd/fumi/<cmd>.go` に小文字スタートの factory (`setupCmd() *cli.Command` 等) を持つ (CLI 詳細は [cli.md](./cli.md))。
- Extension 側は `tsc` のみで build。バンドラ / 外部ビルドツール不要 (詳細 [extension.md](./extension.md))。Go 側の artifact とはファイルパスでのみ接続される。
- 依存ライブラリ採用の原則は [README.md §依存ポリシー](./README.md#依存ポリシー) に集約。

## 2. プロセス境界

```
┌──────────────────────────────────── Chrome ────────────────────────────────────┐
│                                                                                │
│  ┌─────────────┐      ┌──────────────────────────┐      ┌───────────────────┐  │
│  │  Web Page   │◀───▶│ User Script (USER_SCRIPT │──────│  Service Worker   │  │
│  │   (DOM)     │      │ world, fumi.* プレリュ   │      │  (background.ts)  │  │
│  └─────────────┘      │ ード + ユーザー JS)      │      └─────────┬─────────┘  │
│                       └──────────────────────────┘                │            │
│                                                                   │            │
│                                                                   │ sendNative │
│                                                                   │  Message   │
└───────────────────────────────────────────────────────────────────┼────────────┘
                                                                    │
                                                   ┌────────────────▼────────────┐
                                                   │    fumi-host (短命)         │
                                                   │    stdin/stdout (Native     │
                                                   │    Messaging protocol)      │
                                                   └──────────┬──────────────────┘
                                                              │ spawn (execve)
                                                              │ stdin: payload JSON
                                                              ▼
                                                    ┌──────────────────────┐
                                                    │  External Script     │
                                                    │  (scripts/... 内)    │
                                                    └──────────────────────┘
```

- **Web Page ↔ User Script**: 同一プロセス (Renderer)、別 world。User Script から DOM は操作できるが page JS globals にはアクセス不可 (USER_SCRIPT world の性質)。
- **User Script ↔ Service Worker**: `chrome.runtime.sendMessage` / `onMessage`。MV3 Service Worker はアイドル時に停止されるが、メッセージ着信で自動復帰する前提。
- **Service Worker ↔ fumi-host**: `chrome.runtime.sendNativeMessage`。リクエストごとに Chrome が `fumi-host` を spawn し、応答を受け取って自動で kill。常駐接続 (`connectNative`) は採用しない (spec §5.2.1)。
- **fumi-host ↔ External Script**: Go の `os/exec` で直接 spawn (シェル非介在)、stdin に payload JSON を 1 回流して close、exit まで待機。

## 3. 主要シーケンス

### 3.1. 起動時 (アクション登録)

```
Chrome起動  SW(Background)         fumi-host           ストア
   │            │                    │                   │
   │─startup───▶│                    │                   │
   │            │─sendNativeMessage──│                   │
   │            │  {jsonrpc:"2.0",   │                   │
   │            │   method:          │                   │
   │            │    "actions/list"} │                   │
   │            │                    │─actions/ 列挙────▶│
   │            │                    │◀─.js files─────── │
   │            │                    │─frontmatter parse │
   │            │◀── Action[] ─── ── │                   │
   │            │   (exit)           │                   │
   │            │                                        │
   │            │─chrome.userScripts.register(code:      │
   │            │    [preludeJs, userJs], matches, ...)  │
```

- `preludeJs` は Extension にバンドルされる固定の `fumi.*` 実装 (Service Worker への `sendMessage` ラッパ)。
- `userJs` は `code` フィールドとして、`actions/list` レスポンスのまま挿入。
- 既存登録との差分は **常に全置換** する (`unregister` → `register`)。最小差分更新は実装しない (ユーザー数規模で問題にならない)。

### 3.2. scripts/run 呼び出し

```
User Script           Service Worker           fumi-host             External Script
    │                       │                     │                         │
    │  fumi.run("x.sh", p)  │                     │                         │
    │─chrome.runtime.send──▶│                     │                         │
    │  Message({            │                     │                         │
    │    kind:"scripts/run",│                     │                         │
    │    scriptPath, payload│                     │                         │
    │  })                   │                     │                         │
    │                       │─JSON-RPC 2.0 で─────▶│                         │
    │                       │  sendNativeMessage   │                         │
    │                       │  {jsonrpc:"2.0",     │                         │
    │                       │   method:"scripts/run│                         │
    │                       │   params:{...}}      │                         │
    │                       │                     │─path 検証 (realpath+lstat)
    │                       │                     │─exec.Command(no shell)─▶│
    │                       │                     │─stdin: JSON, close      │
    │                       │                     │                         │─ 処理 ─
    │                       │                     │◀── stdout/stderr/exit ──│
    │                       │◀── result JSON ──── │                         │
    │                       │   (exit)            │                         │
    │◀─ Promise resolve ─── │                     │                         │
    │   {exitCode,stdout,...} │                                             │
```

### 3.3. ContextMenu 登録とディスパッチ

```
対象URLを開く   SW              User Script                  ContextMenu Click
    │           │                    │                              │
    │──────────▶│                    │                              │
    │           │─chrome.userScripts │                              │
    │           │  .execute()        │                              │
    │           │                    │                              │
    │           │                    │ fumi.contextMenus.create({id,title,onClicked})
    │           │◀─ sendMessage ──── │                              │
    │           │  {kind:"contextMenu│                              │
    │           │   s/create",...}   │                              │
    │           │─chrome.contextMenus.create (重複 id はそのままエラー)│
    │           │                                                   │
    │           │                                                   │
    │           │ onClicked(info, tab)                              │
    │           │◀──────────────────────────────────────────────────│
    │           │─chrome.tabs.sendMessage(tab.id, {kind:"ctxDispatch│
    │           │   ", menuId,info,tab}) ──▶ User Script で onClicked 実行│
```

- SW 側は冪等化しない (spec §5.1.4: chrome.* セマンティクス完全一致)。重複登録を避けたい action は `remove` → `create` イディオムを使う。dispatch 先は `onClicked` の `tab` 引数から直接得るため registry Map は持たない。
- dispatch 先は `onClicked` の第 2 引数 `tab` そのまま。User Script ↔ SW は `chrome.tabs.sendMessage` (frameId 省略 = top frame)。

## 4. プロセスライフサイクル

| プロセス | 起動契機 | 終了契機 | 状態 |
|---|---|---|---|
| Service Worker | Extension 有効化 / メッセージ着信 | Chrome のアイドル判定 | `chrome.storage` キャッシュ以外は揮発 |
| User Script | match するページのロード | タブ遷移 / ページ閉 | クロージャのみ、永続なし |
| fumi-host | `sendNativeMessage` ごと | 応答 1 回送信後 exit | ディスクから毎回読み直す |
| External Script | `scripts/run` 操作ごと | 自前 exit / timeout → SIGTERM+KILL | 呼び出しごとに独立 |

**設計上の帰結**:
- SW が消えても Action 登録は `chrome.userScripts` 側に永続化されている (MV3 仕様) ため、再起動時の `actions/list` は必須ではない。ただし **起動直後に 1 回は走らせる** (ストアが編集されている可能性があるため)。
- `chrome.contextMenus` は MV3 では Service Worker 再起動時に **自動では復元されない** ので、SW 起動時に対象タブの User Script が再度 `fumi.contextMenus.create` を呼ぶ必要がある。ユーザーには「タブをリロードすれば復活」とドキュメントで告知する。

## 5. エラーハンドリングの階層

| 層 | 扱い |
|---|---|
| External Script の非 0 exit | 正常系。`fumi.run` は resolve し、ユーザーコードが `exitCode` で分岐 |
| External Script の stdout/stderr がサイズ上限超 | Host が `error.data.fumiCode: "EXEC_OUTPUT_TOO_LARGE"` (code -33031) で応答 → `fumi.run` は reject |
| パス検証失敗 (scripts/ 外) | Host が `error.data.fumiCode: "SCRIPT_INVALID_PATH"` (code -33020) → reject |
| timeout | Host が SIGTERM→SIGKILL、`error.data.fumiCode: "EXEC_TIMEOUT"` (code -33030) → reject |
| Host 起動失敗 (manifest 未配置) | Chrome が `chrome.runtime.lastError` を立てる。SW は Popup に「Host 未接続」として反映 |
| actions/list で 1MB 超 | `error.data.fumiCode: "STORE_ACTIONS_TOO_LARGE"` (code -33010) → Popup にエラー表示、登録は前回状態維持 |

エラーコードの一覧は [protocol.md](./protocol.md) §3。
