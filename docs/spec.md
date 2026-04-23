# 仕様書: ブラウザ × ホストマシン連携 Chrome Extension

> 本ドキュメントは要件レベルの仕様。技術的な実装(言語・ライブラリ・データ構造等)は設計フェーズで決定する。
>
> 文中の **【要確認 X】** マーカーは未確定事項。末尾 §13 に集約。

---

## 1. 概要

Chrome Extension からブラウザ上で任意の JavaScript を実行し、その結果や任意のペイロードを、Native Messaging 経由でホストマシン上の任意の実行ファイルに引き渡すシステム。ブラウザ上の情報・操作とローカルの CLI / スクリプト / ツールチェーンを接続する「橋渡し」を目的とする。

### 1.1. コンポーネント

- **Chrome Extension**: Host から取得したアクション JS を `chrome.userScripts` に登録し、アクションからの External Script 実行要求を Host にリレーする薄いクライアント。
- **Native Messaging Host**: Extension からの 2 種類のメッセージ(アクション取得 / External Script 実行)に応答するネイティブプロセス。
- **External Script**: ホストマシン上の任意の実行ファイル。Host から起動され、Extension が送ったデータを stdin で受け取る。

```
[Web Page] ⇄ [User Script] ⇄ [Extension Service Worker] ⇄ [Native Messaging Host] ⇄ [External Script]
                                                               ⇣
                                                    [Host 上のストア: アクション JS + External Script]
```

### 1.2. 設計原則

- **Single source of truth はホストマシン上のファイルシステム**。Extension は純粋な「ランナー」であり状態を持たない。
- **Extension → Host の書き込み操作は存在しない**。Extension は Host からコンテンツを取得するのみ。
- スクリプトの編集・管理はユーザーが普段使うエディタ(VS Code, Vim 等)で直接ファイルシステム上で行う。git 管理・バックアップも通常のファイルとして扱う。

---

## 2. 目的 / 非目的

### 2.1. 目的

- ブラウザ上の DOM / ページ状態を JS で読み書きし、そこからホスト側の任意実行ファイルをトリガーできる。
- アクション JS と External Script はすべてホスト側ファイルとして管理する(git 管理・エディタ編集前提)。
- 開発者向けに、最小摩擦なブラウザ ↔ ホスト連携ユーティリティを提供する。

### 2.2. 想定ユーザー

**開発者向けに配布**。非開発者向けの UX 配慮(GUI 編集画面、親切エラー表示)よりも、設定の可視性・柔軟性・ハック容易性を優先する。Chrome Web Store 公開を最初から目指し、通常インストールと開発時の unpacked ロードの両方を同一仕様でサポートする。

---

## 3. 用語

| 用語 | 定義 |
|---|---|
| アクション (Action) | ページ上で実行される JS スニペット + メタデータ。ストアにファイルとして保存される。 |
| User Script | `chrome.userScripts` API で登録・実行される、ページ内で動くスクリプト。アクションの実体。 |
| External Script | Host から起動される、任意言語の実行ファイル。ストアに保存される。 |
| Host (Native Messaging Host) | ブラウザから起動され stdin/stdout で通信するネイティブプロセス。アクション提供と External Script 起動のみを責務とする。 |
| ストア (Store) | ホスト上でアクション JS と External Script を保管するディレクトリ階層。 |

---

## 4. ユーザーモデル / ユースケース

### 4.1. メンタルモデル

1. ユーザーは自分のエディタでストア内にアクション JS と External Script を書く。
2. アクション JS 内で `fumi.run("script-name", payload)` で External Script を呼び出し、トリガー登録も `fumi.*` ヘルパーで JS 上から宣言する。
3. Extension を開く / 再読み込みすると、Host から最新のアクション一覧を取得して `chrome.userScripts` に反映する。
4. ユーザーが定義したトリガーでアクションが発火し、DOM 操作・値取得・外部スクリプト起動が行われる。

### 4.2. 代表ユースケース

