# Chrome Extension 設計 (TypeScript)

対象: spec §5.1 / §6.1 / §6.2。

## 1. ディレクトリ

```
extension/
├── package.json                # dev deps: typescript, @types/chrome のみ
├── tsconfig.json
├── public/                     # tsc 後に dist/ へそのままコピー
│   ├── manifest.json           # Manifest V3 (静的 JSON)
│   ├── popup.html
│   └── icons/                  # 16/32/48/128 px
├── src/
│   ├── background/
│   │   ├── index.ts            # Service Worker エントリ
│   │   ├── native.ts           # chrome.runtime.sendNativeMessage ラッパ
│   │   ├── actions.ts          # actions/list → chrome.userScripts.register の同期
│   │   └── contextMenus.ts     # fumi.contextMenus.register の Service Worker 側ディスパッチャ
│   ├── popup/
│   │   └── popup.ts
│   ├── user-script/
│   │   └── prelude.ts          # fumi.* 実装 (import 禁止、自己完結)
│   └── shared/
│       ├── protocol.ts         # Request/Response/Action 型 (protocol.md §5.1)
│       ├── messages.ts         # User Script ↔ SW 内部メッセージ型
│       └── storage.ts          # chrome.storage のキー定義
└── tests/
    └── *.test.ts               # node:test + node:assert、chrome API は手書き stub
```

**ビルド (バンドラなし)**:
- `tsc` のみでコンパイル。`tsconfig.json` の `compilerOptions`:
  ```jsonc
  {
    "target": "ES2022",
    "module": "ES2022",
    "moduleResolution": "node16",    // 相対 import の拡張子は .js を付ける (TS 4.7+)
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "types": ["chrome"]
  }
  ```
- `package.json` の build script:
  ```jsonc
  {
    "scripts": {
      "build": "tsc && cp -R public/. dist/",
      "watch": "tsc -w",
      "test":  "node --test --experimental-strip-types 'tests/**/*.test.ts'"
    }
  }
  ```
- Service Worker / Popup / UserScript は **各々独立した JS ファイルとして `dist/` に出力**。ES Modules をそのまま使う (MV3 Service Worker は `"type": "module"` で ESM 対応、Popup は `<script type="module">`)。
- **User Script プレリュードのバンドル問題**は次節で解決する。

## 2. Manifest

`public/manifest.json` に静的 JSON として配置。`tsc` の build 後に `dist/` にコピーされる。

```jsonc
{
  "manifest_version": 3,
  "name": "fumi",
  "version": "0.1.0",
  "description": "Bridge browser JS and host-side scripts via Native Messaging.",
  "permissions": ["userScripts", "nativeMessaging", "contextMenus", "storage"],
  "host_permissions": ["<all_urls>"],
  "background": {
    "service_worker": "background/index.js",
    "type": "module"
  },
  "action": { "default_popup": "popup.html" },
  "icons": {
    "16": "icons/16.png",
    "48": "icons/48.png",
    "128": "icons/128.png"
  },
  "key": "<base64-encoded-public-key>"
}
```

- `commands` / `scripting` / `activeTab` などは **不要** (spec §5.1.1)。
- `key` は `fumi-host` の `allowed_origins` に埋め込む unpacked ID と整合させるため必須。リポジトリにコミットしてよい (秘密鍵ではなく公開鍵由来の導出値)。
- `service_worker` は `tsc` 出力である `background/index.js` をそのまま指す。`.ts` からの相対 import は `import { foo } from "./native.js"` のように **`.js` 拡張子込み**で書く必要がある (`moduleResolution: "node16"` の要件)。

## 3. Service Worker (`src/background/`)

### 3.1. エントリ (`index.ts`)

