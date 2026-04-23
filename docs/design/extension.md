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
└── src/
    ├── background/             # 規則は §1.1
    │   ├── index.ts            # SW lifecycle + 内部メッセージルータ + 薄ハンドラ + event listener
    │   ├── actions.ts          # 実質的オーケストレーションを持つ唯一のドメインファイル
    │   └── chrome/             # 1 ファイル = 1 chrome.* 名前空間スライス
    │       ├── nativeMessaging.ts  # chrome.runtime.sendNativeMessage の Promise/型付け
    │       ├── userScripts.ts      # chrome.userScripts.{register,unregister,getScripts}
    │       └── contextMenus.ts     # chrome.contextMenus.{create,remove}
    ├── popup/
    │   └── popup.ts
    ├── user-script/
    │   └── prelude.ts          # fumi.* 実装 (import 禁止、自己完結) (+ prelude.test.ts)
    └── shared/
        ├── protocol.ts         # Request/Response/Action 型 (protocol.md §5.1)
        ├── messages.ts         # User Script ↔ SW 内部メッセージ型
        ├── storage.ts          # chrome.storage のキー定義
        └── test-stubs/
            └── chrome.ts       # テスト用 chrome API stub (手書き、最小限)
```

**テスト配置方針**: `*.test.ts` は実装ファイルと同じ階層に co-locate する。探索コストを下げ、実装変更時に見落としにくい。本番ビルドから除外するため `tsconfig.build.json` を別に持ち `exclude: ["**/*.test.ts", "**/test-stubs/**"]` を指定する。

### 1.1. background/ レイヤ規則

#### 設計方針

USER_SCRIPT world からは大半の `chrome.*` API に触れない。よって `fumi.*` を増やすたびに「prelude が SW にメッセージ送出 → SW が chrome.* を叩いて返す」定型経路が必要になる。

「**ドメイン層** + **chrome/ 薄ラッパ層**」の 2 層を fumi.* ごとに必ず作るルールは、現実の `fumi.*` ハンドラの大半が薄いパススルーである以上 ceremony になる。代わりに次の原則を採る:

1. **`chrome/` ディレクトリは常に切る**。1 chrome.* 名前空間スライス = 1 ファイル。
2. **薄い fumi.* ハンドラは index.ts のルータに直接書く**。空のラッパ関数を作らない。
3. **複雑化したら独立ファイルに昇格**。`actions.ts` がその先例(Host 呼び出し + userScripts 全置換 + prelude 注入 + status 書き込みで実質的オーケストレーション)。
4. 「一度独立ファイルに昇格したものは `chrome.*` を直接呼ばず、必ず `chrome/<namespace>.ts` 経由で呼ぶ」(テスト時に chrome/ 境界だけ stub すれば済む)。
5. event listener (`chrome.runtime.onMessage`, `onInstalled`, `chrome.contextMenus.onClicked`) はラップ価値が薄いため **例外的に index.ts で chrome.* を直触り**。ラップするとコールバック登録の所有権が分散する。

#### `chrome/` の存在理由と典型経路

`chrome/` は将来の `fumi.*` 追加経路の「Chrome API 直前の薄ラッパ」を集める場所。例として将来 `fumi.tabs.update` を追加するとき:

```
action            → fumi.tabs.update(...)                (prelude)
                  → sendMessage { kind: "tabs/update", params }   (prelude → SW)
                  → index.ts ルータ                                (薄ハンドラ)
                  → chrome/tabs.ts                                 (★薄ラッパ)
                  → chrome.tabs.update(...)                        (Chrome API)
