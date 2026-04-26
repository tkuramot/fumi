import assert from "node:assert/strict";
import { test } from "node:test";
import {
  clearChromeStub,
  installChromeStub,
  makeChromeStub,
} from "./test-stubs/chrome.js";
import {
  getStatus,
  LAST_ACTIONS_KEY,
  setLastActions,
  setStatus,
  STATUS_KEY,
} from "./storage.js";
import type { Action } from "./protocol.js";

function setupStorageStub(): {
  session: Record<string, unknown>;
  local: Record<string, unknown>;
} {
  const stores = { session: {} as Record<string, unknown>, local: {} as Record<string, unknown> };
  installChromeStub(
    makeChromeStub({
      runtime: {},
      storage: {
        session: {
          get: async (key: string | string[]) => {
            const k = Array.isArray(key) ? key[0] : key;
            return { [k]: stores.session[k] };
          },
          set: async (items: Record<string, unknown>) => {
            Object.assign(stores.session, items);
          },
        },
        local: {
          get: async (key: string | string[]) => {
            const k = Array.isArray(key) ? key[0] : key;
            return { [k]: stores.local[k] };
          },
          set: async (items: Record<string, unknown>) => {
            Object.assign(stores.local, items);
          },
        },
      },
    }),
  );
  return stores;
}

test("setStatus() writes to session storage under STATUS_KEY", async () => {
  const stores = setupStorageStub();
  try {
    await setStatus({ ok: true, count: 3, at: 1000 });
    assert.deepEqual(stores.session[STATUS_KEY], { ok: true, count: 3, at: 1000 });
  } finally {
    clearChromeStub();
  }
});

test("setStatus() writes error status", async () => {
  const stores = setupStorageStub();
  try {
    await setStatus({ ok: false, error: "host unreachable", at: 2000 });
    assert.deepEqual(stores.session[STATUS_KEY], {
      ok: false,
      error: "host unreachable",
      at: 2000,
    });
  } finally {
    clearChromeStub();
  }
});

test("getStatus() returns the value previously set", async () => {
  setupStorageStub();
  try {
    await setStatus({ ok: true, count: 5, at: 3000 });
    const status = await getStatus();
    assert.deepEqual(status, { ok: true, count: 5, at: 3000 });
  } finally {
    clearChromeStub();
  }
});

test("getStatus() returns undefined when nothing is stored", async () => {
  setupStorageStub();
  try {
    const status = await getStatus();
    assert.equal(status, undefined);
  } finally {
    clearChromeStub();
  }
});

test("setLastActions() writes to local storage under LAST_ACTIONS_KEY", async () => {
  const stores = setupStorageStub();
  try {
    const actions: Action[] = [
      {
        id: "my-action",
        path: "my-action.js",
        matches: ["https://example.com/*"],
        excludes: [],
        code: "console.log('hi');",
      },
    ];
    await setLastActions(actions);
    assert.deepEqual(stores.local[LAST_ACTIONS_KEY], actions);
  } finally {
    clearChromeStub();
  }
});

test("setLastActions() writes empty array", async () => {
  const stores = setupStorageStub();
  try {
    await setLastActions([]);
    assert.deepEqual(stores.local[LAST_ACTIONS_KEY], []);
  } finally {
    clearChromeStub();
  }
});
