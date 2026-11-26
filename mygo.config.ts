import { execFileSync } from "node:child_process";
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

// Android's package name must be a valid Java name: no hyphens, so the desktop
// identifier below (which has one, and is the keychain service: never change it)
// cannot be Android's. An Android build, which names "android" in its command
// line or sets SLOPEMENT_ANDROID=1, gets this one; keep it once an APK is shared:
// installed copies update only in place.
const androidBuild =
  process.env.SLOPEMENT_ANDROID === "1" || process.argv.some((a) => a.includes("android"));
const identifier = androidBuild ? "io.github.dariusit.slopement" : "io.github.darius-it.slopement";

// Icons are made from icons/logo-*.svg by `go run ./internal/icons/gen`. iOS
// takes no transparency and rounds the corners itself, so it gets a full-bleed
// square; everything else starts from the rounded tile.
// mygo evaluates this file in a process of its own, without its arguments, so
// they are read off that process's parent (iOS builds only on a Mac, which has
// ps). SLOPEMENT_IOS=1 says it too.
function mygoArgs(): string {
  try {
    return execFileSync("ps", ["-o", "args=", "-p", String(process.ppid)], { encoding: "utf8" });
  } catch {
    return ""; // no ps (Windows): not an iOS build
  }
}
const iosBuild = process.env.SLOPEMENT_IOS === "1" || /(\s|=|,)ios(\s|,|\/|$)/.test(mygoArgs());

export default defineConfig({
  name: "slopement",
  icon: iosBuild ? "icons/ios.png" : "resources/icon.png",
  // The keychain service secrets are stored under: keep it once released.
  identifier,
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
  // "Open with slopement" for the files people send most: it opens "Send to…",
  // where a chat is picked. Viewer role, with the types' real MIME types: slopement
  // is offered in the menu, never made the default for anything.
  fileAssociations: [
    { ext: ["png"], name: "PNG image", mimeType: "image/png", role: "Viewer" },
    { ext: ["jpg", "jpeg"], name: "JPEG image", mimeType: "image/jpeg", role: "Viewer" },
    { ext: ["gif"], name: "GIF image", mimeType: "image/gif", role: "Viewer" },
    { ext: ["webp"], name: "WebP image", mimeType: "image/webp", role: "Viewer" },
    { ext: ["mp4"], name: "MP4 video", mimeType: "video/mp4", role: "Viewer" },
    { ext: ["mov"], name: "QuickTime video", mimeType: "video/quicktime", role: "Viewer" },
    { ext: ["webm"], name: "WebM video", mimeType: "video/webm", role: "Viewer" },
    { ext: ["mp3"], name: "MP3 audio", mimeType: "audio/mpeg", role: "Viewer" },
    { ext: ["m4a"], name: "M4A audio", mimeType: "audio/mp4", role: "Viewer" },
    { ext: ["ogg", "opus"], name: "Ogg audio", mimeType: "audio/ogg", role: "Viewer" },
    { ext: ["wav"], name: "WAV audio", mimeType: "audio/wav", role: "Viewer" },
    { ext: ["pdf"], name: "PDF document", mimeType: "application/pdf", role: "Viewer" },
    { ext: ["txt"], name: "Text document", mimeType: "text/plain", role: "Viewer" },
    { ext: ["zip"], name: "ZIP archive", mimeType: "application/zip", role: "Viewer" },
  ],
  linux: {
    comment: "A minimal Matrix client (super early alpha)",
    categories: ["Network", "Chat"],
    // No WebKitGTK: the packages depend on GTK alone, and on libsecret, which
    // keeps the login in the Secret Service (GNOME Keyring, KWallet…): without
    // it nothing can be saved and signing in fails.
    webview: false,
    depends: { deb: ["libsecret-1-0"], rpm: ["libsecret"], arch: ["libsecret"] },
  },
  macos: {
    dmgTitle: "slopement",
    // The Liquid Glass icon (light and dark) of macOS 26, with an .icns made
    // from it for older systems. Needs Xcode 26+; without it the PNG is used.
    icon: "icons/AppIcon.icon",
    // Calls (Element Call in a web view) ask for these; macOS ends an app that
    // uses them without a reason.
    infoPlist: {
      NSMicrophoneUsageDescription: "slopement uses the microphone in calls.",
      NSCameraUsageDescription: "slopement uses the camera when you turn it on in a call.",
    },
  },
  // iPhone: calls need these, or iOS ends the app when a call asks for them.
  // Set `team` to your Apple developer team ID (ios: { team: "ABCDE12345", … })
  // to build for a device or TestFlight.
  ios: {
    team: "UM5D76P9X8",
    infoPlist: {
      NSMicrophoneUsageDescription: "slopement uses the microphone in calls.",
      NSCameraUsageDescription: "slopement uses the camera when you turn it on in a call.",
    },
  },
  // Android: what calls use (INTERNET and notifications are declared anyway).
  // Builds are signed with the debug key unless `keystore` is set: fine for
  // sharing an .apk by hand, not for Google Play.
  android: {
    permissions: ["RECORD_AUDIO", "CAMERA", "MODIFY_AUDIO_SETTINGS"],
    adaptiveIcon: { foreground: "icons/android-foreground.png", background: "#121116" },
  },
  // Phones: `mygo dev -platform ios` and `-platform android` run the app on a
  // simulator, an emulator or a device (`mygo devices` lists them, and
  // `mygo doctor` tells what is missing).
});