```

ドメインファイル `background/tabs.ts` は **作らない**。ハンドラがバリデーション数行で済むなら index.ts に書き、複雑化した時点で昇格する。

#### `chrome/` の中身ルール

- 内容は Promise 化 / 型付け / chrome 操作パターン (batch unregister→register 等) まで。冪等化のような fumi セマンティクスはここに入れない (§4.3 命名規則 7)。
- **fumi 固有の defaults** (`contexts: ["page"]` 等) や **ドメイン状態** (アクション ID、JSON-RPC エンベロープ) は持ち込まない。これらはルータ/ハンドラ側で埋める。
- 命名は扱う chrome.* スライスを表す名 (`nativeMessaging.ts` など)。`runtime.ts` のような名前空間全体名は、全体をラップする錯覚を生むので使わない。
- `chrome/` は 2 系統が混在するが扱いは同じ:
  - `fumi.*` の裏側: `nativeMessaging.ts` (← `fumi.run`)、`contextMenus.ts` (← `fumi.contextMenus.create`)。spec §5.1.4 で増えていく
  - 内部インフラ: `userScripts.ts` (action 自体の登録機構。`fumi.userScripts.*` は提供しない)

#### 新しい chrome.* 名前空間を扱うとき

1. `chrome/<namespace>.ts` を作り必要なメソッドを薄くラップ。
2. index.ts ルータに `kind` ハンドラを追加 (defaults はここで埋める)。
3. prelude に `fumi.<namespace>.<method>` を追加 (spec §5.1.4 の規則)。
4. ハンドラが数行を超えたり状態を持ち始めたら `background/<namespace>.ts` に昇格。

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
      "build": "tsc -p tsconfig.build.json && cp -R public/. dist/",
      "watch": "tsc -p tsconfig.build.json -w",
      "test":  "node --test --experimental-strip-types 'src/**/*.test.ts'"
    }
  }
  ```
  - `tsconfig.build.json` は `extends: \"./tsconfig.json\"` + `exclude: [\"**/*.test.ts\", \"**/test-stubs/**\"]`。
  - **Node 22+ を要求** (`--experimental-strip-types` 利用)。サポート範囲を広げないことで dual パスを避ける。
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
- `service_worker` は `tsc` 出力である `background/index.js` をそのまま指す。`.ts` からの相対 import は `import { foo } from "./chrome/nativeMessaging.js"` のように **`.js` 拡張子込み**で書く必要がある (`moduleResolution: "node16"` の要件)。

## 3. Service Worker (`src/background/`)

### 3.1. エントリ (`index.ts`)

```ts
chrome.runtime.onInstalled.addListener(async () => {
  // USER_SCRIPT world から chrome.runtime.sendMessage を使えるようにする (Chrome 120+)。
  // これを呼ばないと prelude の send() が黙って動かない。
  await chrome.userScripts.configureWorld({ messaging: true });
  await syncActions();
});
chrome.runtime.onStartup.addListener(() => syncActions());

// SW ↔ User Script の内部メッセージルータ (Host 向け JSON-RPC とは別層)。
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  routeUserScriptMessage(msg, sender).then(sendResponse).catch((e) =>
    sendResponse({ error: { message: String(e) } })
  );
  return true;  // async response
});

// 1 行の dispatcher なので独立ドメインファイルにせず onClicked 内に直接書く
chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (!tab?.id) return;
  chrome.tabs.sendMessage(tab.id, { kind: "ctxDispatch", menuId: info.menuItemId, info, tab });
});

async function routeUserScriptMessage(
  msg: { kind: string; params?: unknown },
  _sender: chrome.runtime.MessageSender
) {
  switch (msg.kind) {
    case "scripts/run":
      return { result: await call("scripts/run", msg.params) };
    case "contextMenus/create": {
      // 薄ハンドラ: defaults 埋めて chrome 層に流すだけ。
      // 重複時のセマンティクスは chrome.contextMenus.create と同一 (= エラー)。
      // 「毎ページロードで再実行されても落ちない」ようにしたい場合は action 側で
      // fumi.contextMenus.remove(id).catch(() => {}) → create() のイディオムを使う (spec §5.1.4)。
      const p = msg.params as { id: string; title: string; contexts?: chrome.contextMenus.ContextType[] };
      return { result: await cm.create({ ...p, contexts: p.contexts ?? ["page"] }) };
    }
    case "contextMenus/remove": {
      const p = msg.params as { menuItemId: string | number };
      return { result: await cm.remove(p.menuItemId) };
    }
    case "resync":
      return { result: await syncActions() };
    default:
      throw new Error(`unknown kind: ${msg.kind}`);
  }
}
```