- ページ上の選択テキストを取得し、ローカルの要約 CLI に渡して結果をページに表示。
- 開いている GitHub PR の URL を取得し、ローカルのエディタを開くスクリプトに渡す。
- ページに「保存」ボタンを挿入し、`notes.md` に追記するスクリプトを実行して成否をトースト表示。
- 特定ドメインを開いた瞬間に自動実行し、ホスト側にログを記録。

---

## 5. システム構成要素別の要件

### 5.1. Chrome Extension

#### 5.1.1. マニフェスト / 権限

Manifest V3。必要権限:

```jsonc
{
  "permissions": [
    "userScripts",      // chrome.userScripts API
    "nativeMessaging",  // Host 呼び出し
    "contextMenus",     // fumi.contextMenus.create
    "storage"           // Extension 側キャッシュ・ステータス
  ],
  "host_permissions": ["<all_urls>"]
}
```

- `commands` は使わない(動的登録不可のため、アクション側で `addEventListener('keydown', ...)` を書く前提)。
- `scripting` は使わない(User Script は USER_SCRIPT world 固定で MAIN world 用ブリッジ注入が不要)。
- `host_permissions: ["<all_urls>"]` を静的に要求。Web Store 審査用の説明:「任意サイトで動くユーザースクリプト拡張のため全 URL 権限が必要」。
- `chrome.userScripts` は Chrome の「開発者モード」有効化が前提(README で告知)。

#### 5.1.2. Extension UI

ツールバーアイコンから開く **Popup** のみ提供。表示内容:

- Host の接続状態(OK / エラー)
- 登録中のアクション数
- 最後の取得時刻・最後のエラー
- **「アクション再取得」ボタン**(`actions/list` 再実行 → `chrome.userScripts` 更新)

スクリプト編集・閲覧・CRUD 系 UI、DevTools Panel、`fumi.run` の結果自動表示トースト等は**提供しない**。編集はユーザーが自分のエディタでストアを直接操作する。

#### 5.1.3. アクションの登録と実行

- Extension 起動時および Popup「再取得」押下時に `actions/list` を呼び、結果を `chrome.userScripts.register` / `update` に反映する。
- **実行 World: USER_SCRIPT 固定**。ページから隔離され `chrome.runtime.sendMessage` を直接使える。ページの JS globals(React fiber 等)にはアクセスできないが、DOM 操作・イベントリスナ登録は可能。MAIN world 対応は初期版では提供しない。
- match / exclude 等のメタデータは、アクションファイル上部の **Tampermonkey 風フロントマター**(`// @key value` 連記)で宣言。Host が `actions/list` でメタデータ + 本体を parse して渡す。

#### 5.1.4. トリガー

**ユーザーがアクション内で命令的に登録する (Pattern A)**。Tampermonkey ライクに、アクション JS 本体内で `fumi.*` API を呼んでトリガー登録と DOM 操作を行う。

##### `fumi.*` API 命名規則

USER_SCRIPT world から直接触れない拡張専用 API(`chrome.contextMenus` 等)は `fumi.*` でラップする。規則:

1. **Namespace**: `fumi.<chromeNamespace>.<method>` の形で `chrome.*` の構造を写す(例: `chrome.contextMenus.*` → `fumi.contextMenus.*`)。
2. **メソッド名は chrome のメソッド名と完全一致させる**。意訳・差し替えは禁止 (例: `chrome.contextMenus.create` → `fumi.contextMenus.create`、`register` 等への変更不可)。「create + イベントリスナのバインド」のように複数 API を 1 呼び出しにまとめる場合は、**主役の chrome メソッド名 (= `create`) をそのまま採用** し、イベントハンドラは引数オブジェクトに同梱する形で表現する (規則 4)。
3. **プロパティ名・型は chrome の `CreateProperties` / 引数オブジェクトをそのまま参照** する(例: `id`, `title`, `contexts`)。リネーム・別名・型変換は禁止。
4. **ハンドラは chrome のイベント名をそのままプロパティ名に使う**(例: `onClicked` は `chrome.contextMenus.onClicked` に対応)。シグネチャも chrome のイベントと一致させる。chrome.* 本来は `addListener` で別途登録する形だが、`fumi.*` では create 引数に同梱する API 形にしてよい (これは prelude の便宜であり、ハンドラ名・シグネチャは chrome に従う)。
5. **スコープ**: MVP で必要なフィールドのみホワイトリスト。未知プロパティは Service Worker 側で捨てる。捨てる側は subset 化のみ行い、リネーム禁止。
6. **例外**: `fumi.run` は `chrome.*` ラッパーではない本機能 API のためルート直下に置く。これだけが命名規則の対象外。

