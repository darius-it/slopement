# Known gaps

slopement is a **super early alpha**. This is what is missing or unproven, as
of 2026-11-22. It is not a promise of when anything lands.

## Not built yet
- **Calls** (voice, video, screen share): the buttons are there and do nothing
  useful; call settings are marked "Soon". Planned: Jitsi and MatrixRTC.
- **Moderation:** no kick, ban, report or bulk delete from the app.
- **Private nicknames, notes and room names** (visible only to you).
- **Voice message recording.** Playing them works; recording needs a device
  pass. Opus and AAC audio files do not play (WAV, MP3, Vorbis, FLAC and ALAC do).
- **Creating a new key backup.** Restoring one works; make it in Element.
- **Custom emoji as reactions** (custom emoji work in messages).
- **Joins collapsed into one line** ("Mira, Sam and 3 others joined"); the
  setting hides them, it does not group them.
- **Live maps:** a shared location is a static card that opens in OpenStreetMap.
- **Animated WebP** does not animate. GIF avatars and GIFs do.

## Phones
The phone layout exists and renders, but the app has **never been run on an
iPhone, iPad or Android device**. Treat it as untested. Share extensions and
push notifications are not built.

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
The UI can feel sluggish on some Linux setups (reported while typing). The
cause is not known yet. The AppImage and the `.deb` use the same binary and
your system's GTK 3 and graphics drivers, so they draw the same. To try other
renderers, set these before starting the app:

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
- Updates are opt-in and never automatic. Settings → Privacy → "Check for
  updates at startup" (off by default) asks GitHub once when the app starts;
  Settings → Advanced has "Check for updates now". A newer version is offered
  in a dialog and downloaded only when you press Install; it is checked against
  a signing key built into the app. Copies installed from a package (`.deb`,
  `.rpm`, AppImage, Arch) update through where they came from. Versions before
  the first one with the updater must be replaced by hand.
- GIF and sticker search needs **your own KLIPY API key** (Settings → GIFs);
  builds ship without one, so search and trending stay empty until you add it
  (favorites, folders and sending links work without).
- Data may need resetting between versions while things change (Settings →
  Advanced → Reset local cache does not touch your encryption keys).