ルータは `index.ts` 内に閉じる。SW ↔ Host (JSON-RPC) と SW ↔ User Script (`{kind, params}`) は別レイヤである点に注意。fumi.* ハンドラは数行に収まる限り index.ts 内に置き、複雑化した時点で `background/<namespace>.ts` に昇格する (現状で昇格しているのは `actions.ts` のみ)。

### 3.2. Native Messaging ラッパ (`chrome/nativeMessaging.ts`)

Host とは **JSON-RPC 2.0** で通信する (詳細は design/protocol.md)。

```ts
export const HOST_NAME = "com.tkuramot.fumi";   // manifest の name と一致

type Method = Request["method"];
type ParamsOf<M extends Method> = Extract<Request, { method: M }>["params"];
type ResultOf<M extends Method> = Extract<Response, { method?: M }>["result"];

// M を引数として受けることで params/result の型がメソッド毎に確定する
// (旧版は <R> 単独でジェネリクスが効かず、params がユニオン化していた)
export async function call<M extends Method>(
  method: M,
  params?: ParamsOf<M>
): Promise<ResultOf<M>> {
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
      resolve(res.result);
    });
  });
}
```

- `result` / `error` の判別は `"error" in res` で行う (JSON-RPC 2.0 の相互排他)。
- 失敗はすべて `throw` で上流に伝播。呼び出し側で `chrome.storage.session` に最終エラーを書いて Popup に見せる。
- `HOST_UNREACHABLE` は Extension 内部ローカルのエラー分類 (ワイヤには流れない)。

### 3.3. アクション同期 (`actions.ts`)

現状で唯一の独立ドメインファイル(§1.1 の昇格基準を満たす)。`chrome.*` を直接呼ばず `chrome/userScripts.ts` と `chrome/nativeMessaging.ts` のみを経由する。

```ts
export async function syncActions() {
  try {
    const { actions } = await call("actions/list");
    await replaceRegisteredScripts(actions);
    await setStatus({ ok: true, count: actions.length, at: Date.now() });
  } catch (e) {
    await setStatus({ ok: false, error: String(e), at: Date.now() });
  }
}

async function replaceRegisteredScripts(actions: Action[]) {
  // chrome.userScripts を直接触らず chrome/userScripts.ts 経由 (規則 §1.1)
  const existing = await us.list();
  if (existing.length > 0) {
    await us.unregister(existing.map((s) => s.id));
  }
  if (actions.length === 0) return;
  await us.register(
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

### 3.4. ContextMenus 薄ラッパ (`chrome/contextMenus.ts`)

ドメイン層 `background/contextMenus.ts` は **置かない** (§1.1 の昇格基準に達しない)。`fumi.contextMenus.create` ハンドラは index.ts ルータに直接書き、chrome 操作だけをこの薄ラッパに切り出す。dispatcher (1 行) は §3.1 の `onClicked` リスナ内に直接書く。

エクスポート関数名は chrome.* メソッド名と完全一致させる(命名規則 §4.1-7)。冪等化は fumi セマンティクスなのでここには含めず、ルータ側で `remove` → `create` の順で組み立てる。

```ts
// chrome/contextMenus.ts — 薄ラッパ。fumi 知識を持たない。
// 引数型は chrome の CreateProperties をそのまま参照 (命名規則 §4.1-3)。

export const create = (props: chrome.contextMenus.CreateProperties): Promise<void> =>
  new Promise((resolve, reject) => {
    chrome.contextMenus.create(props, () => {
      const err = chrome.runtime.lastError;
      err ? reject(new Error(err.message)) : resolve();
    });
  });

