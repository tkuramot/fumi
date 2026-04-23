# アーキテクチャ

## 1. リポジトリ構成

spec §12 のモノレポ方針を踏襲し、Go とフロントエンドを同一 repo に置く。

```
fumi/
├── go.mod                       # Go module (module path: github.com/tkuramot/fumi)
├── go.sum
├── host/
│   └── main.go                  # fumi-host エントリポイント
├── cli/
│   ├── main.go                  # fumi エントリポイント (urfave/cli/v2 App)
│   └── internal/cmd/            # サブコマンドを *cli.Command で定義
├── internal/                    # host / cli で共有する Go コード
│   ├── config/                  # ~/.config/fumi/config.toml ローダ
│   ├── store/                   # ストアパス解決・frontmatter parser・セキュリティ検証
│   ├── protocol/                # Native Messaging 4B LE + JSON エンコーダ/デコーダ
│   ├── runner/                  # runScript のコアロジック (host と `fumi scripts run` で共有)
│   └── manifest/                # Chrome Native Messaging manifest の生成・配置 (fumi setup)
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

- `internal/` は Go のアクセス制御で `host/` と `cli/` からのみ import 可能。外部への API 公開は行わない。
- `cli/` は `github.com/urfave/cli/v2` で `App.Commands` + `Subcommands` により `fumi <resource> <verb>` を表現。サブコマンドごとに `cli/internal/cmd/*.go` で `*cli.Command` factory を持つ (CLI 詳細は [cli.md](./cli.md))。
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
   │            │  {op:"getActions"} │                   │
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
- `userJs` は `code` フィールドとして、`getActions` レスポンスのまま挿入。
- 既存登録との差分は **常に全置換** する (`unregister` → `register`)。最小差分更新は実装しない (ユーザー数規模で問題にならない)。

### 3.2. runScript 呼び出し

```
User Script           Service Worker           fumi-host             External Script
    │                       │                     │                         │
    │  fumi.run("x.sh", p)  │                     │                         │
    │─chrome.runtime.send──▶│                     │                         │
    │  Message({            │                     │                         │
    │    op:"runScript",    │                     │                         │
    │    scriptPath, payload│                     │                         │
    │  })                   │                     │                         │
    │                       │─sendNativeMessage──▶│                         │
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
    │           │                    │ fumi.contextMenus.register({id,title,onClicked})
    │           │◀─ sendMessage ──── │                              │
    │           │  {op:"contextMenus │                              │
    │           │   .register",id,..}│                              │
    │           │─chrome.contextMenus.create/update (id冪等)        │
    │           │                                                   │
    │           │                                                   │
    │           │ onClicked(info, tab)                              │
    │           │◀──────────────────────────────────────────────────│
    │           │─chrome.tabs.sendMessage(tab.id, {kind:"ctxDispatch│
    │           │   ", menuId,info,tab}) ──▶ User Script で onClicked 実行│
```

- SW 側は `Map<menuId, { tabId, registeredAt }>` を保持し、同 id の再登録で上書き (spec §5.1.4 冪等性)。
- dispatch 先は `onClicked` の第 2 引数 `tab` そのまま。User Script ↔ SW は `chrome.tabs.sendMessage` (frameId 省略 = top frame)。

## 4. プロセスライフサイクル

| プロセス | 起動契機 | 終了契機 | 状態 |
|---|---|---|---|
| Service Worker | Extension 有効化 / メッセージ着信 | Chrome のアイドル判定 | `chrome.storage` キャッシュ以外は揮発 |
| User Script | match するページのロード | タブ遷移 / ページ閉 | クロージャのみ、永続なし |
| fumi-host | `sendNativeMessage` ごと | 応答 1 回送信後 exit | ディスクから毎回読み直す |
| External Script | `runScript` 操作ごと | 自前 exit / timeout → SIGTERM+KILL | 呼び出しごとに独立 |

**設計上の帰結**:
- SW が消えても Action 登録は `chrome.userScripts` 側に永続化されている (MV3 仕様) ため、再起動時の `getActions` は必須ではない。ただし **起動直後に 1 回は走らせる** (ストアが編集されている可能性があるため)。
- `chrome.contextMenus` は MV3 では Service Worker 再起動時に **自動では復元されない** ので、SW 起動時に対象タブの User Script が再度 `fumi.contextMenus.register` を呼ぶ必要がある。ユーザーには「タブをリロードすれば復活」とドキュメントで告知する。

## 5. エラーハンドリングの階層

| 層 | 扱い |
|---|---|
| External Script の非 0 exit | 正常系。`fumi.run` は resolve し、ユーザーコードが `exitCode` で分岐 |
| External Script の stdout/stderr が 1MB 超 | Host が `error: { code: "OUTPUT_TOO_LARGE" }` で応答 → `fumi.run` は reject |
| パス検証失敗 (scripts/ 外) | Host が `error: { code: "INVALID_SCRIPT_PATH" }` → reject |
| timeout | Host が SIGTERM→SIGKILL、`error: { code: "TIMEOUT" }` → reject |
| Host 起動失敗 (manifest 未配置) | Chrome が `chrome.runtime.lastError` を立てる。SW は Popup に「Host 未接続」として反映 |
| getActions で 1MB 超 | `error: { code: "ACTIONS_TOO_LARGE" }` → Popup にエラー表示、登録は前回状態維持 |

エラーコードの一覧は [protocol.md](./protocol.md) §3。