対象とする chrome.* API は現時点では `chrome.contextMenus` のみ。`chrome.notifications` / `chrome.storage` / `chrome.tabs` 等は MVP では非提供で、必要になれば上記規則で追加する。

##### 基盤 API

- **User Script ↔ Service Worker 通信**: `chrome.runtime.sendMessage` を直接利用。`fumi.*` 内部でラップ。
- **`fumi.*` ヘルパー**:
  - `fumi.run(scriptName, payload)` — External Script を起動。`Promise<{ exitCode, stdout, stderr, durationMs }>` を返す。**本 API のメイン**。
  - `fumi.contextMenus.create({ id, title, contexts?, onClicked })` — `chrome.contextMenus.create` + `chrome.contextMenus.onClicked` のバインドを Service Worker 経由で動的登録。引数名は `chrome.contextMenus.CreateProperties` の subset。`onClicked` のシグネチャは `(info, tab) => void` で `chrome.contextMenus.onClicked` と一致。**重複 id でのエラー挙動は chrome.* と同一**(下記イディオム参照)。
  - `fumi.contextMenus.remove(menuItemId)` — `chrome.contextMenus.remove` の薄ラッパ。存在しない id への remove は chrome 同様 reject する。
- **ショートカット**: アクション側で `document.addEventListener('keydown', ...)` を書く方式に委ねる(ページにフォーカスがある時のみ。ブラウザグローバルは提供しない)。
- **ページ内挿入 UI と DOM 操作**: `document.createElement` 等で要素を追加し、click ハンドラ内で `fumi.run` を await できる(User Script world のクロージャなので dispatch 不要)。
- **URL マッチでの自動起動**: フロントマター `// @match` で宣言。match 時に JS 本体がそのまま走る。

##### 同一アクション内で併用できる例

```js
// ==Fumi Action==
// @match https://github.com/*
// ==/Fumi Action==

const run = async () => {
  const { stdout } = await fumi.run('save-note.sh', {
    title: document.title,
    url: location.href,
    selection: String(window.getSelection()),
  });
  console.log('saved:', stdout);
};

// 1. ページにボタンを挿入
const btn = document.createElement('button');
btn.textContent = 'Save';
btn.onclick = run;
document.body.appendChild(btn);

// 2. ショートカットはアクション側で keydown を拾う
document.addEventListener('keydown', (e) => {
  if (e.ctrlKey && e.shiftKey && e.key === 'S') {
    e.preventDefault();
    run();
  }
});

// 3. コンテキストメニューは専用 API
// マッチページを開くたびに本コードが走るので、重複登録を避けるため remove → create
await fumi.contextMenus.remove('save-note').catch(() => {});  // 未登録は無視
await fumi.contextMenus.create({
  id: 'save-note',
  title: 'Save this page',
  contexts: ['page'],
  onClicked: (_info, _tab) => run(),
});
```

##### 冪等性 / dispatch 先

- `fumi.contextMenus.create` は match するページを開くたびに呼ばれる。**fumi は SW 側で冪等化しない** (chrome.* セマンティクス完全一致の方針)。重複登録を避けたい action は **`remove` → `create` のイディオム** を使う:
  ```js
  await fumi.contextMenus.remove('id').catch(() => {});  // 未登録は無視
  await fumi.contextMenus.create({ id: 'id', ... });
  ```
  - **設計判断の根拠**: SW 側で silent に冪等化すると (a) `fumi.contextMenus.create` のセマンティクスが chrome.* と乖離し命名規則 (§5.1.4) の精神を破る、(b) typo で既存 id とぶつかった時に silent overwrite してしまう、という二点で透明性を損なう。明示的な remove → create は chrome 拡張開発の標準イディオムであり、ボイラープレートは 1 行で済む。
