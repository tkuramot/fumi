# Native Messaging Host 設計 (Go)

対象: spec §5.2 / §9。

## 1. パッケージ構成

```
host/
└── main.go                           # エントリ (short main, 実装は internal/ に委譲)

internal/
├── protocol/
│   ├── codec.go                      # 4B LE + JSON の read/write
│   └── types.go                      # Request / Response / Action (protocol.md §5.2)
├── config/
│   └── config.go                     # ~/.config/fumi/config.toml ローダ
├── store/
│   ├── paths.go                      # ストアルート解決 (env / config / default)
│   ├── frontmatter.go                # ==Fumi Action== ブロックのパーサ
│   ├── actions.go                    # actions/ 列挙 + parse
│   └── scripts.go                    # scripts/ のパス検証 (realpath + lstat)
├── runner/
│   └── runner.go                     # External Script の spawn / timeout / 出力収集
└── hostapp/
    ├── dispatch.go                   # op → handler ルーティング
    ├── get_actions.go                # getActions ハンドラ
    └── run_script.go                 # runScript ハンドラ
```

- `main.go` は `~20 行` 程度: 標準入出力のロック、`hostapp.Run(os.Stdin, os.Stdout)` 呼び出し、終了コード管理のみ。
- **`internal/hostapp/` は Host 専用ロジック**。CLI からは import しない (CLI のデバッグ実行は `internal/runner` を直接使う)。

## 2. `main.go` の骨子

```go
package main

import (
    "os"

    "github.com/tkuramot/fumi/internal/hostapp"
)

func main() {
    // Native Messaging は短命プロセス。1 リクエスト処理して exit。
    exitCode := hostapp.Run(os.Stdin, os.Stdout, os.Stderr)
    os.Exit(exitCode)
}
```

## 3. `internal/protocol/codec.go`

```go
func ReadMessage(r io.Reader) ([]byte, error) {
    var lenBuf [4]byte
    if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
        return nil, err
    }
    n := binary.LittleEndian.Uint32(lenBuf[:])
    if n > 1024*1024 { // 1 MiB
        return nil, ErrMessageTooLarge
    }
    buf := make([]byte, n)
    if _, err := io.ReadFull(r, buf); err != nil {
        return nil, err
    }
    return buf, nil
}

func WriteMessage(w io.Writer, body []byte) error {
    if len(body) > 1024*1024 {
        return ErrMessageTooLarge
    }
    var lenBuf [4]byte
    binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(body)))
    if _, err := w.Write(lenBuf[:]); err != nil {
        return err
    }
    _, err := w.Write(body)
    return err
}
```

- 1 MiB は Chrome Native Messaging の仕様上限。読み書き両側で enforce。
- `Chrome → Host` 方向で 1 MiB 超のリクエストはそもそも Chrome が投げてこない (送信側で拒否される) が、Host 側でも防御的に検査。

## 4. `internal/config/`

```go
type Config struct {
    StoreRoot        string        `toml:"store_root"`
    DefaultTimeoutMs int           `toml:"default_timeout_ms"`
}

func Load() (*Config, error) {
    path := filepath.Join(userConfigDir(), "fumi", "config.toml")
    // 不在時はデフォルトを返す (エラーにしない)
    // ...
}

func (c *Config) DefaultTimeout() time.Duration {
    if c.DefaultTimeoutMs <= 0 { return 30 * time.Second }
    return time.Duration(c.DefaultTimeoutMs) * time.Millisecond
}
```

