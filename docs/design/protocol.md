# プロトコル: Native Messaging メッセージ仕様

spec §7 を実装レベルまで具体化する。Extension (TypeScript) と Host (Go) の両側で同一のスキーマを実装するため、本ドキュメントが **単一の真実の源**。

## 1. ワイヤフォーマット

Chrome Native Messaging 仕様そのまま:

- `fumi-host` の stdin/stdout に対し、**4 バイトのリトルエンディアン長プレフィックス** + **UTF-8 JSON 本文** を連結した形で読み書き。
- 1 メッセージあたり最大 **1 MiB** (Chrome 側の制限。超過すると Chrome は接続を切る)。

### 1.1. 短命接続

`chrome.runtime.sendNativeMessage(host, message, cb)` を使用。
- Chrome が `fumi-host` を spawn → `message` を 1 件 stdin に書く → Host が stdout に 1 件書く → Chrome が受け取って `cb` 呼び出し → Host の stdout close または exit を待たずに Chrome 側の接続は閉じられる。
- Host は **stdout 書き込み後に自発的に exit** する (Go 側で `os.Exit(0)`)。長寿命 (`connectNative`) は採用しない。

## 2. エンベロープ

### 2.1. リクエスト (Extension → Host)

```jsonc
{
  "id": "uuid-v4",
  "op": "getActions" | "runScript",
  "params": { ... }        // op に応じたパラメータ
}
```

- `id`: Extension 側で UUIDv4 を発番。Host はそのまま echo する (現状は短命 1 往復なので主にログ突き合わせ用)。
- `op`: 2 種類のみ (spec §5.2.3)。他の op を受け取った場合は `UNKNOWN_OP` で応答。
- `params`: op ごとに異なる。後述。

### 2.2. レスポンス (Host → Extension)

成功:
```jsonc
{ "id": "uuid-v4", "ok": true,  "result": { ... } }
```

失敗:
```jsonc
{ "id": "uuid-v4", "ok": false, "error": { "code": "...", "message": "human readable" } }
```

- `ok` は必ず boolean。`result` / `error` のどちらか一方だけ present。
- `error.message` はデバッグ用であり、ユーザー向けメッセージは Extension 側で `code` を見て差し替える。

## 3. エラーコード一覧

| code | 発生元 | 意味 |
|---|---|---|
| `UNKNOWN_OP` | Host | 未知の `op` |
| `INVALID_PARAMS` | Host | `params` のバリデーション失敗 (型違い・必須欠落) |
| `STORE_NOT_FOUND` | Host | ストアルートが存在しない |
| `CONFIG_INVALID` | Host | `config.toml` のパースエラー |
| `ACTIONS_TOO_LARGE` | Host | `getActions` レスポンスが 1 MiB 超過 (spec §5.2.4) |
| `FRONTMATTER_INVALID` | Host | 個別アクションの frontmatter parse 失敗 |
| `INVALID_SCRIPT_PATH` | Host | 絶対パス / `..` / `scripts/` 外を指している |
| `SCRIPT_NOT_FOUND` | Host | `realpath` 後に該当ファイルなし |
| `SCRIPT_NOT_REGULAR_FILE` | Host | シンボリックリンクまたは通常ファイル以外 |
| `SCRIPT_NOT_EXECUTABLE` | Host | `x` bit 未設定 (POSIX) |
| `TIMEOUT` | Host | 実行時間が `timeoutMs` 超過、SIGKILL された |
| `OUTPUT_TOO_LARGE` | Host | stdout または stderr が 1 MiB 超 (spec §5.2.5) |
| `SPAWN_FAILED` | Host | `execve` 失敗 (例: `#!` インタプリタ不在) |
| `INTERNAL` | Host | 予期せぬエラー (バグ) |
| `HOST_UNREACHABLE` | Extension | `chrome.runtime.lastError` が立った (manifest 未配置など) |

## 4. オペレーション詳細

### 4.1. `getActions`

**Request**:
```jsonc
{ "id": "...", "op": "getActions", "params": {} }
```