export const remove = (menuItemId: string | number): Promise<void> =>
  chrome.contextMenus.remove(menuItemId);
```

**設計上の注意点**:

- SW 側は dispatch 先タブを `onClicked` の `tab` 引数から直接得るので、register 時の sender 情報は保存しない (registry Map 不要)。
- MV3 SW 停止後も `chrome.contextMenus.create` 側の登録は維持される。ただし **SW 再起動後に onClicked → User Script へ dispatch しようとしても、対象タブで User Script が再び `fumi.contextMenus.register` を呼び直していなければ handler が存在しない**。対策: dispatch は `chrome.tabs.sendMessage` で送るだけにし、**ハンドラ本体は USER_SCRIPT クロージャに閉じ込める** (§4.2)。SW 再起動直後のタブでは単に no-op (USER_SCRIPT が未マウントなら空振り)。spec §4.1「タブを再読込すれば復活」で許容。

## 4. User Script プレリュード (`src/user-script/prelude.ts`)

**制約**: このファイルは `import` 文を一切持たない。`tsc` の出力 `dist/user-script/prelude.js` が単独で動くようにする (バンドラを使わない前提)。

Service Worker の `chrome.userScripts.register` で各アクションの `code` より前に必ず注入される固定コード。`fumi` グローバルを User Script world の `globalThis` に生やす。

### 4.1. `fumi.*` 命名規則(spec §5.1.4 を厳格化)

すべての層で **chrome.* と命名を 1:1 対応** させる。これは prelude / SW ルータの kind / chrome 層ラッパすべてに適用する。

1. **名前空間**: `fumi.<chromeNamespace>` ↔ `chrome.<chromeNamespace>`。
   例: `fumi.contextMenus.*` ↔ `chrome.contextMenus.*`、将来の `fumi.tabs.*` ↔ `chrome.tabs.*`。
2. **メソッド名は chrome.* と完全一致**。意訳しない (`register` ではなく `create` に揃える、`update` を `set` に変えない、等)。
   例: `fumi.contextMenus.create` ↔ `chrome.contextMenus.create`。
3. **引数名・型は chrome.* の型定義と同じものをそのまま使う** (CreateProperties 等)。
4. **受理フィールドは MVP で必要なもののみホワイトリスト**。chrome 側にあっても使わないフィールドは prelude / SW のどちらかで捨てる。捨てる側はホワイトリストの subset 化のみ行い、**フィールド名のリネーム・型変換・別名追加は禁止**。将来必要になったら同名で素直に追加。
5. **イベントハンドラは chrome イベント名をそのまま prelude の引数キーとして使う** (`onClicked`、シグネチャも `(info, tab) => void`)。chrome.* では `addListener` で別途登録する形だが、prelude では create 引数に同梱する API 形にしてよい(これは prelude の便宜であり、ハンドラ名・シグネチャは chrome に従う)。
6. **SW 内部メッセージの `kind`** も `<chromeNamespace>/<method>` に揃える (例: `contextMenus/create`)。
7. **chrome 層 (`chrome/<namespace>.ts`) のエクスポート関数名も chrome.* メソッド名と一致** させる (`create`, `remove`, `update`, ...)。fumi セマンティクスの追加 (冪等化等) はここに含めず、ルータ側で組み立てる(薄ラッパ原則 §1.1)。
8. **例外**: `fumi.run` は chrome.* ラッパーではないためルート直下に置く。これだけが命名規則の対象外。

### 4.2. 実装

**前提**: USER_SCRIPT world からは `chrome.contextMenus` 等の拡張 API を直接呼べない。`chrome.runtime.sendMessage` のみ Chrome 120+ で例外的に許可される (要 `chrome.userScripts.configureWorld({ messaging: true })`、§3.1)。よって `fumi.*` のすべてのメソッドは `send()` 経由で SW にメッセージを投げて実行を依頼する。`onClicked` のような関数引数は sendMessage でシリアライズできないため、prelude のクロージャに保持し SW からは `{kind:"ctxDispatch"}` で ID と info だけを受け取って呼び出す。

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
      // 命名・セマンティクスは chrome.contextMenus.* に完全一致させる (§4.1)。
      // 重複 id でのエラーも chrome.* と同じ。冪等化したい action は remove → create のイディオムを使う。
      create: (props: {
        id: string;
        title: string;
        contexts?: chrome.contextMenus.ContextType[];
        onClicked: CtxHandler;
      }) => {
        ctxHandlers.set(props.id, props.onClicked);
        return send("contextMenus/create", {
          id: props.id,
          title: props.title,
          contexts: props.contexts,    // defaults は SW 側で埋める (defaults を二重に持たない)
        });
      },
      remove: (menuItemId: string | number) => {
        ctxHandlers.delete(menuItemId);    // クロージャ側のハンドラも掃除
        return send("contextMenus/remove", { menuItemId });
      },
    },
  };
})();
```