- `~/.config/fumi/config.toml` が不在でも動作する (デフォルト値で埋める)。
- TOML ライブラリ: `github.com/BurntSushi/toml` を採用 (依存採用の理由は [README.md の依存ポリシー](./README.md#依存ポリシー) 参照)。stdlib には TOML parser が無く、手書きはコスト過大と判断。

## 5. `internal/store/paths.go`

優先順: **env `FUMI_STORE`** > **`config.toml` の `store_root`** > **`~/.config/fumi`** (spec §5.2.6)。

```go
type Paths struct {
    Root    string   // 解決済み絶対パス
    Actions string   // Root/actions
    Scripts string   // Root/scripts
}

func Resolve(cfg *config.Config) (*Paths, error) {
    root := firstNonEmpty(os.Getenv("FUMI_STORE"), cfg.StoreRoot, defaultRoot())
    root = expandTilde(root)
    abs, err := filepath.Abs(root)
    if err != nil { return nil, err }
    // 存在確認は ops ごとに行う (STORE_NOT_FOUND はレスポンスエラー)
    return &Paths{
        Root:    abs,
        Actions: filepath.Join(abs, "actions"),
        Scripts: filepath.Join(abs, "scripts"),
    }, nil
}
```

## 6. `internal/store/frontmatter.go`

Tampermonkey 風の `// ==Fumi Action==` ブロックを parse。

```go
var (
    startRe = regexp.MustCompile(`^\s*//\s*==Fumi Action==\s*$`)
    endRe   = regexp.MustCompile(`^\s*//\s*==/Fumi Action==\s*$`)
    lineRe  = regexp.MustCompile(`^\s*//\s*@(\w+)\s+(.+?)\s*$`)
)

type Frontmatter struct {
    ID       string
    Matches  []string
    Excludes []string
}

func Parse(src string) (*Frontmatter, error) {
    // 先頭行から ==Fumi Action== を探す (コメント以外が先にあれば NoFrontmatter)
    // 行ごとに key/value を抽出。未知キーは "FRONTMATTER_INVALID"。
    // ...
}
```

- `@id`, `@match`, `@exclude` のみサポート (spec §5.2.4)。未知キー・重複 `@id` は `FRONTMATTER_INVALID`。
- 空の `==Fumi Action==` ブロックも許容 (defaults のみでアクション成立)。
- `@id` 省略時は filename の拡張子除去 + `kebab-case` 正規化。

## 7. `internal/store/actions.go`

```go
func LoadAll(p *Paths) ([]protocol.Action, error) {
    entries, err := os.ReadDir(p.Actions)
    if err != nil { return nil, err }

    seen := map[string]string{} // id -> path (重複検出)
    var actions []protocol.Action
    for _, e := range entries {
        if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") { continue }
        full := filepath.Join(p.Actions, e.Name())
        src, err := os.ReadFile(full)
        if err != nil { return nil, err }
        fm, err := frontmatter.Parse(string(src))
        if err != nil { return nil, fmt.Errorf("%s: %w", e.Name(), err) }
        id := fm.ID
        if id == "" { id = deriveIDFromFilename(e.Name()) }
        if prev, ok := seen[id]; ok {
            return nil, fmt.Errorf("duplicate @id %q in %s and %s", id, prev, e.Name())
        }
        seen[id] = e.Name()
        actions = append(actions, protocol.Action{
            ID: id, Path: e.Name(),
            Matches: fm.Matches, Excludes: fm.Excludes,
            Code: string(src),
        })
    }
    return actions, nil
}
```

- `actions/` 直下の `.js` のみ対象。サブディレクトリは現状走査しない (フラット前提)。
- ファイル走査は**単層のみ**。将来ネスト対応するなら再帰化は容易。

## 8. `internal/store/scripts.go` (セキュリティ要件の心臓部)

spec §5.2.5 / §9.2 を実装する。

```go
type ResolvedScript struct {
    AbsPath string   // realpath 後の絶対パス
    Cwd     string   // スクリプトのディレクトリ
}

func ResolveScript(p *Paths, rel string) (*ResolvedScript, *protocol.ErrorBody) {
    // 1. 絶対パス / .. / 空文字を拒否
    if rel == "" || filepath.IsAbs(rel) {
        return nil, errBody("INVALID_SCRIPT_PATH", "must be relative")
    }
    cleaned := filepath.Clean(rel)
    if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "/../") {
        return nil, errBody("INVALID_SCRIPT_PATH", "parent traversal not allowed")
    }

    candidate := filepath.Join(p.Scripts, cleaned)

    // 2. lstat: シンボリックリンクを拒否
    li, err := os.Lstat(candidate)
    if err != nil {
        if os.IsNotExist(err) { return nil, errBody("SCRIPT_NOT_FOUND", err.Error()) }
        return nil, errBody("INTERNAL", err.Error())
    }
    if li.Mode()&os.ModeSymlink != 0 {
        return nil, errBody("SCRIPT_NOT_REGULAR_FILE", "symlinks are rejected")
    }
    if !li.Mode().IsRegular() {
        return nil, errBody("SCRIPT_NOT_REGULAR_FILE", "not a regular file")
    }

    // 3. realpath: 中間ディレクトリが symlink でストア外を指さないか確認
    resolved, err := filepath.EvalSymlinks(candidate)
    if err != nil { return nil, errBody("INTERNAL", err.Error()) }
    scriptsRoot, err := filepath.EvalSymlinks(p.Scripts)
    if err != nil { return nil, errBody("STORE_NOT_FOUND", err.Error()) }
    if !isWithin(resolved, scriptsRoot) {
        return nil, errBody("INVALID_SCRIPT_PATH", "resolved outside scripts/")
    }

    // 4. 実行権限
    if li.Mode().Perm()&0o111 == 0 {
        return nil, errBody("SCRIPT_NOT_EXECUTABLE", "missing +x")
    }

    return &ResolvedScript{
        AbsPath: resolved,
        Cwd:     filepath.Dir(resolved),
    }, nil
}