- 発火時の dispatch 先は `chrome.contextMenus.onClicked` の第 2 引数 `tab`(右クリックしたタブ)をそのまま使うため、曖昧さはない。

#### 5.1.5. 戻り値の扱い

`fumi.run(...)` は双方向 RPC として動作し、`exitCode` / `stdout` / `stderr` / `durationMs` を Promise で返す。ユーザーは戻り値を `alert` / DOM 描画 / `console.log` 等で自由に扱える。デフォルトのトースト UI 等は提供しない **【要確認 J】**。

#### 5.1.6. ストレージ

`chrome.storage` には**キャッシュとステータス情報のみ**(最後の `actions/list` 結果、接続エラー情報等)を持つ。

### 5.2. Native Messaging Host

#### 5.2.1. 通信プロトコル

- Native Messaging プロトコル(stdin/stdout、4B リトルエンディアン長プレフィックス + JSON)に従う。
- **短命プロセス (`chrome.runtime.sendNativeMessage`)**: メッセージごとに spawn → 応答 → exit。push 通知は使わない。
- マニフェスト(`allowed_origins`、実行パス等)は `fumi setup` で配置(§11)。

#### 5.2.2. 対応 OS

**macOS のみ**。Linux / Windows はサポート対象外。

#### 5.2.3. Host の責務 (最小限)

以下の 2 種類の操作のみ実装する:

1. **`actions/list`**: ストアからアクション JS とメタデータを読み出して返す。
2. **`scripts/run`**: 指定された External Script を起動し、結果を返す。

> **CRUD 系 API は実装しない**(`listScripts` / `readScript` / `writeScript` / `deleteScript` / 設定の読み書き等)。攻撃面を最小化するための強い制約(詳細 §9)。

#### 5.2.4. `actions/list` の挙動

- `actions/` 配下の `.js` ファイルを列挙。
- 各ファイルを parse してフロントマター(match パターン・ID 等)と本体コードを抽出。
- 配列として Extension に返す。

##### フロントマター仕様 (Tampermonkey 風)

ファイル先頭の `// ==Fumi Action==` と `// ==/Fumi Action==` で挟まれたブロック内で、1 行 1 指令で宣言する。

```js
// ==Fumi Action==
// @id         save-note
// @match      https://github.com/*
// @match      https://*.github.com/*
// @exclude    https://github.com/settings/*
// ==/Fumi Action==
```

対応キー(初期版で確定):

| キー | 用途 | 複数指定 |
|---|---|---|
| `@id` | アクション識別子。省略時はファイル名から導出 | × |
| `@match` | match パターン | ◯ |
| `@exclude` | exclude パターン | ◯ |

World は USER_SCRIPT 固定、`runAt` は `chrome.userScripts` の既定値に従う。`@world` / `@runAt` / `@name` / `@description` / `@version` / `@noframes` 等は初期版では採用しない。

**サイズ上限**: レスポンス全体が Native Messaging の 1MB 上限を超える場合は `STORE_ACTIONS_TOO_LARGE` (code -33010) を返す。分割返却は初期版では実装しない(アクション数が多すぎたらファイル分割を見直す指針)。

#### 5.2.5. `scripts/run` の挙動

**Extension から受け取るフィールド**:

- `scriptPath` — `scripts/` 配下の相対パス。**拡張子込み、サブディレクトリ OK**。例: `"summarize.sh"`, `"tools/open-in-editor.py"`。
- `payload` — 任意の JSON 値。
- `timeoutMs`(任意)— 省略時は Host 設定の `default_timeout_ms`(既定 30000)。

**Host 内の処理**:

1. パスは必ず `scripts/` 配下として解釈(絶対パス・`..` を含む値は拒否)。
2. `realpath` で解決し、解決後も `scripts/` 配下であることを検証。
3. `lstat` で通常ファイルかつシンボリックリンクでないことを確認。
4. **シェルを介さず直接 spawn**(`execve` 系)。
5. ペイロードを JSON として **stdin に 1 回流して close**。
6. stdout / stderr を収集、exit を待機。

