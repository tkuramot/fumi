import assert from "node:assert/strict";
import { test } from "node:test";
import {
  clearChromeStub,
  installChromeStub,
  makeChromeStub,
} from "../../shared/test-stubs/chrome.js";
import { create, remove } from "./contextMenus.js";

test("create() resolves when no lastError", async () => {
  installChromeStub(
    makeChromeStub({
      runtime: {},
      contextMenus: {
        create: (_props: unknown, cb?: () => void) => {
          cb?.();
        },
        onClicked: { addListener: () => {} },
      },
    }),
  );
  try {
    await assert.doesNotReject(() =>
      create({ id: "item1", title: "My Item" } as chrome.contextMenus.CreateProperties),
    );
  } finally {
    clearChromeStub();
  }
});

test("create() rejects when lastError is set", async () => {
  let capturedCallback: (() => void) | undefined;
  installChromeStub(
    makeChromeStub({
      runtime: {
        lastError: { message: "Duplicate id" },
      },
      contextMenus: {
        create: (_props: unknown, cb?: () => void) => {
          capturedCallback = cb;
          cb?.();
        },
        onClicked: { addListener: () => {} },
      },
    }),
  );
  try {
    await assert.rejects(
      () =>
        create({ id: "item1", title: "My Item" } as chrome.contextMenus.CreateProperties),
      (err: unknown) => err instanceof Error && (err as Error).message === "Duplicate id",
    );
  } finally {
    clearChromeStub();
    void capturedCallback; // suppress unused warning
  }
});

test("remove() resolves when no lastError", async () => {
  installChromeStub(
    makeChromeStub({
      runtime: {},
      contextMenus: {
        remove: (_id: string | number, cb?: () => void) => {
          cb?.();
          return Promise.resolve();
        },
        onClicked: { addListener: () => {} },
      },
    }),
  );
  try {
    await assert.doesNotReject(() => remove("item1"));
  } finally {
    clearChromeStub();
  }
});

test("remove() rejects when lastError is set", async () => {
  installChromeStub(
    makeChromeStub({
      runtime: {
        lastError: { message: "No such menu item" },
      },
      contextMenus: {
        remove: (_id: string | number, cb?: () => void) => {
          cb?.();
          return Promise.resolve();
        },
        onClicked: { addListener: () => {} },
      },
    }),
  );
  try {
    await assert.rejects(
      () => remove("item1"),
      (err: unknown) => err instanceof Error && (err as Error).message === "No such menu item",
    );
  } finally {
    clearChromeStub();
  }
});