func isWithin(child, parent string) bool {
    rel, err := filepath.Rel(parent, child)
    if err != nil { return false }
    return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
```

**検証順序の根拠**:
1. まず文字列として絶対/`..`/空を弾く (安い)。
2. `lstat` で symlink を早期拒否 (`realpath` は中間 symlink を素通りしてしまうので、最終成分が symlink のパターンはここで止める)。
3. `realpath` でストア外に出ないことを確認 (中間ディレクトリが symlink で抜け出すケースを捕捉)。
4. 実行権限チェック。

**意図的に検査しない項目**:
- ファイルサイズ上限: 任意バイナリの実行が前提なので制限しない。
- ファイル所有者チェック: `chmod 700` ポリシーはセットアップ時に設定するが、起動時の再確認は行わない (ユーザーが意図的に緩めた場合は尊重)。

## 9. `internal/runner/` (External Script 起動)

```go
type RunParams struct {
    Script     *store.ResolvedScript
    Payload    json.RawMessage   // stdin にそのまま流す (minify せず raw bytes)
    Timeout    time.Duration
    StoreRoot  string
    ExtraEnv   map[string]string // FUMI_ACTION_ID, FUMI_TAB_URL など
}

type RunOutcome struct {
    ExitCode   int
    Stdout     []byte
    Stderr     []byte
    DurationMs int64
}

func Run(ctx context.Context, p *RunParams) (*RunOutcome, *protocol.ErrorBody) {
    runCtx, cancel := context.WithTimeout(ctx, p.Timeout)
    defer cancel()

    cmd := exec.CommandContext(runCtx, p.Script.AbsPath) // シェル非介在
    cmd.Dir = p.Script.Cwd
    cmd.Env = buildEnv(p.StoreRoot, p.ExtraEnv)          // os.Environ() + 追加

    stdin, err := cmd.StdinPipe()
    if err != nil { return nil, errBody("SPAWN_FAILED", err.Error()) }

    stdout := &cappedBuffer{limit: 1024 * 1024}
    stderr := &cappedBuffer{limit: 1024 * 1024}
    cmd.Stdout = stdout
    cmd.Stderr = stderr

    start := time.Now()
    if err := cmd.Start(); err != nil {
        return nil, errBody("SPAWN_FAILED", err.Error())
    }

    // stdin への書き込みはゴルーチンで (payload が 1 MiB 近くでパイプが詰まる可能性)
    writeErr := make(chan error, 1)
    go func() {
        _, e := stdin.Write(p.Payload)
        stdin.Close()
        writeErr <- e
    }()

    err = cmd.Wait()
    duration := time.Since(start).Milliseconds()

    if stdout.Overflowed() || stderr.Overflowed() {
        return nil, errBody("OUTPUT_TOO_LARGE", "stdout or stderr exceeded 1 MiB")
    }

    if ctx.Err() == context.DeadlineExceeded || runCtx.Err() == context.DeadlineExceeded {
        // CommandContext は既に SIGKILL を送っているが、SIGTERM→SIGKILL の段階化は
        // 手動で cmd.Process.Signal(syscall.SIGTERM) → 数百ms → cmd.Process.Kill() を行う
        return nil, errBody("TIMEOUT", "script timed out")
    }

    exitCode := 0
    if ee, ok := err.(*exec.ExitError); ok {
        exitCode = ee.ExitCode()
    } else if err != nil {
        return nil, errBody("INTERNAL", err.Error())
    }

    _ = <-writeErr   // stdin 書き込みエラーは握り潰し (プロセスが早期 exit したケース)

    return &RunOutcome{
        ExitCode: exitCode,
        Stdout:   stdout.Bytes(),
        Stderr:   stderr.Bytes(),
        DurationMs: duration,
    }, nil
}
```

**段階的 kill の実装**: `exec.CommandContext` は context キャンセル時に即 `Kill()` を呼ぶ挙動なので、spec §5.2.5 の「SIGTERM → 数百 ms → SIGKILL」を満たすには **独自に context を監視**する:

```go
go func() {
    <-runCtx.Done()
    if cmd.Process == nil { return }
    _ = cmd.Process.Signal(syscall.SIGTERM)
    select {
    case <-time.After(500 * time.Millisecond):
        _ = cmd.Process.Kill()
    case <-processExited:
    }
}()
```

`exec.CommandContext` は使わず、自前で `exec.Command(...)` + 上記ゴルーチンで管理する (`ctx.Done()` を自分で受ける) 方がコントロールしやすい。

**`cappedBuffer`**:

```go
type cappedBuffer struct {
    buf   bytes.Buffer
    limit int
    over  bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
    if b.over { return len(p), nil } // 早期 return、後続は drop
    remaining := b.limit - b.buf.Len()
    if len(p) > remaining {
        b.buf.Write(p[:remaining])
        b.over = true
        return len(p), nil
    }
    return b.buf.Write(p)
}

func (b *cappedBuffer) Overflowed() bool { return b.over }
func (b *cappedBuffer) Bytes() []byte    { return b.buf.Bytes() }
```

溢れた時点で `over` を立て、以降の書き込みは drop。最後に `Overflowed()` をチェックして `OUTPUT_TOO_LARGE` にマップ。部分出力は破棄して error のみ返す (spec §5.2.5: truncate しない方針)。

## 10. `internal/hostapp/dispatch.go`

```go
func Run(stdin io.Reader, stdout, stderr io.Writer) int {
    req, err := readRequest(stdin)
    if err != nil {
        fmt.Fprintln(stderr, "fumi-host: read error:", err)
        return 1
    }

    cfg, _ := config.Load()
    paths, perr := store.Resolve(cfg)
    if perr != nil {
        writeError(stdout, req.ID, "INTERNAL", perr.Error())
        return 0
    }

    var result any
    var errBody *protocol.ErrorBody
    switch req.Op {
    case "getActions":
        result, errBody = handleGetActions(paths)
    case "runScript":
        result, errBody = handleRunScript(context.Background(), cfg, paths, req.Params)
    default:
        errBody = &protocol.ErrorBody{Code: "UNKNOWN_OP", Message: req.Op}
    }

    if errBody != nil {
        writeError(stdout, req.ID, errBody.Code, errBody.Message)
    } else {
        writeOK(stdout, req.ID, result)
    }
    return 0
}
```

- **1 リクエスト処理して main に return**。`for { ReadMessage }` ループは作らない (短命方針)。
- 未捕捉パニックは `recover` で `INTERNAL` エラーに変換。

## 11. テスト戦略

| 対象 | 手段 |
|---|---|
| `protocol/codec.go` | unit: ラウンドトリップ、1 MiB 境界、途中切断 |
| `store/frontmatter.go` | unit: fixture ファイルをディスク上に置き parse |
| `store/scripts.go` | unit: symlink / 親ディレクトリ symlink / `..` / 絶対パス / +x なし、各パターン |
| `runner/runner.go` | integration: 実スクリプト (`echo` / `sleep 10` / `cat`) を spawn、timeout・出力制限を検証 |
| `hostapp/` | integration: stdin に JSON を流し stdout をパース、全 op を end-to-end |

CI: `go test ./...` で回るようにする。macOS 専用なので CI ランナーも macOS。

## 12. セキュリティチェックリスト (spec §9.2 対応)

| 要件 | 実装箇所 |
|---|---|
| `realpath(scriptPath)` が `scripts/` 配下 | `store/scripts.go` #3 |
| `lstat` で通常ファイルかつ非 symlink | `store/scripts.go` #2 |
| シェルを介さず直接 spawn | `runner/runner.go` `exec.Command(absPath)` 引数 0 個、 `sh -c` 不使用 |
| payload は stdin 経由のみ、argv に詰めない | `runner/runner.go` stdin pipe |
| `allowed_origins` を Extension ID に固定 | `internal/manifest/` (CLI 側、host 本体は関知しない) |
| ストアディレクトリ 0700 | `fumi setup` で `os.MkdirAll` + `os.Chmod(0o700)` |
