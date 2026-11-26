# Changelog

What is new in each release, in plain words. The release workflow puts the
section named after the tag (or, failing that, "Unreleased") at the top of the
GitHub release notes. Add a line under **Unreleased** for every change a user
would notice; when you tag a release, rename that heading to the tag
(`## v0.2.0-alpha.1`) and start a fresh **Unreleased** above it.

## Unreleased

### Added
- **Phones: a new Communities overview.** Every community is a card on one page,
  with its busiest rooms (last message, time, unread count) right under its
  name: tap a room to open it, or "All rooms" to see the rest in place. The ⋯
  on a card has Invite and the community's options. No more small "All
  communities" back.
- **Phones: a bigger + menu.** The + beside the message field opens a sheet of
  large buttons: Photos, Camera, File and Poll.
- **Read receipts fill the check with who read it.** Your message shows a hollow
  check once sent; when someone reads it, the check turns into their picture (in
  groups, a few overlapping pictures and "+N"). Each person shows once, on your
  latest message they've seen. Someone who answered counts as having read, even
  with Read receipts off.
- **Faster reconnecting.** Coming back to the app (unlocking your phone,
  switching to it, waking the computer) reconnects at once instead of after the
  offline bar's retries, and a dropped connection is retried after 1, 2, 4 and 8
  seconds rather than every 10.
- **Phones: swipe a message to reply.** Drag a message toward the middle of the
  screen (to the right for messages on the left, to the left for your own): an
  arrow comes out beside it and ticks when it is far enough; let go there to
  reply, or sooner to let it spring back.
- **Phones: flick a picture away to close it.** In the picture viewer, drag the
  picture up or down (or flick it): it follows your finger, the background
  clears, and it closes if you take it far enough or flick it.
- **Phones: holding a message opens its options over the blurred chat,** as in
  Signal: the message stays sharp where it was and a card of options (Reply, Copy
  text, Pin, Forward, Delete…) extends down from it, on its side; it moves up with
  the message if it would not fit above the keyboard. The message sinks slightly
  under your finger as you hold it, with the system's tap of feedback.
- **Phones: counts on the tab bar.** Chats and Communities each show a solid
  count of what is waiting there (the accent color when it includes a mention), so
  you can see from any tab that something came in elsewhere. Muted rooms are left
  out.
- **Phones: a small arrow to jump to the latest message** appears at the bottom
  right of a chat when you scroll away from the newest messages, with a count of
  the messages that arrived meanwhile.
- **Phones: read circles.** As on desktop, the small avatar circles of who has
  read up to a message sit on the side of the bubble (in direct chats too), and
  your last message shows a hollow check circle until it is read (a clock while it
  is still sending). The check circle shows even with "Read receipts" off, since it
  only means the server has the message; the avatars need the setting on.
- **Phones: photos and the camera in the + menu.** "Photo or video" opens the
  system's library picker (several at once, no permission needed) and "Take a
  photo" the camera; what you choose waits in the message box like any file.

### Changed
- **AAC (.m4a) audio no longer plays in the app.** Voice messages are Ogg Opus
  (what Element sends) and still play; an AAC attachment now shows as a file you
  can save. (Apple Lossless .m4a still plays.)
- **Phones: the favorite star for GIFs moved to the picture viewer's top bar.** It
  no longer sits over every GIF in the chat; open a GIF and press the star beside
  Download. (On a computer it still shows when you point at a GIF.)
- **Phones: chats with one person no longer repeat their picture and name** over
  their messages: the header already says who it is.
- **Phones: "is typing" sits under the last message** instead of over it, and
  shows in group rooms too.
- **Phones: one layout.** Phones always show the Chats / Communities / People /
  Settings tabs; the Layout setting is now only on computers.
- **Phones: the GIF picker opens without the keyboard,** so there is room to
  browse; tap the search to type. The "Powered by KLIPY" line moved below the
  results instead of taking its own row. (On a computer the search still takes
  your typing at once.)

### Fixed
- **Phones: the picture viewer's top bar no longer stays over the chat** after
  flicking a picture away.
