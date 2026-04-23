# CLI (`fumi`) 設計 (Go)

対象: spec §11.4 / §11.5 / §11.6。

`fumi` バイナリは **ユーザー向けの管理 CLI**。`fumi-host` と Go module / `internal/` パッケージを共有する。

## 1. パッケージ構成

```
cmd/fumi/                         # package main — fumi バイナリのすべて
├── main.go                       # urfave/cli App の組み立て (§2)
├── setup.go                      # fumi setup        (*cli.Command を return)
├── uninstall.go                  # fumi uninstall
├── doctor.go                     # fumi doctor
├── actions.go                    # fumi actions ...  (Subcommands を束ねる)
├── scripts.go                    # fumi scripts ...  (list / run)
├── manifest.go                   # Native Messaging manifest の組み立て・配置先解決
└── constants.go                  # ldflags で注入される定数 (§3)

internal/                         # host と共有するコードのみ
└── (store, config, runner, protocol は host.md §1)
```

- `cmd/fumi/` 配下はすべて `package main`。コマンド実装とユーザー向け表示ロジックを同居させる。ドメインロジックは `internal/` の共有パッケージに置く。
- **manifest 関連** (`fumi setup` / `fumi uninstall` / `fumi doctor` で使用) は host 本体が関与しないため、共有 `internal/` ではなく `cmd/fumi/manifest.go` に置く。将来 host 側でも manifest を扱う必要が出たら `internal/manifest/` に切り出す。
- Command factory は `package main` 内の小文字スタート関数 (`setupCmd()` 等)。外部からの import も external test package もこの project では不要なため、`package main` 同居で十分。

## 2. コマンドディスパッチ (`urfave/cli/v2`)

`urfave/cli/v2` の `App.Commands` + 入れ子の `Subcommands` で `fumi <resource> <verb>` 体系を表現する。flag パースとヘルプ生成はライブラリに任せる。

```go
// cmd/fumi/main.go
package main

import (
    "context"
    "os"

    "github.com/urfave/cli/v2"
)

func main() {
    app := &cli.App{
        Name:  "fumi",
        Usage: "ブラウザ × ホストマシン連携ユーティリティ",
        Commands: []*cli.Command{
            setupCmd(),
            uninstallCmd(),
            doctorCmd(),
            actionsCmd(),    // fumi actions list
            scriptsCmd(),    // fumi scripts list | run
        },
    }
    if err := app.RunContext(context.Background(), os.Args); err != nil {
        os.Exit(1)
    }
}
```

各コマンドファイルは `*cli.Command` を返す factory 関数として実装する (同 package なので小文字スタート):

```go
// cmd/fumi/scripts.go
package main

func scriptsCmd() *cli.Command {
    return &cli.Command{
        Name:  "scripts",
        Usage: "External Script の一覧・実行",
        Subcommands: []*cli.Command{
            {
                Name:   "list",
                Usage:  "scripts/ 配下を列挙",
                Action: runScriptsList,
            },
            {
                Name:      "run",
                Usage:     "CLI から External Script を叩く (デバッグ用)",
                ArgsUsage: "<script-relative-path>",
                Flags: []cli.Flag{
                    &cli.StringFlag{Name: "payload", Usage: "stdin に流す JSON"},
                    &cli.IntFlag{Name: "timeout", Value: 30000, Usage: "ms"},
                    &cli.BoolFlag{Name: "json", Usage: "結果を JSON で出力"},
                    &cli.BoolFlag{Name: "propagate-exit", Usage: "script の exitCode を CLI の終了コードに反映"},
                },
                Action: runScriptsRun,
            },
        },
    }
}
```

