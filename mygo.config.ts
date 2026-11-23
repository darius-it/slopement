import { existsSync, readFileSync } from "node:fs";
import { defineConfig } from "mygo-cli";

// slopement draws its own UI (no webview), so there is no frontend to build
// or serve: no devUrl, devCommand, buildCommand, frontendDist or bindings.

// The version follows the release tag (the release workflow sets
// SLOPEMENT_VERSION=0.1.0-alpha.2 for the tag v0.1.0-alpha.2): the app compares
// versions to know an update is newer, so each release needs its own.
const version = process.env.SLOPEMENT_VERSION || "0.1.0";

// Updates: the app can check GitHub's releases for a newer, signed version,
// only when the user turns that on (Settings → Privacy). Builds are signed
// with the secret key (MYGO_UPDATER_PRIVATE_KEY in CI); the app trusts only
// the public key, which `mygo keygen` writes as mygo-update.pub: commit that
// file next to this one (or set MYGO_UPDATER_PUBLIC_KEY). Without it the build
// has no updater at all.
const publicKey =
  process.env.MYGO_UPDATER_PUBLIC_KEY ||
  (existsSync("mygo-update.pub") ? readFileSync("mygo-update.pub", "utf8").trim() : "");

export default defineConfig({
  name: "slopement",
  // The keychain service secrets are stored under: keep it once released.
  identifier: "io.github.darius-it.slopement",
  version,
  copyright: "slopement",
  out: "build",
  ...(publicKey
    ? {
        updates: {
          publicKey,
          // The repository the release workflow runs in; the guess is for
          // builds on a computer.
          github: process.env.GITHUB_REPOSITORY || "darius-it/slopement",
          // Small alpha builds: whole archives, no deltas (which need the
          // earlier releases downloaded while building).
          deltas: 0,
        },
      }
    : {}),
  linux: {
    comment: "A minimal Matrix client (super early alpha)",
    categories: ["Network", "Chat"],
    // No WebKitGTK: the packages depend on GTK alone, and on libsecret, which
    // keeps the login in the Secret Service (GNOME Keyring, KWallet…): without
    // it nothing can be saved and signing in fails.
    webview: false,
    depends: { deb: ["libsecret-1-0"], rpm: ["libsecret"], arch: ["libsecret"] },
  },
  macos: { dmgTitle: "slopement" },
  // Phones: `mygo dev -platform ios` and `-platform android` run the app on a
  // simulator, an emulator or a device (`mygo devices` lists them, and
  // `mygo doctor` tells what is missing).
});