- **Phones: swiping back from a chat no longer jumps.** On iPhone, letting go of
  the edge swipe made the chat snap back to fill the screen and then slide out
  again; it now carries on from where your finger left it. Going back by a button
  or swipe takes a fixed 0.3 s and ends exactly there, and the slide in no longer
  keeps the screen busy for a second after it looks finished.
- **Phones: holding a message no longer selects a word.** The text in message
  bubbles is not selectable on a touch screen any more (it showed the system's
  Copy / Select All bar instead of the message's options); use "Copy text" in the
  message's options.
- **Phones: a flick that catches a flinging list now scrolls on.** Touching a
  list that was still gliding used to stop it and then ignore the finger until you
  lifted it; now it stops, and follows the finger as soon as it moves. (From a
  newer mygo, not yet released: pinned to its fix.)
- **Phones: going back feels native.** On iPhone, swipe from the left edge to
  leave a chat: it follows your finger, with the list moving beneath it. On
  Android, the back button and the back gesture leave the chat (and from the list
  they leave the app). Opening and closing a chat uses a soft spring, like the
  system's, that you can catch and turn around mid-way.
- **Phones:** the picture viewer's buttons no longer sit under the status bar.
- **Phones: scrolling up through history no longer stops dead.** Older messages
  are now asked for while the top is still a few screens away, not six messages.
- **Phones: the keyboard no longer stays up** after leaving a chat.
- **Phones:** no "GIF folders" in Settings or the GIF picker (an app on a phone
  can't read your folders).
- **Phones: finger-sized controls.** Buttons, text fields, chips and the clear
  button in search were drawn at desktop size on iPhone and Android; they are
  now 44 DIPs tall (chips 34), as the phone layout intended.
- **Phones: the strip under the tab bar and the home indicator** is now the same
  color as the bar (and as a room's wallpaper inside a room), not a separate band.
- **Phones: opening a room slides it in from the right** and Back slides it out;
  sheets no longer bounce as they open.
- **Sign-in:** the username and password fields tell iOS what they are (saved
  passwords are offered, no auto-capitalizing); the server field takes a URL
  keyboard. The message box asks for predictions instead of the Passwords key.
- **People on a phone:** "Add person" no longer runs off the screen; the
  "other servers and bridges" switch sits on its own line.
- **Phones:** the Settings page bar (and the chat filters, the search filters and
  wide code blocks) now scroll sideways with a finger. Bottom sheets and dialogs
  stay clear of the system bars (Android's back/home buttons, the iOS home
  indicator), and the pins and thread sheets fit short screens.

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
- **Voice messages (recording):** a microphone button in the message box on
  Macs and iPhones/Android phones (Linux and Windows have none yet). Press it,
  speak, and send; the bar shows the time and the level, and the bin throws it
  away. It goes out as a normal voice message with a waveform, which Element and
  other apps show as one. The microphone is only on while the bar is up, and is
  switched off if you leave the chat. Only tried with a fake microphone so far.
- **Send files to slopement from other apps:** "Open with slopement" on common
  pictures, videos, audio, PDFs, text and zip files (or drop one on the Dock
  icon) asks "Send to…" with your chats; on Android, slopement is in the share
  sheet for words, pictures and files. Nothing is sent until you pick a chat.
  Windows and Linux now run one copy of the app, so a file opened while it runs
  goes to that window. The Android share sheet is not tried on a device yet, and
  iPhone has none (see known gaps).
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
- **A file too large for your server** is now turned away when you drop, pick or
  paste it, with the server's own limit in the message ("big.zip (60 MB) is over
  your server's limit of 25 MB"), instead of uploading and then failing. The
  app asks the server for its limit after sign-in; a server (or proxy) that
  refuses with a bare "413" now also reads as "too large for the server".
- **A new device on every restart or switch between builds.** A development
  build and the installed app shared one saved login but kept its token in
  different places, so each found the other's device without a token, signed in
  again and overwrote the record: one more device in your session list each
  time. A development build now keeps its own folder (`slopement-dev`, signing
  in once there). Sessions already made stay on your account until you sign them
  out (Settings → Security → sessions, or your server's account page).
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