**起動時のパラメータ**:

- **cwd**: スクリプトを置いたディレクトリ(例: `scripts/tools/open-in-editor.py` → `scripts/tools/`)。同居リソースを相対パスで参照しやすくするため。
- **環境変数**: Host の env を全継承 + 以下を追加。
  - `FUMI_STORE` — ストアルート (`~/.config/fumi`)
  - `FUMI_ACTION_ID` — 呼び出し元アクション id(分かる場合)
  - `FUMI_TAB_URL` — 呼び出し時のタブ URL(Extension が送っていれば)
- **タイムアウト**: 既定 30 秒。`timeoutMs` で override 可。超過時は SIGTERM →(数百 ms 後)SIGKILL。
- **並列実行**: 上限なし(OS スケジューラ任せ)。
- **キャンセル**: 初期版では実装しない(timeout で打ち切り)。

**返却情報**: `exitCode`, `stdout`, `stderr`, `durationMs`, `error`(Host 側エラー)。

**stdout / stderr のサイズ上限**: Native Messaging の 1MB 制約からレスポンス全体の上限を逆算し、stdout ≤ 768 KiB / stderr ≤ 128 KiB を既定とする (design/protocol.md §4.2)。超過時は `EXEC_OUTPUT_TOO_LARGE` (code -33031) で応答。truncate はしない(部分出力で誤動作するのを避けるため)。大量出力が必要なスクリプトは `$TMPDIR` に書いてパスだけ返す運用をドキュメント化。

#### 5.2.6. ストアのレイアウト (初期案)

```
~/.config/fumi/                       <- ストアルート (既定)
  actions/                            <- アクション JS。chrome.userScripts 用
    my-action.js
  scripts/                            <- External Script。scripts/run で起動される
    summarize.sh
    open-in-editor.py
```

- `actions/` と `scripts/` を**物理的に分ける**ことで、`scripts/run` の対象を `scripts/` 配下に限定でき §9 の検証が単純になる。
- ログディレクトリはストア内に置かない。必要ならスクリプト側で `$TMPDIR` 等に保存する(標準機能としては提供しない)。

#### 5.2.7. Host の設定

Host の挙動は TOML ファイル `~/.config/fumi/config.toml` で制御する。

```toml
store_root       = "~/.config/fumi"        # actions/ と scripts/ を置くルート
default_timeout_ms = 30000                  # scripts/run のデフォルトタイムアウト
```

- Extension からこの設定を読み書きする API は**提供しない**。ユーザーが直接編集する。
- **機微情報(トークン・API キー等)は config.toml に書かない**方針。必要な External Script は環境変数や macOS Keychain 等から取得する(README で明記)。

#### 5.2.8. スクリプト変更の反映

**手動のみ**。ストア編集後、Popup「再取得」で `actions/list` を再実行して `chrome.userScripts` を更新する。watcher / ポーリングは採用しない(短命 Host では watcher を持てない)。

#### 5.2.9. ロギング

- **Host 側はログを残さない**。stdout / stderr は Extension に返すのみでディスクには書き出さない。必要ならスクリプト側で `$TMPDIR` 等に書く。
- Host 自身の致命的エラー(manifest parse 失敗等)のみ stderr に吐き、Chrome のネイティブメッセージング標準挙動で拾う。

---

## 6. データフロー

### 6.1. 起動時 / アクション登録

1. Chrome 起動 / Extension 有効化。
2. Service Worker が `sendNativeMessage` で `actions/list` を送信(Host が spawn → 応答 → exit)。
3. Host が `actions/` を列挙・parse し、フロントマター + 本体コードを返す。
4. Extension が各アクションの match / exclude に従って `chrome.userScripts.register` で登録(world は USER_SCRIPT 固定)。
5. 対象 URL を開くと User Script が実行され、`fumi.contextMenus.create` 等が Service Worker に届き `chrome.contextMenus` に反映される(dispatch 先は §5.1.4)。