```ts
chrome.runtime.onInstalled.addListener(() => syncActions());
chrome.runtime.onStartup.addListener(() => syncActions());

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  // User Script からの kind:"scripts/run" / "contextMenus/register" を受ける
  // (SW ↔ User Script の内部プロトコル。Host 向け JSON-RPC とは別層)
  handleUserScriptMessage(msg, sender).then(sendResponse);
  return true;  // async response
});

chrome.contextMenus.onClicked.addListener((info, tab) => {
  dispatchContextMenu(info, tab);
});
```

### 3.2. Native Messaging ラッパ (`native.ts`)

Host とは **JSON-RPC 2.0** で通信する (詳細は design/protocol.md)。

```ts
export const HOST_NAME = "com.tkuramot.fumi";   // manifest の name と一致

type Method = Request["method"];
type ParamsOf<M extends Method> = Extract<Request, { method: M }>["params"];

export async function call<R>(
  method: Method,
  params?: ParamsOf<Method>
): Promise<R> {
  const req = { jsonrpc: "2.0" as const, id: crypto.randomUUID(), method, params };
  return new Promise((resolve, reject) => {
    chrome.runtime.sendNativeMessage(HOST_NAME, req, (res) => {
      if (chrome.runtime.lastError) {
        reject(hostUnreachable(chrome.runtime.lastError.message));
        return;
      }
      if ("error" in res) {
        reject(new FumiHostError(res.error));  // error.data.fumiCode で UI 分岐
        return;
      }
      resolve(res.result as R);
    });
  });
}
```

- `result` / `error` の判別は `"error" in res` で行う (JSON-RPC 2.0 の相互排他)。
- 失敗はすべて `throw` で上流に伝播。呼び出し側で `chrome.storage.session` に最終エラーを書いて Popup に見せる。
- `HOST_UNREACHABLE` は Extension 内部ローカルのエラー分類 (ワイヤには流れない)。

### 3.3. アクション同期 (`actions.ts`)

```ts
export async function syncActions() {
  try {
    const { actions } = await call<{ actions: Action[] }>("actions/list");
    await replaceRegisteredScripts(actions);
    await setStatus({ ok: true, count: actions.length, at: Date.now() });
  } catch (e) {
    await setStatus({ ok: false, error: String(e), at: Date.now() });
  }
}

async function replaceRegisteredScripts(actions: Action[]) {
  const existing = await chrome.userScripts.getScripts();
  if (existing.length > 0) {
    await chrome.userScripts.unregister({ ids: existing.map((s) => s.id) });
  }
  if (actions.length === 0) return;
  await chrome.userScripts.register(
    actions.map((a) => ({
      id: `fumi:${a.id}`,
      matches: a.matches,
      excludeMatches: a.excludes,
      world: "USER_SCRIPT",
      runAt: "document_idle",
      js: [
        { code: PRELUDE_JS },     // src/user-script/prelude.ts をバンドルした string
        { code: a.code },
      ],
    }))
  );
}
```

- **全置換方式**: spec §6.1 で差分更新は不要としたため、毎回 `unregister` → `register` で十分。
- `PRELUDE_JS` は **Service Worker 起動時に `fetch(chrome.runtime.getURL("user-script/prelude.js"))` で読み込む**:
  ```ts
  let preludeCache: string | null = null;
  async function getPrelude(): Promise<string> {
    if (preludeCache !== null) return preludeCache;
    const res = await fetch(chrome.runtime.getURL("user-script/prelude.js"));
    preludeCache = await res.text();
    return preludeCache;
  }
  ```
  - `prelude.ts` は **import を持たない自己完結した 1 ファイル** として書く。`tsc` が出力する `dist/user-script/prelude.js` がそのまま Chrome の Extension URL で読める。
  - バンドラを使わないため、prelude.ts は必ず単一ファイルに収める (ユーティリティを分けたくなっても inline する)。サイズは数十行なので問題にならない。
  - SW が停止→再起動しても `chrome.runtime.getURL` + `fetch` は再度実行されるだけ。`chrome.userScripts` 側の登録は永続化されているので `syncActions` を走らせた時のみ再注入される。

### 3.4. ContextMenu ディスパッチャ (`contextMenus.ts`)

