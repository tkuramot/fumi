# 設計ドキュメント: fumi

[spec.md](../spec.md) の要件を、実装可能な粒度まで落とし込んだ設計ドキュメント群。

## 言語・ツールチェーン

| コンポーネント | 言語 | ビルド |
|---|---|---|
| Chrome Extension | TypeScript | `tsc` のみ (バンドラなし) |
| Native Messaging Host (`fumi-host`) | Go | `go build` |
| CLI (`fumi`) | Go | `go build` |

Go 側は **単一 Go module** (`go.mod` をリポジトリルートに配置) とし、`host/` と `cli/` をそれぞれの `main` パッケージ、共有コードを `internal/` に集約する。TypeScript 側は `extension/` 単独の npm プロジェクト。

## 依存ポリシー

**原則として標準ライブラリのみで実装する**。第三者ライブラリを採用するのは、以下を満たす場合に限る:

1. 標準ライブラリに同等機能が存在しない
2. 自前実装は仕様自体が非自明、または相当なコード量になる (= 車輪の再発明コストが高い)
3. 採用候補が**デファクトかつ小さく**、推移的依存が限定的

現時点で採用する非標準ライブラリ:

| ライブラリ | 場所 | 採用理由 |
|---|---|---|
| `github.com/BurntSushi/toml` | Go | TOML は空白/クォート/コメント/型など仕様が広く、最小限でも parser 自作は割に合わない。spec §5.2.7 が TOML を指定 |
| `github.com/urfave/cli/v2` | Go (CLI のみ) | `fumi <resource> <verb>` の 2 段構造 + flag パース + help 生成を stdlib `flag` で書くと 200 行以上の boilerplate になる。`cli/v2` は推移的依存が小さく、cobra より軽量 ([cli.md §2](./cli.md)) |
| `@types/chrome` | TypeScript (dev only) | 型定義。事実上の標準、実行時コードは生成しない |

**採用しない** (stdlib / 手書きで十分):

| 候補 | 代替 |
|---|---|
| `spf13/cobra` | 機能過剰。`urfave/cli/v2` の方が軽量 |
| `fatih/color` | MVP では無色。必要になったら `\x1b[...m` を数行 |
| Vite, webpack, esbuild | `tsc` のみ。User Script プレリュードは SW から `fetch(chrome.runtime.getURL(...))` で読み込み ([extension.md §3](./extension.md)) |
| Vitest, Jest | `node:test` + `node:assert` (Node 18+ 標準) |

新規依存を追加する場合は、この表を更新した上で採用理由を明記する。

## ドキュメント構成

| ファイル | 内容 |
|---|---|
| [architecture.md](./architecture.md) | リポジトリ構成、プロセス境界、主要シーケンス |
| [protocol.md](./protocol.md) | Native Messaging プロトコル・メッセージ型・エラーコード |
| [extension.md](./extension.md) | Chrome Extension (TypeScript) の内部設計 |
| [host.md](./host.md) | `fumi-host` (Go) の内部設計 |
| [cli.md](./cli.md) | `fumi` CLI (Go) の内部設計 |

## 設計の重点

spec §1.2 / §9 の原則をそのまま実装に写経する:

1. **ストアが唯一の真実の源**。Extension / Host どちらも状態を持たない。
2. **Host に書き込み系 API は存在しない**。`actions/list` / `scripts/run` のみ。
3. **短命 Host**。`sendNativeMessage` ごとに spawn → 応答 → exit。
4. **`scripts/` 配下に限定**。`realpath` + `lstat` で厳格に検証し、シェルを介さず直接 spawn。

各ドキュメントはこの 4 点を前提に、言語固有の具象(パッケージ分割・型・API 呼び出し順)を決める。
