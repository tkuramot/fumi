# プロトコル: Native Messaging メッセージ仕様

spec §7 を実装レベルまで具体化する。Extension (TypeScript) と Host (Go) の両側で同一のスキーマを実装するため、本ドキュメントが **単一の真実の源**。

本プロトコルは **[JSON-RPC 2.0](https://www.jsonrpc.org/specification) 準拠**とする。ワイヤフォーマットのみ Chrome Native Messaging の制約 (4B LE 長プレフィックス) に従う。既存の JSON-RPC 2.0 ライブラリ (TS: `json-rpc-2.0`, Go: `github.com/sourcegraph/jsonrpc2` 等) をそのまま利用できる。

## 1. ワイヤフォーマット

Chrome Native Messaging 仕様そのまま:

- `fumi-host` の stdin/stdout に対し、**4 バイトのリトルエンディアン長プレフィックス** + **UTF-8 JSON 本文** を連結した形で読み書き。
- 1 メッセージあたり最大 **1 MiB** (Chrome 側の制限。超過すると Chrome は接続を切る)。

### 1.1. 短命接続

`chrome.runtime.sendNativeMessage(host, message, cb)` を使用。

- Chrome が `fumi-host` を spawn → `message` を 1 件 stdin に書く → Host が stdout に 1 件書く → Chrome が受け取って `cb` 呼び出し → Host の stdout close または exit を待たずに Chrome 側の接続は閉じられる。
- Host は **stdout 書き込み後に自発的に exit** する (Go 側で `os.Exit(0)`)。長寿命 (`connectNative`) は採用しない。

### 1.2. 使わない JSON-RPC 2.0 機能

短命一問一答に特化するため、以下は**本プロトコルでは非対応**:

- **Notification** (id 省略リクエスト): Host が受け取った場合は無視し、即 exit。応答は返さない (仕様通り)。
- **Batch** (配列リクエスト): Host が受け取った場合は `-32600 Invalid Request` を 1 件返して exit。
- **Host → Extension の push**: 採用しない (短命プロセスのため)。

## 2. エンベロープ

### 2.1. リクエスト (Extension → Host)

```jsonc
{
  "jsonrpc": "2.0",
  "id": "uuid-v4",
  "method": "actions/list" | "scripts/run",
  "params": { ... }        // method に応じたパラメータ。不要なら省略可
}
```

- `jsonrpc`: 固定文字列 `"2.0"`。欠落または不一致は `-32600 Invalid Request`。
- `id`: Extension 側で UUIDv4 を発番 (string)。Host はそのまま echo。JSON-RPC 2.0 は number / string / null を許容するが、本プロトコルでは string を推奨 (ログ突き合わせ用途)。
- `method`: §4 の 2 種類のみ。未知の値は `-32601 Method not found`。命名規則は §2.3。
- `params`: method ごとに異なる (§4)。パラメータを取らない method では **省略可** (JSON-RPC 2.0 §4.2)。

### 2.2. レスポンス (Host → Extension)

成功:
```jsonc
{ "jsonrpc": "2.0", "id": "uuid-v4", "result": { ... } }
```

失敗:
```jsonc
{
  "jsonrpc": "2.0",
  "id": "uuid-v4",
  "error": {
    "code": -33030,
    "message": "human readable",
    "data": { "fumiCode": "EXEC_TIMEOUT", ... }
  }
}
```

不変条件 (JSON-RPC 2.0 §5):

- 1 レスポンスにつき `result` と `error` は**相互排他**。両方 present / 両方欠落は不正。
- リクエストの `id` が識別不能なエラー (例: Parse error) の場合のみ `id: null`。それ以外はリクエストの `id` を必ず echo。
- `error.message` はデバッグ用 (英語推奨、1 行)。ユーザー向け文言は Extension 側で `error.data.fumiCode` を見て差し替える。
- `error.data` は **必ず object** とし、機械可読な追加情報を入れる。少なくとも `fumiCode` (string) を含める。

### 2.3. 命名規則

**method**: `<namespace>/<verb>` 形式。

- `<namespace>`: **複数形小文字** (`actions`, `scripts`)。リソースのコレクションを指す。
- `<verb>`: **小文字**。[Google AIP-131〜136](https://google.aip.dev/131) の標準動詞を優先する。
  - `list` — コレクションを複数返す (クエリ条件なしの列挙)
  - `get` — 単一リソースを ID で取得
  - `create` / `update` / `delete` — 標準 CRUD
  - カスタム動作は意味を反映した動詞 (`run`, `validate`, `cancel` 等)
- セパレータは slash `/` (LSP / MCP 踏襲)。dot `.` は使わない。
- JSON-RPC 2.0 §4.3 により `rpc.` プレフィックスは予約。本プロトコルでは使用禁止。

**フィールド名**: JSON オブジェクトのキーは **lowerCamelCase** (`scriptPath`, `exitCode`, `durationMs`, `timeoutMs`, `fumiCode`)。Chrome API / JSON-RPC コミュニティ慣例と揃える。スネークケース (`script_path`) / kebab-case は使わない。

**エラーコードシンボル (`fumiCode`)**: **SCREAMING_SNAKE_CASE** + カテゴリプレフィックス (`STORE_NOT_FOUND` 等、§3)。

## 3. エラーコード

JSON-RPC 2.0 は `code` を **number** で規定する。本プロトコルは以下の方針:

- **標準域** (-32768 〜 -32000, JSON-RPC 予約) はそのまま活用。
- **アプリ定義域**は -33000 台を fumi 用に確保。
- **可読性のため、全アプリ定義エラーは `error.data.fumiCode` に文字列シンボルを併記**する。Extension 側の UI 分岐はこの文字列を見る。数値 code は transport 層の分類用。

### 3.1. 標準域 (JSON-RPC 2.0 既定)

| code | 意味 | fumiCode |
|---|---|---|
| -32700 | Parse error (JSON として不正) | `PROTO_PARSE_ERROR` |
| -32600 | Invalid Request (エンベロープ不正 / `jsonrpc` 欠落 / batch 非対応等) | `PROTO_INVALID_REQUEST` |
| -32601 | Method not found | `PROTO_METHOD_NOT_FOUND` |
| -32602 | Invalid params (型違い・必須欠落) | `PROTO_INVALID_PARAMS` |
| -32603 | Internal error (Host のバグ) | `INTERNAL` |

### 3.2. アプリ定義域 (-33000 台)

命名は `<カテゴリ>_<詳細>` で階層化。Extension はプレフィックスで UI 分岐できる (STORE_/SCRIPT_ はユーザー操作で直せる、EXEC_ は実行環境由来、PROTO_/INTERNAL は実装バグ)。

| code | fumiCode | 意味 | `data` 追加フィールド |
|---|---|---|---|
| -33001 | `STORE_NOT_FOUND` | ストアルートが存在しない | `path` |
| -33002 | `STORE_CONFIG_INVALID` | `config.toml` のパースエラー | `path`, `reason` |
| -33010 | `STORE_ACTIONS_TOO_LARGE` | `actions/list` レスポンスが 1 MiB 超過 (spec §5.2.4) | `sizeBytes`, `limitBytes` |
| -33011 | `STORE_FRONTMATTER_INVALID` | 個別アクションの frontmatter parse 失敗、または `@id` 重複 | `path`, `reason` |
| -33020 | `SCRIPT_INVALID_PATH` | 絶対パス / `..` / `scripts/` 外を指している | `scriptPath` |
| -33021 | `SCRIPT_NOT_FOUND` | `realpath` 後に該当ファイルなし | `scriptPath`, `resolved` |
| -33022 | `SCRIPT_NOT_REGULAR_FILE` | シンボリックリンクまたは通常ファイル以外 | `scriptPath`, `resolved` |
| -33023 | `SCRIPT_NOT_EXECUTABLE` | `x` bit 未設定 (POSIX) | `scriptPath`, `resolved` |
| -33030 | `EXEC_TIMEOUT` | 実行時間が `timeoutMs` 超過、SIGKILL された | `timeoutMs`, `durationMs` |
| -33031 | `EXEC_OUTPUT_TOO_LARGE` | stdout または stderr が上限超 (§4.2 参照) | `stream` ("stdout"/"stderr"), `sizeBytes`, `limitBytes` |
| -33032 | `EXEC_SPAWN_FAILED` | `execve` 失敗 (例: `#!` インタプリタ不在) | `scriptPath`, `errno` |

### 3.3. Extension 側でのみ発生するコード

これらは**プロトコル上は流れない**。Extension 内部で `chrome.runtime.lastError` 等を分類するためのローカル enum として定義する (Host 実装者は参照不要)。

| fumiCode | 意味 |
|---|---|
| `HOST_UNREACHABLE` | `chrome.runtime.lastError` が立った (manifest 未配置・Host 未インストール等) |

### 3.4. 「Host 呼び出しの成否」と「スクリプト実行の成否」の区別

`scripts/run` では以下を明確に切り分ける:

- **Host 呼び出しが成立**した (Host が spawn できてレスポンスを返せた) → `result` 側。スクリプトが non-zero exit しても **`result.exitCode` に入る** (`error` にはならない)。
- **Host 呼び出し自体が失敗**した (タイムアウトで kill / 出力サイズ超過 / spawn 失敗 / パス検証失敗) → `error` 側。

OS が SIGKILL を送った場合の扱い:

- Host 側タイムアウト起因の kill → `EXEC_TIMEOUT` (error)
- それ以外の外的要因 (OOM killer 等) による異常終了 → `result.exitCode` に負値 or signal 値として反映 (Host 実装は `exitCode = -signal` で返す)

## 4. メソッド詳細

### 4.1. `actions/list`

ストア内の全アクションを列挙する (AIP-132 List 相当)。単一取得 (`actions/get`) は将来追加予定だが MVP では不要。

**Request**:
```jsonc
{ "jsonrpc": "2.0", "id": "...", "method": "actions/list" }
```

`params` は取らないので省略する。`params: {}` や `params: null` を送ってきた場合は受理する (JSON-RPC 2.0 §4.2 準拠)。

**Response (success)**:
```jsonc
{
  "jsonrpc": "2.0",
  "id": "...",
  "result": {
    "actions": [
      {
        "id": "save-note",                  // @id または filename から導出
        "path": "save-note.js",             // actions/ からの相対
        "matches": ["https://github.com/*"],
        "excludes": ["https://github.com/settings/*"],
        "code": "// ユーザーJSの本体 (frontmatterコメントブロックごと保持)"
      }
    ]
  }
}
```

- `code` は frontmatter コメントブロックごと含めた **ファイル全文**。`chrome.userScripts.register` の `js[].code` にそのまま渡す。
- `matches` / `excludes` は Host が原文そのまま配列化。正規化 (ワイルドカード置換・小文字化等) はしない。

### 4.2. `scripts/run`

External Script を起動する (AIP-136 のカスタム動詞)。

**Request**:
```jsonc
{
  "jsonrpc": "2.0",
  "id": "...",
  "method": "scripts/run",
  "params": {
    "scriptPath": "tools/open-in-editor.py",
    "payload": { "url": "https://..." },
    "timeoutMs": 30000,
    "context": {
      "actionId": "save-note",
      "tabUrl": "https://github.com/a/b"
    }
  }
}
```

- `scriptPath` (string, required): `scripts/` 相対。拡張子込み。
- `payload` (any, required): JSON 値。Host → External Script の **stdin** に `JSON.stringify` して流す。stdin は書き込み後 close。`null` 許容。
- `timeoutMs` (integer, optional): 省略または `null` は Host config の `default_timeout_ms` を使う。`0` は **「タイムアウト無効」**を意味する (spec 合意済みなら)、または使用禁止とする — **本プロトコルでは「無制限」は非対応とし、`0` 以下は `PROTO_INVALID_PARAMS`**。最大値は `int53` (JSON number 安全域) の範囲内。
- `context` (object, optional): 環境変数として External Script に渡される追加情報。`context[k] = v` は **`FUMI_<SCREAMING_SNAKE_CASE(k)>`** として env に展開される (例: `actionId` → `FUMI_ACTION_ID`, `tabUrl` → `FUMI_TAB_URL`)。値は string のみ許容 (string でないフィールドは `PROTO_INVALID_PARAMS`)。`FUMI_` で始まる既存 env を上書きしないよう、Host が衝突キーを拒否。予約済みキー: `STORE` (常に Host が設定する `FUMI_STORE`)。未知キーは受理するが、キー名は `^[a-zA-Z][a-zA-Z0-9_]*$` に一致しないものを拒否 (env 名サニタイズ)。

**Response (success)**:
```jsonc
{
  "jsonrpc": "2.0",
  "id": "...",
  "result": {
    "exitCode": 0,
    "stdout": "...",
    "stderr": "...",
    "durationMs": 123
  }
}
```

- `stdout` / `stderr` は UTF-8 文字列。バイナリは想定外 (base64 して返す運用)。

**サイズ上限**:

レスポンス全体が Native Messaging 1 MiB を超えてはいけないため、stdout + stderr + エンベロープ + JSON エスケープ由来のオーバーヘッドの合計で 1 MiB を切る必要がある。実装上の既定:

- `stdout` ≤ 768 KiB
- `stderr` ≤ 128 KiB

超過時は `EXEC_OUTPUT_TOO_LARGE` (truncate はしない)。大量出力が必要な場合は `$TMPDIR` にファイルで書いてパスだけ返す運用。

**失敗時**: §3 の `error` で返す。

## 5. TypeScript / Go の型対応

### 5.1. TypeScript (`extension/src/shared/protocol.ts`)

```ts
type JsonRpcId = string;  // UUIDv4

export type Request =
  | { jsonrpc: "2.0"; id: JsonRpcId; method: "actions/list"; params?: undefined }
  | {
      jsonrpc: "2.0";
      id: JsonRpcId;
      method: "scripts/run";
      params: {
        scriptPath: string;
        payload: unknown;
        timeoutMs?: number;
        context?: Record<string, string>;   // FUMI_<UPPER_SNAKE(key)> として env 展開
      };
    };

export type Action = {
  id: string;
  path: string;
  matches: string[];
  excludes: string[];
  code: string;
};

export type GetActionsResult = { actions: Action[] };

export type RunScriptResult = {
  exitCode: number;
  stdout: string;
  stderr: string;
  durationMs: number;
};

export type FumiErrorCode =
  // Standard (JSON-RPC 2.0)
  | "PROTO_PARSE_ERROR"
  | "PROTO_INVALID_REQUEST"
  | "PROTO_METHOD_NOT_FOUND"
  | "PROTO_INVALID_PARAMS"
  | "INTERNAL"
  // Store
  | "STORE_NOT_FOUND"
  | "STORE_CONFIG_INVALID"
  | "STORE_ACTIONS_TOO_LARGE"
  | "STORE_FRONTMATTER_INVALID"
  // Script resolution
  | "SCRIPT_INVALID_PATH"
  | "SCRIPT_NOT_FOUND"
  | "SCRIPT_NOT_REGULAR_FILE"
  | "SCRIPT_NOT_EXECUTABLE"
  // Execution
  | "EXEC_TIMEOUT"
  | "EXEC_OUTPUT_TOO_LARGE"
  | "EXEC_SPAWN_FAILED";

export type RpcError = {
  code: number;
  message: string;
  data: { fumiCode: FumiErrorCode } & Record<string, unknown>;
};

export type Response<R> =
  | { jsonrpc: "2.0"; id: JsonRpcId; result: R }
  | { jsonrpc: "2.0"; id: JsonRpcId; error: RpcError };

// Extension 内部のみで使うローカル分類 (ワイヤには流れない)
export type LocalErrorCode = "HOST_UNREACHABLE";
```

### 5.2. Go (`internal/protocol/types.go`)

```go
const JsonRpcVersion = "2.0"

type Request struct {
    JsonRpc string          `json:"jsonrpc"`
    ID      json.RawMessage `json:"id"`      // string/number/null 許容
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
    JsonRpc string          `json:"jsonrpc"`
    ID      json.RawMessage `json:"id"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *RpcError       `json:"error,omitempty"`
}

type RpcError struct {
    Code    int            `json:"code"`
    Message string         `json:"message"`
    Data    map[string]any `json:"data"`   // fumiCode を必ず含む
}

type Action struct {
    ID       string   `json:"id"`
    Path     string   `json:"path"`
    Matches  []string `json:"matches"`
    Excludes []string `json:"excludes"`
    Code     string   `json:"code"`
}

type GetActionsResult struct {
    Actions []Action `json:"actions"`
}

type RunScriptParams struct {
    ScriptPath string            `json:"scriptPath"`
    Payload    json.RawMessage   `json:"payload"`
    TimeoutMs  *int              `json:"timeoutMs,omitempty"`
    Context    map[string]string `json:"context,omitempty"` // FUMI_<UPPER_SNAKE(key)> で env 展開
}

type RunScriptResult struct {
    ExitCode   int    `json:"exitCode"`
    Stdout     string `json:"stdout"`
    Stderr     string `json:"stderr"`
    DurationMs int64  `json:"durationMs"`
}
```

`Params` / `Result` は `json.RawMessage` で受け、method ごとに専用構造体へ再 Unmarshal する。`Response.Result` と `Response.Error` は相互排他 (`omitempty` で片方だけ出力)。

## 6. バージョニング方針

プロトコル全体のバージョンは `jsonrpc: "2.0"` で表現される (エンベロープ層のバージョンはこれで確定)。fumi 固有のメソッド追加・破壊的変更は以下の方針:

- **非破壊な追加** (新 method、既存 method の optional param 追加、`error.data` のフィールド追加): そのまま追加。Host / Extension のどちらが古くても壊れない形で入れる。
- **破壊的変更** (既存 method のシグネチャ変更、エラーコード意味変更): namespace にバージョンを付けて別名化 (`actions/list` → `actions/listV2` もしくは `v2/actions/list`) し、旧名も当面残す。
- **Host と Extension は brew + Web Store で独立配布**されるため、起動時のバージョン突合は行わず、個別の method が `-32601 Method not found` を返したら Extension 側で機能縮退する (例: 新 method がなければ旧 method にフォールバック)。
- `fumi doctor` は manifest 配置確認のみで、バージョン突合はしない。