### 6.2. アクション実行 / External Script 呼び出し

1. ユーザーがトリガーを発火(ショートカット / コンテキストメニュー / ページ内ボタン / URL マッチ自動)。
2. アクションのハンドラが呼ばれ、ページから値を取得。
3. アクションが `fumi.run("summarize.sh", { text })` を呼ぶ。
4. User Script → Service Worker → Host へ `scripts/run` 送信(Host spawn)。
5. Host が §5.2.5 の検証を通過させ、External Script を直接 spawn。stdin に JSON を流す。
6. Host が exit を待機して結果を返し、自身も exit。
7. Service Worker 経由で User Script に結果が返り、Promise 解決。
8. アクションが結果を DOM 操作や通知に反映。

### 6.3. スクリプト編集

Extension は関与しない。ユーザーが自分のエディタでストアを編集し、Popup「再取得」で §6.1 step 2 以降が再実行される。

---

## 7. メッセージ仕様 (暫定)

本プロトコルは **[JSON-RPC 2.0](https://www.jsonrpc.org/specification) 準拠**。詳細は design/protocol.md。

### 7.1. 共通エンベロープ

```jsonc
// Extension → Host
{ "jsonrpc": "2.0", "id": "uuid", "method": "actions/list" | "scripts/run", "params": { ... } }

// Host → Extension (成功)
{ "jsonrpc": "2.0", "id": "uuid", "result": { ... } }

// Host → Extension (失敗)
{ "jsonrpc": "2.0", "id": "uuid",
  "error": { "code": -33030, "message": "...", "data": { "fumiCode": "EXEC_TIMEOUT", ... } } }
```

- `result` / `error` は相互排他 (JSON-RPC 2.0 §5)。
- `error.code` は数値 (JSON-RPC 標準域 + fumi アプリ定義域 -33000 台)。可読な文字列シンボルは `error.data.fumiCode` に併記し、Extension の UI 分岐はこちらを見る。
- Host → Extension の push 通知は**採用しない**(短命 Host のため)。常に一問一答。
- Notification (id 省略) / batch (配列) は本プロトコルでは非対応。

### 7.2. メソッド

| method | params | result |
|---|---|---|
| `actions/list` | — | `{ actions: Action[] }`(各要素は `{ id, path, matches, excludes, code }`) |
| `scripts/run` | `scriptPath`, `payload`, `timeoutMs?`, `context?` | `{ exitCode, stdout, stderr, durationMs }` |

**実装しないメソッド**(セキュリティ境界として明示): `listScripts`, `readScript`, `writeScript`, `deleteScript`, `renameScript`, `getConfig`, `setConfig`、および任意ファイル I/O。

---

## 8. 非機能要件 **【要確認 CC】**

以下は実装後の計測値で確定:

- レイテンシ: ユーザー発火 → External Script 起動までの目標時間。
- ペイロードサイズ上限: Chrome 側の 1MB 制約を前提。超過時の扱い(エラー / 分割)。
- `actions/list` 返却サイズが 1MB を超える場合の扱い。
- 同時実行数の上限。
- Host 接続失敗時・ストア不在時の UX(Popup での告知)。

---

## 9. セキュリティ要件

### 9.1. 許可モデル: 「ストア内限定 + 書き込み API 非存在」

- External Script のパスは **`scripts/` 配下に限定**。
- Extension から Host にスクリプトを書き込む / 列挙する / 読み取る API は**存在しない**。
- スクリプトの追加・変更はユーザーが自分のエディタでストアを直接編集する方式のみ。

この設計により、以下の攻撃経路が構造的に封じられる:

| 攻撃経路 | 状態 |
|---|---|
| 任意パス実行(LOLBAS 含む) | ✗ 封鎖(ストア外は realpath 検証で拒否) |
| `writeScript` + `scripts/run` 連鎖で悪性ファイルを落として実行 | ✗ 封鎖(`writeScript` が存在しない) |
| シンボリックリンクでストア外を指す | ✗ 封鎖(lstat + realpath 検証) |
| シェルインジェクション via argv | ✗ 封鎖(shell を介さず spawn、payload は stdin 専用) |
| 他拡張からの乗っ取り呼び出し | ✗ 封鎖(`allowed_origins` で Extension ID 固定) |

### 9.2. Host 実装の強制事項 (必須)

- `realpath(scriptPath)` が `scripts/` 配下にあることを検証、外れれば拒否。
- `lstat` で通常ファイル(regular file)かつシンボリックリンクでないことを確認。
- シェル(`sh -c`)を介さず **直接 spawn**(`execve` 系)。
- ペイロードは **stdin 経由のみ**。argv に任意文字列を詰めない。
- Host マニフェストの `allowed_origins` を本 Extension ID に固定。
- ストアディレクトリはユーザー所有のみ読み書き可(`chmod 700` 相当)をセットアップ時に設定 **【要確認 DD】**。

### 9.3. 残存リスク

以下は構造的に防げないが、すべて**ユーザー自身が書いたスクリプトの範囲に閉じ込められ**、「見知らぬ任意コード実行」には至らない。最悪でも「ユーザー自身のツールの誤爆」。

1. **ユーザー自身のスクリプトが unsafe**(例: `run.sh` 内で `sh -c "$(cat)"` 相当)。対策: ドキュメントで注意喚起、ユーザー責任。
2. **payload でユーザーのスクリプトを誤動作させる**(例: stdin 値をファイル名として扱うスクリプトが意図しないファイルを破壊)。対策: スクリプト作者が自前でバリデーション。
3. **動的な `fumi.run(dynamicName, ...)` 呼び出し**: ページ由来データで呼び出し先を決めると confused deputy が残るが、誘導先はユーザー自身のスクリプトに限定され被害は自己完結。対策: ドキュメントで注意喚起、許容リスク。
4. **Extension 自体の compromise**: 乗っ取られても `actions/list` / `scripts/run` しか使えず、攻撃者は既存のユーザー自身のスクリプトしか叩けない。対策: サプライチェーン管理 **【要確認 EE】**。
5. **ローカル攻撃者による直接書き込み**: 既に RCE 済みであり本システムの責務外。

### 9.4. 機微情報

- ペイロードに機微情報が含まれる可能性があるため、ログ書き出しポリシーは §5.2.9 参照。
- Host 設定ファイルにトークン等を保存する場合の扱い **【要確認 FF】**。

---

## 10. 制約・前提

- Chrome(Chromium 系)のみを対象。Edge / Brave 等の対応は暫定的になし **【要確認 GG】**。
- Manifest V3 前提。
- `chrome.userScripts` 利用のため「開発者モード」有効化が必要(README で告知)。
- macOS のみサポート。
- Native Messaging Host のインストールが必要(§5.2.1)。

---

## 11. 開発者体験 (想定フロー)

### 11.1. 配布物

**A. Chrome Extension**
- **Chrome Web Store 経由**(本命): 公開後の Extension ID は固定。
- **unpacked ロード**(開発時): repo を clone した開発者が `chrome://extensions` から読み込む。

**B. Host / CLI**: Homebrew tap(例: `tkuramot/fumi`)で `brew install fumi`:

| 成果物 | パス例 | 役割 |
|---|---|---|
| `fumi` | `/opt/homebrew/bin/fumi` | ユーザー向け管理 CLI。セットアップ・状態確認・ストア操作 |
| `fumi-host` | `/opt/homebrew/bin/fumi-host` | Native Messaging Host。Chrome からのみ spawn される |
| Extension ソース | `$(brew --prefix fumi)/share/fumi/chrome-extension/` | 開発時 unpacked ロード用 |

### 11.2. Extension ID の扱い

Web Store / unpacked 双方に対応するため、`fumi setup` が `allowed_origins` に **両方の Extension ID** を書き込む:

- **Web Store 公開時の ID**: Web Store 側で確定する固定値。`fumi` バイナリに同梱(コンパイル時定数)。
- **unpacked 時の ID**: manifest に `key` フィールドを埋め込むことで決定的に導出される固定値。同梱。

```jsonc
// manifest.json (Native Messaging Host)
{
  "allowed_origins": [
    "chrome-extension://<web-store-id>/",
    "chrome-extension://<unpacked-pinned-id>/"
  ]
}
```

### 11.3. 署名・Notarize

- Host バイナリは **未署名で可**(brew 経由なら quarantine bit なし)。polish するなら後から Developer ID + Notarize を追加。
- Chrome Extension の公開には Web Store のデベロッパー登録($5 登録料)が必要 **【要確認 A】**。

### 11.4. 初回セットアップ手順

**Web Store 経由(一般ユーザー)**

```bash
brew tap tkuramot/fumi
brew install fumi
fumi setup          # manifest 配置、ストア初期化、サンプル配置
```

Chrome で Web Store から Extension をインストール後、`fumi doctor` で確認。

**unpacked 経由(開発者)**

上記 `brew install` / `fumi setup` の後、`chrome://extensions` でデベロッパーモードを有効化 →「パッケージ化されていない拡張機能を読み込む」で `$(brew --prefix fumi)/share/fumi/extension` を指定 → `fumi doctor` で Extension ID を確認。

### 11.5. 日常のワークフロー

1. `~/.config/fumi/actions/*.js` を編集してアクションを書く。
2. `~/.config/fumi/scripts/` に External Script を置く。
3. Popup「再取得」で反映し、対象ページで動作確認。必要なら `fumi doctor` で調査。

### 11.6. `fumi` CLI のサブコマンド

コマンド体系は `fumi <resource> <verb>` を基本とする(`setup` 等のトップレベル動詞は例外)。

| コマンド | 役割 |
|---|---|
| `fumi setup [--browser chrome]` | manifest を所定パス(chrome: `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/`)に配置、ストアを `chmod 700` で作成、サンプル配置(既存は skip) |
| `fumi uninstall [--browser chrome]` | 指定ブラウザの manifest 削除(ストアは残す)。`--all-browsers` で全一括 |
| `fumi doctor [--browser chrome]` | manifest 存在確認、`allowed_origins` と実 Extension ID の一致、ストア権限(0700)、`fumi-host` パス整合性 |
| `fumi actions list` | `actions/` 配下の一覧 |
| `fumi scripts list` | `scripts/` 配下の一覧 |
| `fumi scripts run <name> [--payload '<json>']` | ブラウザを通さず CLI から External Script を叩くデバッグ用。`scripts/run` と同等の検証を通す |

- 初期版は `--browser chrome` のみ実装。Edge / Brave / Arc 等は manifest 配置先の分岐を追加するだけで拡張できる設計。
- `status` サブコマンドは作らない(Popup と重複)。
- `fumi-host` は stdin/stdout プロトコル実装のみで、ユーザー向けサブコマンドは持たない。

---

## 12. 成果物 / リポジトリ構成

**モノレポ**(本 repo 内にすべて)。想定レイアウト:

```
fumi/
├── chrome-extension/    <- Chrome Extension (Web Store 公開対象)
├── host/         <- Native Messaging Host
├── cli/          <- fumi コマンド
├── samples/      <- サンプル (actions/, scripts/)
├── docs/         <- 仕様書・設計ドキュメント
└── README.md
```

- 言語 / ビルドツールは設計フェーズで決定。
- `host` と `cli` は別バイナリだが共通コード(ストアパス解決・検証ロジック等)を共有する想定。
- Homebrew tap は別 repo (`homebrew-fumi`) で管理。

---

## 13. 要確認事項 (保留中)

| ID | 論点 | 決定タイミング |
|---|---|---|
| EE | Extension のサプライチェーン管理(Web Store 公開に伴う鍵管理・リリースプロセス) | Web Store 公開が近づいた時点 |
| CC | 非機能指標の具体値(レイテンシ目標等) | 実装後の計測値で確定 |

本文中の他の **【要確認】** マーカー(A, J, DD, FF, GG 等)は決定タイミング未定。該当箇所のコンテキストで個別判断する。