**Response (success)**:
```jsonc
{
  "id": "...",
  "ok": true,
  "result": {
    "actions": [
      {
        "id": "save-note",                  // @id または filename から導出
        "path": "save-note.js",             // actions/ からの相対
        "matches": ["https://github.com/*"],
        "excludes": ["https://github.com/settings/*"],
        "code": "// ユーザーJSの本体 (frontmatterは除去済み or 保持) "
      }
    ]
  }
}
```

- `code` は frontmatter コメントブロックごと含めた **ファイル全文** (Extension 側で困らない方針)。`chrome.userScripts.register` の `js[].code` にそのまま渡す。
- `matches` / `excludes` は Host 側で正規化済み (ワイルドカード置換・小文字化等はしない、原文そのまま配列化のみ)。
- `@id` 重複は Host が検出して `FRONTMATTER_INVALID` で失敗させる (部分成功させない)。

### 4.2. `runScript`

**Request**:
```jsonc
{
  "id": "...",
  "op": "runScript",
  "params": {
    "scriptPath": "tools/open-in-editor.py",    // scripts/ 相対、拡張子込み
    "payload": { "url": "https://..." },         // 任意 JSON (null OK)
    "timeoutMs": 30000,                          // 省略時は config の default
    "context": {                                 // 任意、Host → External Script の env として展開
      "actionId": "save-note",
      "tabUrl": "https://github.com/a/b"
    }
  }
}
```

- `payload` は Host → External Script の **stdin** に `JSON.stringify` して流す。stdin は書き込み後 close。
- `context` は `FUMI_ACTION_ID` / `FUMI_TAB_URL` として env 追加 (spec §5.2.5)。

**Response (success)**:
```jsonc
{
  "id": "...",
  "ok": true,
  "result": {
    "exitCode": 0,
    "stdout": "...",
    "stderr": "...",
    "durationMs": 123
  }
}
```

- `stdout` / `stderr` は UTF-8 文字列。バイナリ出力は想定外 (ユーザーが base64 して返す運用)。

**Response (failure)**: §3 のいずれかの `code`。

## 5. TypeScript / Go の型対応

### 5.1. TypeScript (`extension/src/shared/protocol.ts`)

```ts
export type Request =
  | { id: string; op: "getActions"; params: Record<string, never> }
  | {
      id: string;
      op: "runScript";
      params: {
        scriptPath: string;
        payload: unknown;
        timeoutMs?: number;
        context?: { actionId?: string; tabUrl?: string };
      };
    };

export type Action = {
  id: string;
  path: string;
  matches: string[];
  excludes: string[];
  code: string;
};

export type RunResult = {
  exitCode: number;
  stdout: string;
  stderr: string;
  durationMs: number;
};

export type Response<R> =
  | { id: string; ok: true; result: R }
  | { id: string; ok: false; error: { code: ErrorCode; message: string } };

export type ErrorCode =
  | "UNKNOWN_OP"
  | "INVALID_PARAMS"
  /* ... §3 の全コード ... */
  | "HOST_UNREACHABLE";
```

### 5.2. Go (`internal/protocol/types.go`)

```go
type Request struct {
    ID     string          `json:"id"`
    Op     string          `json:"op"`
    Params json.RawMessage `json:"params"`
}

type Response struct {
    ID     string      `json:"id"`
    OK     bool        `json:"ok"`
    Result interface{} `json:"result,omitempty"`
    Error  *ErrorBody  `json:"error,omitempty"`
}

type ErrorBody struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

type Action struct {
    ID       string   `json:"id"`
    Path     string   `json:"path"`
    Matches  []string `json:"matches"`
    Excludes []string `json:"excludes"`
    Code     string   `json:"code"`
}

type RunResult struct {
    ExitCode   int    `json:"exitCode"`
    Stdout     string `json:"stdout"`
    Stderr     string `json:"stderr"`
    DurationMs int64  `json:"durationMs"`
}
```

`Params` は `json.RawMessage` として受け、op ごとに専用構造体に再 Unmarshal する (op が不明な段階で先に型を決めずに済む)。

## 6. バージョニング方針

- 現時点では **バージョンフィールドなし**。破壊的変更は `op` 名の別名化 (`getActions2` 等) で対応する。
- Host と Extension は brew + Web Store で独立配布されるため、起動時の互換性チェックは `fumi doctor` に委ねる (manifest の存在確認のみで、バージョン突合はしない)。