- **カスタム exit code**: `urfave/cli/v2` は `Action` が `error` を返すと stderr に表示して exit 1 する。§5 の終了コード体系に合わせるため、`cli.Exit(msg, code)` (= `cli.ExitCoder` 実装) を return することで任意の exit code を返す。
- **ヘルプの日本語化**: `App.Usage` / `Command.Usage` を日本語で記述。`cli/v2` はヘルプ骨格 (`USAGE:` / `OPTIONS:` などの見出し) が英語固定だが、本体のテキストは自由に書ける。必要ならカスタム `HelpPrinter` を差し込む (MVP では不要)。
- **テスタビリティ**: `App.Writer` / `App.ErrWriter` を `*bytes.Buffer` に差し替えれば stdout/stderr を捕捉してテストできる ([§7](#7-テスト戦略))。

## 3. コンパイル時定数

spec §11.2 より、`allowed_origins` に入れる 2 つの Extension ID は `fumi` バイナリに埋め込む:

```go
// cmd/fumi/constants.go
package main

// リリースビルド時に -ldflags で上書きされる。開発中はダミー値。
var (
    webStoreExtensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    unpackedExtensionID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    hostBinaryPath      = "/opt/homebrew/bin/fumi-host"
)
```

ビルド時 (ldflags は `package main` の変数を `main.<name>` で指定する):
```bash
go build -ldflags "
  -X main.webStoreExtensionID=$WS_ID
  -X main.unpackedExtensionID=$UP_ID
  -X main.hostBinaryPath=$BIN_PATH
" ./cmd/fumi
```

Homebrew formula 側で `brew --prefix` から `hostBinaryPath` を注入する。

## 4. サブコマンド詳細

### 4.1. `fumi setup [--browser chrome]`

**動作**:

1. ストア初期化:
   - `~/.config/fumi/` を `0o700` で作成 (既存なら chmod のみ)。
   - `actions/`, `scripts/` サブディレクトリを `0o700` で作成。
   - `config.toml` が不在なら雛形をコピー。
   - サンプル (`samples/`) を `actions/` と `scripts/` に配置 (既存ファイルは skip、`--force` フラグで上書き可)。
2. Native Messaging manifest 配置:
   - Chrome: `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.tkuramot.fumi.json`
   - 内容:
     ```json
     {
       "name": "com.tkuramot.fumi",
       "description": "fumi native messaging host",
       "path": "/opt/homebrew/bin/fumi-host",
       "type": "stdio",
       "allowed_origins": [
         "chrome-extension://<web-store-id>/",
         "chrome-extension://<unpacked-pinned-id>/"
       ]
     }
     ```
3. 最後に成功メッセージ + 次の手順 (Chrome で拡張機能をロード) を表示。

**フラグ**:
- `--browser chrome` (default): 現状 Chrome のみ。Brave / Edge / Arc は追加時に分岐を追加。
- `--force`: 既存の manifest / サンプルを上書き。

**エラーケース**:
- `$HOME` 不明 (通常ありえない) → 1 を返す。
- 配置先ディレクトリが存在しない (Chrome 未インストール等) → 親ディレクトリから作成。
- 既存 manifest の Extension ID が想定と異なる → 既定では `--force` なしで中止 (上書きは明示操作)。

### 4.2. `fumi uninstall [--browser chrome] [--all-browsers]`

- 指定ブラウザの manifest のみ削除。`~/.config/fumi/` (ストア本体) は **削除しない** (ユーザー資産)。
- `--all-browsers`: 対応ブラウザ全部を walk。

### 4.3. `fumi doctor [--browser chrome]`

各チェックを OK/NG/WARN で出力し、最後に合計判定:

| 項目 | OK 条件 |
|---|---|
| manifest 存在 | `com.tkuramot.fumi.json` が配置先に存在 |
| `allowed_origins` 突合 | manifest 内の 2 つの ID が `WebStoreExtensionID` / `UnpackedExtensionID` と一致 |
| `fumi-host` 実体 | manifest 内 `path` が存在し実行可能 |
| Store 権限 | `~/.config/fumi` が 0700、所有者が $UID |
| `actions/` `scripts/` 存在 | 両ディレクトリが存在 |
| `config.toml` parse | 不在 or 空は OK、構文エラーは NG |

```
$ fumi doctor
[OK]   Native Messaging manifest: /Users/.../com.tkuramot.fumi.json
[OK]   allowed_origins matches embedded Extension IDs
[OK]   fumi-host at /opt/homebrew/bin/fumi-host (executable)
[OK]   Store: /Users/you/.config/fumi (mode 0700)
[OK]   actions/ (3 files), scripts/ (2 files)
[OK]   config.toml: valid

All checks passed.
```

### 4.4. `fumi actions list`

`internal/store.LoadAll` をそのまま呼び、id / path / matches を表形式で表示。エラー時は該当ファイルだけ `[ERR]` 表示、他は通常表示 (Host の `getActions` と違い、CLI は部分表示してデバッグを助ける)。

```
$ fumi actions list
ID            PATH              MATCHES
save-note     save-note.js      https://github.com/*
open-editor   open-editor.js    https://*.github.com/pull/*
[ERR] bad.js: duplicate @id "save-note"
```

### 4.5. `fumi scripts list`

`scripts/` を `filepath.WalkDir` で再帰列挙 (サブディレクトリ対応)。実行可否 (`+x`) と種別 (regular/symlink) を表示。**symlink も表示**するが、警告を添える (Host 側で拒否されるため)。

### 4.6. `fumi scripts run <name> [--payload '<json>'] [--timeout 30000]`

- Host を spawn せず、`internal/runner` を **CLI プロセス内で直接呼ぶ**。
- 検証経路は Host と完全に同一 (`store.ResolveScript` → `runner.Run`)。
- stdout / stderr / exit / duration を human-readable に表示。`--json` で JSON 出力。

```
$ fumi scripts run summarize.sh --payload '{"text":"hello"}'
exit: 0 (123ms)
--- stdout ---
summary: hello
--- stderr ---
(empty)
```

**なぜ Host 経由ではなく直接呼ぶか**: `fumi scripts run` は「ブラウザを介さず検証したい」のが目的。Chrome 経由にすると Chrome 起動が必要になり逆に不便。検証ロジックは `internal/` に集約されているので **同じ安全性で再利用できる**。

## 5. 終了コード

| コード | 意味 |
|---|---|
| 0 | 正常 (`scripts run` の場合 external script の exitCode とは無関係に、CLI 自身は 0) |
| 1 | CLI 使用エラー (flag 不正等) |
| 2 | ドメインエラー (manifest 不在, 検証失敗, doctor で NG あり) |
| 3 | Internal (予期せぬバグ) |

`scripts run` 専用オプション `--propagate-exit` を用意し、external script の exitCode をそのまま CLI の exitCode に反映する (shell script への組み込み用)。

## 6. ログ・出力

- 通常出力は stdout、警告・エラーは stderr。
- `--quiet` / `-v` フラグは最初の版では不要。必要になったら追加。
- 色付けは MVP では導入しない (`[OK]` / `[NG]` などプレーンテキスト)。将来必要になったら raw ANSI シーケンス (`\x1b[32m` 等) を数行で自前実装する。

## 7. テスト戦略

- `cmd/fumi/*` の各 factory が返す `*cli.Command` を、テストでは専用の小さな `cli.App` に包んで `app.Run([]string{"fumi", "scripts", "list"})` のように呼ぶ。テストファイルは `cmd/fumi/*_test.go` (`package main`) に置く。`App.Writer` / `App.ErrWriter` を `*bytes.Buffer` に差し替えて stdout/stderr を検査。
- `fumi setup` / `fumi uninstall` は tempdir をストアルート / manifest 配置先に `--store-root` / `--manifest-dir` フラグで差し替えられるようにしておく (テストフック)。
- `fumi scripts run` は内部で `internal/runner` を叩くのでそこの integration test と合流。

## 8. ユーザー向けエラーメッセージ例

| 状況 | メッセージ |
|---|---|
| Chrome 未インストール | `NativeMessagingHosts directory not found. Is Chrome installed? Try: open -a "Google Chrome" once to initialize.` |
| manifest 既存 (別 ID) | `Existing manifest points to a different Extension ID. Re-run with --force to overwrite.` |
| `scripts run` 対象無し | `Script not found: scripts/foo.sh` (フルパスも併記) |
| `scripts run` 非実行 | `Script is not executable. chmod +x scripts/foo.sh` |

エラーは「何が起きたか + 次に何をすればいいか」を必ず併記する。
