# Known gaps

slopement is a **super early alpha**. This is what is missing or unproven, as
of 2026-11-26. It is not a promise of when anything lands.

## Not built yet
- **Calls** are an early prototype: MatrixRTC through Element Call, whose page
  loads from call.element.io (Element's server) — the app asks before the first
  call, and Settings → Calls takes that back. It needs a LiveKit call service on
  your homeserver. Tried between two people on macOS only; group calls, screen
  sharing, Linux, Windows and phones are untested. Jitsi was dropped on
  purpose: MatrixRTC is the only call system. Element Call is AGPL-3.0, so it
  is not bundled with the app.
- **Moderation:** no kick, ban, report or bulk delete from the app.
- **Voice message recording is unproven.** It is built (the `modules/mic`
  native module: AVFoundation on Macs and iPhones, AudioRecord on Android) and
  tested against a fake microphone only: it has never recorded on real hardware.
  There is no microphone code for Linux and Windows, so no mic button there. It
  sends WAV (about 2 MB a minute), not Opus. Playing voice messages works (Opus
  and the usual formats; AAC is not played).
- **Creating a new key backup**, on purpose: restoring one works, but a new
  backup holds the keys to all your encrypted history, so it is left to Element,
  which has had that code tested for years.
- **Live location sharing.** A shared place shows as a map when you turn maps
  on (Settings → Privacy → Maps); a place that moves (a "live location") is not
  followed, and sending your location is not built.
- **Hardly tested:** animated WebP, Opus playback and custom emoji reactions are
  checked against samples and a fake server only; the Opus decoder is a young
  pure-Go library, so report a voice message that sounds wrong or won't play.

## Tray
"Minimize to tray" and "When the window is closed → Keep in tray" (Settings →
Advanced, off by default) have only been built from the documentation, and the
unread count on the tray icon too, never tried on a real desktop. On Linux the
tray needs `libayatana-appindicator3` (and a panel that shows tray icons: GNOME
needs its AppIndicator extension); without it the settings turn themselves off.

## Phones
The app has been tried on an iPhone, but only lightly; **iPad and Android are
untested**. Releases carry desktop builds only: phone builds are made from the
source. Push notifications are not built.

- **Icons:** iOS gets one flat dark icon (no light or dark variants), and
  Android has no themed icon.

- **Sharing into slopement:** on desktops, "Open with" and the Dock drop work as
  built, but only from the documentation; on Android the share sheet is a plugin
  (`sharetarget/`) never compiled or run on a device. **iPhone has no share
  extension**: an iOS share extension is a separate app-extension target, and
  mygo 0.10 builds the app without one. It needs support in mygo (or a custom
  Xcode project), so it is waiting on that.

## Not verified
- Only tried with a few accounts on a few servers. Not tested with large
  accounts (1,000 rooms) or very large rooms (50,000 members).
- Checked against Element and bridged rooms by hand only; the compatibility
  fixtures in `testdata/events/` are written from the documented event
  shapes, not captured from live traffic.
- Previews fetched from this device, and identity reset on a server that uses
  an account service, have only run against fakes.
- Sign-in uses the classic `/sync`; sliding sync is not used.

## Linux performance
The sluggish typing reported earlier on Linux is fixed (confirmed 2026-11-23).
If a setup still feels slow, the AppImage and the `.deb` use the same binary
and your system's GTK 3 and graphics drivers, so they draw the same. To try
other renderers, set these before starting the app:

```sh
MYGO_GPU=0 ./slopement-x86_64.AppImage    # draw on the CPU
MYGO_GPU=1 ./slopement-x86_64.AppImage    # force OpenGL, even if it runs in software
GDK_BACKEND=wayland ./slopement-x86_64.AppImage   # or x11, to compare
```

To report it, run with frame timing on while typing and send the output, with
your distro and whether it is a virtual machine:

```sh
MYGO_FRAME_STATS=1 ./slopement-x86_64.AppImage
```

Each slow frame says how long building, layout, painting and presenting took,
and whether it was drawn on the GPU or the CPU.

## Builds
- **Not signed or notarized.** macOS, Windows and Linux builds make your
  system warn you once (see the release notes for how to open them).
- Arch Linux: each release has a `PKGBUILD` (x86-64) and `PKGBUILD-aarch64` that
  fetch that release's files: `curl -LO <release>/PKGBUILD && makepkg -si`. It
  installs the unsigned alpha build, not from the AUR.
- Updates are opt-in and never automatic. Settings → About → "Check for
  updates at startup" (off by default) asks GitHub once when the app starts,
  and "Check for updates now" is beside it. A newer version is offered
  in a dialog and downloaded only when you press Install; it is checked against
  a signing key built into the app. Copies installed from a package (`.deb`,
  `.rpm`, AppImage, Arch) update through where they came from. Versions before
  the first one with the updater must be replaced by hand.
- GIF and sticker search needs **your own KLIPY API key** (Settings → GIFs);
  builds ship without one, so search and trending stay empty until you add it
  (favorites, folders and sending links work without).
- Data may need resetting between versions while things change (Settings →
  Advanced → Reset local cache does not touch your encryption keys).