`fumi.contextMenus.register`(User Script から `kind: "contextMenus.register"` で SW に送られる)を受け取り、`chrome.contextMenus.create` と `chrome.contextMenus.onClicked` のバインドを担う。SW 側は chrome API を呼ぶだけで、ハンドラ本体は User Script クロージャに残る。

```ts
type Registration = { actionId: string; tabId: number };
const registry = new Map<string, Registration>();

// params は User Script からの { id, title, contexts? } — MVP 範囲のフィールドのみ受理
export async function registerContextMenu(
  params: { id: string; title: string; contexts?: chrome.contextMenus.ContextType[] },
  sender: chrome.runtime.MessageSender
) {
  // 同 id は冪等に上書き (spec §5.1.4)
  try { await chrome.contextMenus.remove(params.id); } catch {}
  await chrome.contextMenus.create({
    id: params.id,
    title: params.title,
    contexts: params.contexts ?? ["page"],
  });
  registry.set(params.id, {
    actionId: sender.documentId ?? "",
    tabId: sender.tab?.id ?? -1,
  });
}

export function dispatchContextMenu(
  info: chrome.contextMenus.OnClickData,
  tab?: chrome.tabs.Tab
) {
  if (!tab?.id) return;
  chrome.tabs.sendMessage(tab.id, {
    kind: "ctxDispatch",
    menuId: info.menuItemId,
    info,     // OnClickData をそのまま転送
    tab,
  });
}
```

- `registry` は MV3 SW では永続しないが、`chrome.contextMenus.create` 側の登録は SW 停止をまたいで維持される。ただし **SW 再起動後に onClicked → User Script へ dispatch しようとしても、対象タブで User Script が再び `fumi.contextMenus.register` を呼び直していなければ handler が存在しない** という問題が残る。対策: ディスパッチは `chrome.tabs.sendMessage` で送るだけにし、**User Script 側で `addListener` してハンドラ本体は User Script クロージャに閉じ込める**。SW 再起動直後のタブでは単に no-op (User Script が未マウントならメッセージは空振り)。

## 4. User Script プレリュード (`src/user-script/prelude.ts`)

**制約**: このファイルは `import` 文を一切持たない。`tsc` の出力 `dist/user-script/prelude.js` が単独で動くようにする (バンドラを使わない前提)。

Service Worker の `chrome.userScripts.register` で各アクションの `code` より前に必ず注入される固定コード。`fumi` グローバルを User Script world の `globalThis` に生やす。

### 4.1. `fumi.*` 命名規則(spec §5.1.4 参照)

- `chrome.*` の名前空間をそのまま写す(`fumi.contextMenus.register`, 将来的に `fumi.notifications.create` など)。
- プロパティ名は chrome の型定義と同名同型。ハンドラ名は chrome のイベント名をそのまま使う(`onClicked`、シグネチャも `(info, tab) => void` と一致)。
- 受理フィールドは MVP で必要なもののみホワイトリスト。未知プロパティは prelude / SW のどちらかで捨てる。
- `fumi.run` のみ chrome.* ラッパーではないためルート直下に置く例外扱い。

### 4.2. 実装