- `fumi.run` の戻り値型は `Promise<RunResult>`。**TypeScript 型定義** は `@types/fumi` 相当を `samples/` 側で提供 (オプション)。ユーザーは JS でアクションを書くので型必須ではない。
- `contexts` の既定は `["page"]` (最頻ケース)。
- `onClicked` の引数は `chrome.contextMenus.onClicked` と同じ `(info, tab)` を SW 側ディスパッチャが `chrome.tabs.sendMessage` に載せて届ける。
- MV3 SW が停止してから再起動するケースで `ctxHandlers` は空になるが、User Script 側のクロージャに紐づいているので、**そのタブで再び `fumi.contextMenus.create` が呼ばれるまでは dispatch が空振り** する。これは spec §4.1 の「タブを再読込すれば復活」で許容。

## 5. Popup (`src/popup/`)

spec §5.1.2 の要件のみ。

- 表示項目: 接続状態 / アクション数 / 最後の取得時刻 / 最後のエラー。
- ボタン: 「アクション再取得」。
- 実装: プレーンな HTML + TS (フレームワーク不要)。`chrome.storage.session` を読み、再取得時は SW に `{ kind: "resync" }` を送る。
- `public/popup.html` は `<script type="module" src="popup/popup.js"></script>` を含む (`tsc` 出力先と一致させる)。`web_accessible_resources` の宣言は不要 (SW / popup ともに自拡張内 fetch / load であり、ページからの参照ではない)。

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

- **Node 標準の `node:test` + `node:assert`** でユニットテストを書く (Node 22+)。
  - 採用理由: 依存ゼロ。Vitest/Jest を入れる利得 (watch/snapshot/parallel) は本拡張の規模では小さく、デプロイ可搬性 (`@types/chrome` 1 個だけ) の維持を優先。watch やスナップショットが必要になったら再評価。
- **配置**: `*.test.ts` は実装ファイルと同じ階層に co-locate (`src/background/native.test.ts` など)。本番ビルドからは §1 の `tsconfig.build.json` で除外する。
- `package.json`:
  ```jsonc
  { "scripts": { "test": "node --test --experimental-strip-types 'src/**/*.test.ts'" } }
  ```
- Chrome API は `src/shared/test-stubs/chrome.ts` に最小限の stub を手書きして使い回す。`globalThis.chrome = makeChromeStub({ ... })` 形式で、テスト毎に必要なメソッドだけ override 可能にする。
- 対象: `native.ts` のエラーマッピング (`HOST_UNREACHABLE` / `FumiHostError`)、`actions.ts` の `replaceRegisteredScripts` の全置換挙動、`contextMenus.ts` の冪等 `remove`→`create`、`prelude.ts` の `fumi.run` Promise 挙動と `ctxDispatch` ハンドラ呼び出し、`routeUserScriptMessage` の分岐。
- E2E は MVP の範囲外。必要になったら Playwright を入れる余地はあるが、`node:test` で基本ロジックはカバー可能。
