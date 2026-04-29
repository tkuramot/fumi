// Injects the public "key" into a built manifest.json so an unpacked Chrome
// install resolves to the same extension ID as the Chrome Web Store build
// (see cmd/fumi/constants.go). Never ship a manifest with this field to CWS —
// the store rejects uploads that include "key".
import { readFileSync, writeFileSync } from "node:fs";

const DEV_KEY =
  "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA07DxfESpUiSYeni4rUC7" +
  "hUHdwaJn6xynq5pCK8ATWkkYfX4odXgclTkUgjTegPNSEpVAJoPKIXhQIo6nMMbi" +
  "WAK95XwXQQqcfyADaOWl8GblDc2ziNaAhmXi1HEFcHoTsmNtTFah9tbGgJdt69ZE" +
  "17a306+zpeU4HDwctvJa0tzwYDEwwP6SiJQaDNOZhGjAVzLsc2TP9i/OUW4EyQgP" +
  "G5gXrWhylQxEsvlC8Wk/nDF/im9SmucrABMZRDy4rrlDQ9nb3cCdUTnE2LndI6AB" +
  "ggXk6XnFaTzOfYpcjPHAbqwb3tVT73dMEbXodKv9wWsYEYhyNlgwYuX0GjpULopA" +
  "AwIDAQAB";

const [, , path] = process.argv;
if (!path) {
  console.error("usage: inject-dev-key.mjs <manifest.json>");
  process.exit(1);
}

const manifest = JSON.parse(readFileSync(path, "utf8"));
manifest.key = DEV_KEY;
writeFileSync(path, `${JSON.stringify(manifest, null, 2)}\n`);
