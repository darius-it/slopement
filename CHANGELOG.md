# Changelog

What is new in each release, in plain words. The release workflow puts the
section named after the tag (or, failing that, "Unreleased") at the top of the
GitHub release notes. Add a line under **Unreleased** for every change a user
would notice; when you tag a release, rename that heading to the tag
(`## v0.1.0-alpha.6`) and start a fresh **Unreleased** above it.

## Unreleased

### Added
- **Calls (early prototype).** The call button starts an Element Call
  (MatrixRTC) call in the room, using the call service your homeserver
  advertises. Mic and camera can be toggled from the bar, and Leave ends your
  call for everyone else. Needs a server with MatrixRTC (LiveKit) set up. Very
  rough: audio, video and screen sharing have only been tried between two
  people.
- A "started a call" line in the chat when a call begins, "The call ended"
  when the last person leaves, and a desktop notification when somebody calls
  you.
- Call lines in the chat show the time, and "The call ended" says how long the
  call lasted.
- Before your first call the app says that the call page loads from Element's
  server (call.element.io) and what it learns; Settings → Calls takes your
  agreement back.
- **Settings → Calls:** start with the microphone off, the camera on, or ask
  first (the call page's lobby).
- "Call in progress · N in call" with a Join button while a call is going in a
  room.
- The call page resizes: drag the thin grip under it. While you are in a call
  and look at another chat, a bar at the top brings you back to it, or leaves.
- **Room → ⋯ → "Clear my call from this room"** removes a call you left badly,
  for when it still shows as going on.
- **Settings → GIFs → Favorite GIFs:** import from Discord, remove the
  unavailable ones, and reset all favorites to start over.
- **Notes to self:** a chat with yourself now has a name, and works.

- **Settings → Appearance → Your status:** in the Rail layout, show your status
  as a small thought cloud on your picture's corner (like the chat list's mini
  pictures) with its emoji and words, always; a click on it edits the status.

### Changed
- Favorite GIFs imported from Discord are copied to your device while their
  links still work (Discord's links expire after a day). Importing again no
  longer makes duplicates.

### Fixed
- Discord GIF favorites showing "unavailable" the day after importing them.
- A call that stayed in the chat as "waiting for this message".
- Older messages would not load in a chat that had "The call ended" lines at its
  top.

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