```ts
// prelude.ts (IIFE 化してバンドル)
(() => {
  // SW ↔ User Script の内部メッセージ (Host 向け JSON-RPC とは別層)。
  // SW 側で JSON-RPC に変換して Host に転送する。
  const send = <R>(kind: string, params: unknown): Promise<R> =>
    new Promise((resolve, reject) => {
      chrome.runtime.sendMessage({ kind, params }, (res) => {
        if (chrome.runtime.lastError) { reject(new Error(chrome.runtime.lastError.message)); return; }
        if (res?.error) { reject(new Error(res.error.data?.fumiCode ?? res.error.message ?? "UNKNOWN")); return; }
        resolve(res.result as R);
      });
    });

  type CtxHandler = (
    info: chrome.contextMenus.OnClickData,
    tab?: chrome.tabs.Tab
  ) => void;
  const ctxHandlers = new Map<string | number, CtxHandler>();

  chrome.runtime.onMessage.addListener((msg) => {
    if (msg?.kind === "ctxDispatch") ctxHandlers.get(msg.menuId)?.(msg.info, msg.tab);
  });

  (globalThis as any).fumi = {
    run: (scriptPath: string, payload: unknown, opts?: { timeoutMs?: number }) =>
      send("scripts/run", { scriptPath, payload, ...(opts ?? {}) }),   // SW が JSON-RPC に包んで Host へ

    contextMenus: {
      register: (opts: {
        id: string;
        title: string;
        contexts?: chrome.contextMenus.ContextType[];
        onClicked: CtxHandler;
      }) => {
        ctxHandlers.set(opts.id, opts.onClicked);
        return send("contextMenus/register", {
          id: opts.id,
          title: opts.title,
          contexts: opts.contexts ?? ["page"],
        });
      },
    },
  };
})();
```

- `fumi.run` の戻り値型は `Promise<RunResult>`。**TypeScript 型定義** は `@types/fumi` 相当を `samples/` 側で提供 (オプション)。ユーザーは JS でアクションを書くので型必須ではない。
- `contexts` の既定は `["page"]` (最頻ケース)。
- `onClicked` の引数は `chrome.contextMenus.onClicked` と同じ `(info, tab)` を SW 側ディスパッチャが `chrome.tabs.sendMessage` に載せて届ける。
- MV3 SW が停止してから再起動するケースで `ctxHandlers` は空になるが、User Script 側のクロージャに紐づいているので、**そのタブで再び `fumi.contextMenus.register` が呼ばれるまでは dispatch が空振り** する。これは spec §4.1 の「タブを再読込すれば復活」で許容。

## 5. Popup (`src/popup/`)

spec §5.1.2 の要件のみ。

- 表示項目: 接続状態 / アクション数 / 最後の取得時刻 / 最後のエラー。
- ボタン: 「アクション再取得」。
- 実装: プレーンな HTML + TS (フレームワーク不要)。`chrome.storage.session` を読み、再取得時は SW に `{ kind: "resync" }` を送る。

```ts
document.getElementById("resync")!.addEventListener("click", async () => {
  await chrome.runtime.sendMessage({ kind: "resync" });
  await render();
});
```

`background/index.ts` 側で `onMessage` が `kind: "resync"` を拾って `syncActions()` を呼ぶ。

## 6. `chrome.storage` 利用

| キー | 型 | ストレージ | 用途 |
|---|---|---|---|
| `status` | `{ ok: boolean; count?: number; error?: string; at: number }` | `session` | Popup 表示用 |
| `lastActions` | `Action[]` | `local` | デバッグ用キャッシュ (必須ではない) |

`chrome.storage` に書くのは **キャッシュ+表示用のみ**。設定・スクリプト本体は絶対に書かない (spec §5.1.6)。

## 7. テスト

- **Node 標準の `node:test` + `node:assert`** でユニットテストを書く (Node 18+)。`package.json`:
  ```jsonc
  { "scripts": { "test": "node --test --experimental-strip-types 'tests/**/*.test.ts'" } }
  ```
  (`--experimental-strip-types` は Node 22+。Node 20 系をサポートする場合は `tsc` で `tests/` も JS 化してから `node --test dist/tests/**/*.test.js` に変える)
- Chrome API はテストファイル冒頭で `globalThis.chrome = { runtime: { sendMessage: ..., ... } }` と最小限の stub を手書き。stub は `tests/_stubs/chrome.ts` に切り出して使い回す。
- 対象: `native.ts` のエラーマッピング、`actions.ts` の register 差分ロジック、`prelude.ts` の `fumi.run` Promise 挙動、メッセージ codec。
- E2E は MVP の範囲外。必要になったら Playwright を入れる余地はあるが、`node:test` で基本ロジックはカバー可能。
