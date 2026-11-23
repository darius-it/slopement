# Changelog

What is new in each release, in plain words. The release workflow puts the
section named after the tag (or, failing that, "Unreleased") at the top of the
GitHub release notes. Add a line under **Unreleased** for every change a user
would notice; when you tag a release, rename that heading to the tag
(`## v0.2.0-alpha.1`) and start a fresh **Unreleased** above it.

## Unreleased

### Added
- **Notification sound** on every desktop: a soft two-note chime of the app's own
  (Settings → Notifications → Sound), at most once every 1.5 seconds, and it
  plays even when the banners are off. The system's own sound is no longer used.
- **Unread count on the icons:** a red dot with the number on the tray icon, and
  the count on the Dock or launcher icon. Muted rooms are not counted.
- **Settings → Advanced → Window → When the window is closed:** Quit (as
  before) or Keep in tray, which hides the window and keeps slopement running so
  messages and notifications still arrive. Quit from the tray menu or ⌘Q still
  ends it.
- **↑ in an empty message box edits your last message** (on a computer).
- **Private names for rooms and chats:** "Private name…" in a room's menu (and
  the phone's room sheet) gives it a name only you see, on all your devices. It
  is stored with your account, so your homeserver can read it.
- **Maps for shared places** (Settings → Privacy → Maps, off by default): a
  place someone shares is drawn as a small map with a pin. The map's pictures
  come from OpenStreetMap's servers, which see your IP address and the area;
  nothing is fetched while it is off.

### Changed
- A message that is still sending while there is no connection is drawn in red,
  like one that failed, and turns normal once it is delivered.

### Fixed
- The red "No internet connection" bar now shows within seconds of the network
  being switched off, and goes within seconds of it coming back. It waits for
  the system to say the network changed (macOS and Linux; no polling), so it
  costs nothing while the connection is steady. Elsewhere the bar still comes
  when a sync fails.
- The release notes in the update dialog now scroll.
- "Waiting for this message. It will appear when the keys arrive." tiles that
  were really a call's own signalling (call reactions, ring notices) no longer
  show up in the chat, and the call's "started a call" line is no longer
  missed when it was read from the saved cache.

## v0.2.0-alpha.1

The big one is calls, as an early prototype.

### Added
- **Calls (early prototype).** The call button starts an Element Call
  (MatrixRTC) call in the room, using the call service your homeserver
  advertises; it needs a server with MatrixRTC (LiveKit) set up. The call is
  Element Call's own page: its buttons mute, turn the camera on, share the
  screen and hang up. Drag the thin grip under the page to make it bigger or
  smaller. In another chat, a bar at the top brings you back to the call, or
  leaves it. Very rough: only tried between two people on a Mac.
- Before your first call the app says the call page loads from Element's server
  (call.element.io) and what it learns; Settings → Calls takes your agreement
  back, and sets whether you join with the microphone off, the camera on, or ask
  first.
- In the chat: "started a call" and "The call ended", with the time and how long
  the call lasted. A desktop notification when somebody calls you. A room shows
  "Call in progress · N in call" with Join while a call goes on, and
  Room → ⋯ → "Clear my call from this room" removes a call you left badly.
- **Settings → About:** the version, updates (the startup check and "Check for
  updates now", moved here from Privacy and Advanced), what else the app uses,
  and the open source software it is built from, each with its full license.
  Releases carry the same list as THIRD_PARTY_NOTICES.md.
- **Settings → Advanced → Window → Minimize to tray** (off by default).
  Minimizing hides the window in a tray icon; a click on the icon, or on a
  notification, brings it back. On Linux it needs libayatana-appindicator3; the
  setting turns itself off, with a note, where the tray cannot be made.
- **Settings → Appearance → Status on my picture:** in the Rail layout, your
  status as a small cloud on your picture, with its emoji and words, always; a
  click edits it.
- **Settings → GIFs → Favorite GIFs:** import from Discord, remove the
  unavailable ones, reset all favorites. Favorites from Discord are copied to
  your device while their links still work (they expire after a day), and
  importing again no longer makes duplicates.
- A red bar across the top says "No internet connection" while the connection is
  down. A message that does not send says "Not sent" and why, in words, with a
  "Try again" button for text, files and stickers.
- A chat with yourself, "Notes to self", has a name and works.

### Fixed
- The "sent" check mark showed on a message that had not been delivered; it
  now only goes on one the server has.
- Linux: when no keychain (Secret Service) is running, signing in now starts one
  instead of failing: D-Bus is asked to launch it, then KDE Wallet (ksecretd,
  kwalletd6 or kwalletd5) or GNOME Keyring. If it still cannot, the error says
  what to turn on. Not tried on a real desktop yet.
- Discord GIF favorites showing "unavailable" the day after importing them.
- Older messages would not load in a chat that had call lines at its top.
- A call that stayed in the chat as "waiting for this message".
- Calls: the buttons at the bottom of the call page could be dead (a part of the
  chat list was laid over them). A fix is in; it needs confirming on a real call.

### Known rough edges
- Group calls, screen sharing, Linux, Windows and phones are untested for calls.
- Calls need your homeserver's call service; without one the call area says why.

## v0.1.0-alpha.5

### Added
- Settings → Appearance → "Jump to last chat on tab switch" (off by default):
  switching between Chats and a community reopens the chat you had there.
  Otherwise the pane says "Select a chat".
- Your status shows under your name in the Tabs and One inbox layouts, and one
  click edits it.
- The reaction bar on a message offers the emoji you used most recently.

### Fixed
- The Members panel listed everyone you had ever seen, in a shifting order;
  it now shows the room's members, sorted by name.
- The chat box takes the keyboard when you come back to a chat.
