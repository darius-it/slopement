import { defineConfig } from "mygo-cli";

// slopement draws its own UI (no webview), so there is no frontend to build
// or serve: no devUrl, devCommand, buildCommand, frontendDist or bindings.
export default defineConfig({
  name: "slopement",
  // The keychain service secrets are stored under: keep it once released.
  identifier: "io.github.darius-it.slopement",
  version: "0.1.0",
  copyright: "slopement",
  out: "build",
  linux: {
    comment: "A minimal Matrix client (super early alpha)",
    categories: ["Network", "Chat"],
    // No WebKitGTK: the packages depend on GTK alone.
    webview: false,
  },
  macos: { dmgTitle: "slopement" },
  // Phones: `mygo dev -platform ios` and `-platform android` run the app on a
  // simulator, an emulator or a device (`mygo devices` lists them, and
  // `mygo doctor` tells what is missing).
});
