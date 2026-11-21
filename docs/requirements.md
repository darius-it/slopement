# slopement requirements

A minimal, low-resource Matrix client in mygo's native `ui` (Go, GPU-drawn,
no webview) for macOS, Windows, Linux, iOS and Android. References: Element
(completeness), FluffyChat (simplicity, phone layout), Fluxer (desktop
rail and density).

Priority tags: **v1** (first usable release), **v1.1**, **later**. Items
without a tag are v1. Merged on 2026-11-21 with a friend's consolidated
feature list; its technical notes are folded into the sections below.

## Non-functional
- Native UI only. `ui.WebView` is used only for the call views (Jitsi,
  Element Call) and the optional inline YouTube player.
- Phone layout below 720 DIP (system stack and tab bar). Desktop layout
  otherwise: rail, room list, timeline, optional right panel. The right
  panel (members, thread, room info, search) **stays open across room
  switches** and shows the same panel for the new room.
- Idle means no frames. Under 150 MB RSS with 100 rooms, and it must still
  work with **1,000+ rooms** (the scope is decided on purpose, so heavy
  users aren't left out). `ui.List` for the timeline, rooms, members and
  results. Thumbnails only, with an LRU image cache. GIFs animate only while
  visible.
- **Timeline memory cap:** keep at most ~300 events per room in memory, plus
  the open rooms' windows. Everything else lives in SQLite and is paged back
  in when scrolled to.
- mautrix-go + goolm (pure Go, no cgo), with SQLite state and crypto stores.
  The access token goes in the OS keychain.
  - **Crypto is never hand-rolled.** SAS, QR, cross-signing, secret storage
    (SSSS) and key backup come from mautrix-go's `crypto` packages
    (`crypto/verificationhelper`, `crypto/ssss`, `crypto/backup`). We spend
    our effort on clear recovery and error messages instead.
  - matrix-rust-sdk (via FFI) was considered and rejected: it needs cgo and
    a Rust toolchain for every mobile target, which breaks the pure-Go
    build. Revisit this if goolm falls behind on a spec change.
- **Sync:** use Simplified Sliding Sync (MSC4186; native in Synapse since
  1.114) when the server advertises it, for a fast start with many rooms.
  Otherwise fall back to `/v3/sync` with a filter (`lazy_load_members`, a
  small `timeline.limit`, `unread_thread_notifications: true`).
  - Before building on it, check mautrix-go's sliding sync support. If it
    is missing, write a thin client for the `/simplified_msc3575/sync`
    endpoint behind the same `matrix/` interface.
- **Multi-account from day one** (UI in v1.1): each login gets its own
  directory, `<config>/slopement/accounts/<sha256(user_id)>/`, holding
  `state.db`, `crypto.db` and its media cache. Keychain entries are keyed
  by `user_id + device_id`. Every store and the sync loop take an account
  handle; nothing is global.
- **Session lock:** one process per account, enforced by an `flock`ed lock
  file in the account directory. A second launch hands its arguments
  (deep links, shared files) to the running instance and focuses it.
  Several windows inside one process share the session safely.
- **Crash-proof timeline:** each event renders inside a recover. A panic,
  an unknown type or malformed content shows a fallback tile ("Couldn't
  show this message · View source") instead of breaking the view.
- WCAG AA, screen-reader names, full keyboard navigation, labels on icon
  buttons, adjustable font size (Appearance, 85–150%), light/dark follows
  the system.
- **Text input:** IME composition (CJK, Korean, Vietnamese, dead keys)
  works in every field. The preedit is shown in place, and Enter while
  composing confirms the composition instead of sending. This needs a test
  per platform.
- i18n: strings go through one table with plural forms (CLDR), English
  first. Dates and times use the locale, and the layout mirrors for RTL.

## Account & login
- Homeserver first: a **matrix.org** chip, or **Other server** with
  `.well-known` discovery and live validation.
- Password, SSO/OIDC (system browser and a deep link back), and registration
  where offered. Logout, and a list of sessions with sign-out.
- **OIDC / Matrix Authentication Service is a first-class path, not a
  fallback.** matrix.org has used MAS since April 2025, so password login
  there is gone, and this is the likeliest thing to block users. The flow:
  1. Discovery: `.well-known/matrix/client` → the homeserver, then
     `GET /_matrix/client/v1/auth_metadata` (MSC2965). On older servers,
     use `/auth_issuer` and the issuer's OpenID configuration.
  2. Dynamic client registration (MSC2966) with our `client_uri`,
     `redirect_uris`, logo and policy links. Cache the `client_id` per
     issuer.
  3. Authorization code + PKCE (MSC2964), requesting the scopes
     `urn:matrix:client:api:*` and `urn:matrix:client:device:<device_id>`.
     The redirect is a loopback `http://127.0.0.1:<port>/callback` on
     desktops and a claimed scheme (`ai.slopement:/callback`) or universal
     link on phones.
  4. Access and refresh tokens are kept in the keychain, refreshed before
     expiry, and refreshed once on a 401 before giving up.
  5. "Manage account" (password, email, sessions) opens the server's
     `account_management_uri` (MSC4191) in the browser.
  - Login by QR from another signed-in device (MSC4108) is **later**.
- When a token is revoked (`M_UNKNOWN_TOKEN` with `soft_logout`), keep the
  stores and ask the user to sign in again. Never wipe the keys.

## Encryption & verification
- E2EE on by default in DMs.
- Verify after login: **another device (emoji SAS)** or **Security Key /
  Phrase**. Set up cross-signing and key backup. A reset flow.
- Verify other users from their profile. Trust shields on avatars and
  devices. An "unable to decrypt" state with a retry.
- **QR verification** too: show a QR code, or scan one on phones
  (`m.qr_code.show.v1` / `m.qr_code.scan.v1`, `m.reciprocate.v1`), with
  emoji SAS as the fallback when a camera is missing.
- **Sessions** in Settings → Security, grouped as **Current**, **Verified**,
  **Unverified** and **Inactive** (not seen for 90+ days). Each shows its
  name (renamable), last seen time and IP, and the client. Sign out one, or
  every inactive session at once (with user-interactive auth or the OIDC
  account page).
- **Recovery that forgives mistakes:**
  - Specific errors instead of "something went wrong": wrong key or phrase,
    backup created by another key, no backup on the server, server
    unreachable, and a "your other device must be online" hint for SAS.
  - A visible **"Key storage out of sync"** state when SSSS secrets are
    missing or the cached backup key doesn't match the current backup
    version. A **Fix** button re-fetches the secrets with the recovery key,
    or creates a new backup version.
  - Reset identity ("I lost everything") as a separate, clearly worded,
    two-step flow.
- **Export / import room keys**, always available (the standard Megolm
  export file, encrypted with a passphrase), independent of backup.
- **Later:** sharing older history with newly invited members in encrypted
  rooms (MSC3061 `shared_history` key bundles / MSC4268).

## Navigation: Chats and Communities (decided 2026-11-21)
People are not "inside" spaces. The app has two places:
- **Chats:** everyone you talk to directly, meaning DMs (`m.direct`) and
  small group chats (no parent space, up to ~10 members), in one list by
  recency, whatever space Matrix keeps them in. **People** (statuses at a
  glance) sits at the top of it.
- **Communities:** spaces, called "communities" with "rooms" in the UI
  (never "spaces"). Each opens to a home (avatar, name, about, N online / N
  members, Invite) and its rooms. Large rooms in no space form an "Other
  rooms" community.

**Appearance → Layout** has three options. The default is the platform's,
until the user picks one:
| Layout | Desktop | Phone |
|---|---|---|
| **Rail** (desktop default) | Chats first on the rail, then the communities; each shows only its own rooms | shown as Tabs (the rail needs room) |
| **Tabs** (phone default) | a Chats / Communities switch over the list | tab bar: Chats / Communities / People / Settings |
| **One inbox** | one list with filters (All · People · Communities · Unread); communities unfold to their rooms in place | the same, with Inbox / People / Calls / Settings |

- Also in the menu bar under View → Layout, with the current one checked.
- Stored in account data (`com.slopement.layout`) so it follows the user
  across devices of the same kind (desktop vs phone, kept separately).

## Spaces & rooms (details)
- The rail's **+** (Create a space / Join a space / Explore public spaces)
  sits right under the last space, as a dashed tile. The user's avatar, at
  the rail's foot, opens their status.
- Space rail (a row on phones): Home plus spaces, nested spaces, drag to
  reorder.
- Room list sections: Favourites, People, Rooms, Low priority. Filter chips
  All / Unread / Favourites / People. Rows show avatar, name, preview, time,
  badge (mention = accent) and mute.
- Space home, room directory, join by alias, invites, and create
  room / DM / space.
- **Knocking:** "Ask to join" on rooms with `join_rule: knock` or
  `knock_restricted` (`POST /knock`, with an optional reason). Admins see
  pending requests in the room header and accept or decline them.
- **Sorting:** by recent activity by default (A–Z as an option). Only real
  messages bump a room: reactions, edits, receipts and most state events
  don't. With sliding sync this uses the server's `bump_stamp`; otherwise
  we compute it locally.
- **Tags are standard and shared with other clients:** `m.favourite` and
  `m.lowpriority` in `m.tag` (with `order`), and DMs in `m.direct`, so the
  state matches Element.
- **Saved filters / folders (v1.1):** besides All / Unread / Favourites /
  People, the user saves their own filters (unread, DMs only, per community,
  "mentions only"), stored in `com.slopement.filters`. The Unread filter is
  in v1.
- **Nested communities:** a breadcrumb ("Matrix HQ › Dev › Go") with
  working back navigation at every depth, including Escape and the system
  back gesture. A sub-space opens in place, and Back returns to the scroll
  position you left.

## Room actions
- **Mute: 15 minutes, 2 hours, 1 day, 1 week, until turned back on.**
  Implemented as a push-rule override plus an expiry; the expiry is mirrored
  in account data. "Muted until 14:30" is shown.
  - Matrix push rules have **no expiry**. A mute is an `override` rule with
    the room's ID as `rule_id`, an `event_match` on `room_id` and
    `actions: []`, so it works in every client and on every device's push.
  - The expiry lives in account data, `com.slopement.mute_until` →
    `{room_id: unix_ms}`. Any of our clients deletes the rule when the time
    is up: at that moment if running, otherwise at the next start or sync.
    Other clients **won't** unmute it automatically. The mute menu says so
    in small print ("Other apps will show it muted until Slopement runs").
  - "Until I turn it back on" has no expiry entry. An existing rule that we
    didn't create is shown as muted forever.
- **Favourite** (`m.favourite`), low priority, mark read/unread, notification
  level, copy link, leave.
- Available from right-click / long-press, the ⋯ button, the header menu and
  room info.

## Timeline & composer
- Markdown/HTML (see "Message formatting"), media and link embeds (see
  "Media & embeds"), replies, threads, edits, redactions, reactions, receipts,
  typing, date separators, grouping, jump to unread/bottom, back-pagination.
- Images, video, files, voice messages, stickers. Upload progress. Paste and
  drop.
- Composer: Enter sends (Shift+Enter breaks the line), @/# autocomplete,
  :shortcode:, attach, emoji, GIF, reply/edit banner, per-room drafts.
- Link previews and pins. Polls are v1.1.
- **Hover action bar** on desktop messages: react, reply, thread, edit
  (own messages), and ⋯ (copy text, copy link, pin, mark unread, remind me,
  view source, delete, report). On phones a long press opens the same
  actions as a sheet.
- **Pinned messages** (`m.room.pinned_events`): a bar under the header
  showing the latest pin, which cycles through the pins when clicked, plus
  a pins list in room info.
- **Copy message link:** a `https://matrix.to/#/<room>/<event>?via=…`
  permalink, with up to 3 `via` servers picked as the spec suggests.
- **Mark as unread** (`m.marked_unread` room account data, Matrix v1.12)
  and **Remind me** about this message (in 1 h / tomorrow / next week /
  pick a time). Reminders are kept in `com.slopement.reminders` so they
  sync across devices, and fire as a local notification. **v1.1.**
- **Message selection:** select text across messages with the mouse.
  ⌘/Ctrl+C copies it as plain text, with "Sender: " prefixes when the
  selection spans senders.
- **Albums:** several images or videos picked together are sent as one
  album. Use MSC4274 (`dm.filament.gallery`) when the room's other clients
  can read it; for now, send separate events and group consecutive media
  from one sender within 5 s into one grid client-side.
- **Open attachments directly:** a file opens in the OS's default app
  (decrypted to a private temp file that is deleted when the app quits),
  and "Save as…" stays available. Images and videos open in the viewer.

## System events (timeline)
- Shown as quiet one-line rows (muted, small, with an icon), never as
  bubbles: joins, leaves, invites, kicks and bans (with reasons), display
  name and avatar changes, and room name, topic, avatar, encryption, join
  rule, history visibility and power level changes. Also calls started or
  ended, and pinned or unpinned messages.
- Consecutive membership changes collapse: "Mira, Sam and 3 others joined"
  expands on click. Avatar changes show the old and new picture.
- Settings → Timeline lets the user hide joins and leaves, and hide avatar
  and name changes. The defaults are shown in DMs and small groups, and
  collapsed in large rooms.
- Redacted events show "Message deleted" (with by whom when it wasn't the
  sender), and stay in place so threads and replies keep their anchors.

## Unread, read state & notification badges
Correctness here is a release blocker: badges must never come back after a
restart or after reading on another device.
- Counts come from the server (`unread_notifications`, and
  `unread_thread_notifications` per thread), not from local guesses. Main
  timeline and threads are counted **separately** (threaded receipts,
  MSC3771: receipts carry `thread_id`, `"main"` or the thread root).
- Reading sends `m.read` (or `m.read.private` when the "Send read receipts"
  setting is off) plus `m.fully_read` through `/read_markers`. Both only
  move forward.
- **Mark all as read** (per community and globally) sends a receipt for
  each room's latest event in batches and persists. It does not just clear
  the badges locally.
- **OS notifications are cleared when read elsewhere:** when a sync brings
  our own receipt past a notified event, that notification is withdrawn
  (desktop: by tag per room/event; iOS: `removeDeliveredNotifications`;
  Android: `cancel(tag)`). A push with `unread_count: 0` does the same while
  the app is in the background.
- An unread dot (no number) for rooms set to "mentions only" that have new
  messages. A number for notifying messages, and the accent for mentions
  and keywords.

## Keyboard shortcuts (desktop)
⌘ on macOS, Ctrl elsewhere. Listed in Help → Keyboard shortcuts (⌘/), and
shown in the menus.
| Keys | Action |
|---|---|
| ⌘K | quick switcher |
| ⌘F | search in room; ⌘⇧F everywhere |
| Alt+↑ / Alt+↓ | previous / next room; with Shift: next **unread** room |
| ↑ in an empty composer | edit your last message |
| Esc | close the panel, dialog, reply or edit; in a room, jump to the bottom and mark it read |
| Shift+Esc | mark the room as read |
| ⌘E / ⌘D | toggle mic / camera in a call |
| ⌘B / ⌘I / ⌘U / ⌘⇧X | bold / italic / underline / strike |
| ⌘D (outside calls) | favourite the room |
| ⌘, | settings |
| PageUp / PageDown | scroll the timeline |
| ⌘C | copy the selection |

## Message formatting (Discord-flavoured Markdown)
Typed in the composer, sent as `body` (plain text) plus `formatted_body`
(`org.matrix.custom.html`). Received HTML is rendered from the same subset,
so Element and FluffyChat messages look right too.

| Syntax | Result | Sent as |
|---|---|---|
| `# ` / `## ` / `### ` | headings (start of line) | `<h1>`–`<h3>` |
| `*italic*` / `_italic_` | italic | `<em>` |
| `**bold**` | bold | `<strong>` |
| `***bold italic***` | bold italic | `<strong><em>` |
| `__underline__` | underline (**Discord semantics, not CommonMark bold**) | `<u>` |
| `~~strike~~` | strikethrough | `<del>` |
| `\|\|spoiler\|\|` | spoiler: hidden until clicked, per message | `<span data-mx-spoiler>` |
| `\*` etc. | backslash escapes any marker | (literal) |
| `[text](https://…)` | link; bare URLs autolink | `<a href>` |
| `> quote` | quote block (multi-line) | `<blockquote>` |
| `` `code` `` | inline monospace | `<code>` |
| ```` ```go ```` … ```` ``` ```` | code block with language label, highlighting, copy button | `<pre><code class="language-go">` |
| `- item` / `* item`, `1. item` | bullet / numbered lists, nested by indent | `<ul>` / `<ol>` |

- Mentions (`@user`, `#room`) become pills, plus `m.mentions`.
- Composer: live syntax hints stay plain text (no WYSIWYG). Formatting
  shortcuts ⌘B / ⌘I / ⌘U / ⌘⇧X on the selection.
- Sanitizing: an allowlist of tags and attributes; links show their real
  host on hover; no remote images in HTML (only `mxc://`).
- Edits re-render; "(edited)" is shown.
- **Compatibility of `__underline__`:** CommonMark reads `__text__` as bold,
  but we send `<u>`. Other clients render the HTML correctly, but the plain
  `body` keeps the raw `__text__`. Typing `**` for bold works everywhere.
- **Emoji shortcodes:** `:joy:` and `:arrow_up:` are replaced on send even
  without the autocomplete. The autocomplete opens after `:` plus 2
  characters, ranks recent emoji first and supports skin tones
  (`:thumbsup::skin-tone-3:`). The data comes from emojibase/CLDR
  shortcodes plus the GitHub/Slack aliases, embedded at build time. Custom
  pack emoji (`:blobcat:`) come from the same autocomplete.
- **LaTeX (opt-in, later):** off by default, because a lone `$` appears
  constantly in normal chat (prices).
  - Delimiters: `$$…$$` for blocks; `$…$` inline only when the opening `$`
    isn't followed by a space, the closing `$` isn't preceded by one or
    followed by a digit (pandoc's rule), and it's on one line.
  - Sent per the Matrix convention, `<span data-mx-maths="…">` /
    `<div data-mx-maths="…">`, with the TeX source inside as `<code>` as the
    fallback.
  - Received maths always renders (or shows its fallback).
  - Rendering: no native TeX engine exists for mygo. Options are a small
    pure-Go TeX-subset layout, or rasterizing in the call web view.
    Evaluate before committing; until then, show the source in a code chip.

## Media & embeds
- **Attachments:** images, video and GIFs show inline previews:
  - Thumbnails at display size with a blurhash placeholder until loaded.
    Aspect ratio comes from `info.w/h`, so the timeline never jumps.
  - Max 360×280 (phones: full width). Rounded 10.
  - Video: a poster frame, a duration pill and a play button. It plays
    inline (`ui.Video`) muted, then with sound on tap.
  - GIFs autoplay only while visible and the window is focused, honour
    Reduce Motion (still frame plus a GIF badge), and pause off-screen.
  - Several images in one batch become a 2×2 grid ("+3" on the last).
  - Click opens a full-screen viewer: zoom/pan, swipe between the room's
    media, download and share.
  - Spoilered media is blurred behind "Spoiler" until clicked.
  - Files show a card with an icon, name, size and download.
  - Upload: paste, drag-drop, or the picker. A preview before sending with a
    caption, "send as file" and a spoiler toggle. A progress ring on the
    pending message.
- **Link preview embeds** for the first 1–3 URLs of a message:
  - Generic sites: a card with the site name, title, description and
    thumbnail, from the server's `/_matrix/client/v1/media/preview_url`
    (OpenGraph). An accent edge in the site's colour when known, else
    neutral.
  - **YouTube** (youtube.com, youtu.be, shorts): a large thumbnail, title and
    channel, a play overlay and a duration. Clicking plays in an inline web
    view (youtube-nocookie embed) or opens externally (a setting).
  - **Imgur** (i.imgur.com, imgur.com/a, gifv): shown as an image/GIF/video
    preview, not a card; albums show the first item plus a count.
  - Direct media URLs (.png/.jpg/.gif/.webp/.mp4) preview like attachments.
  - The sender can remove the embed (`m.url_previews` suppression); readers
    can collapse it.
  - **Privacy:** previews are **off by default in encrypted rooms** (fetching
    leaks the URL to the server), with a per-room and global toggle. They are
    on in unencrypted rooms. Results are cached per URL.
  - Where previews come from, as a setting with these options:
    - **Off.**
    - **Via my homeserver** (the default for unencrypted rooms). The server
      can't see inside encrypted rooms, so in those it has to be sent the
      URL.
    - **Directly from my device.** This exposes the user's IP to every
      linked site, so the setting warns about it.
  - **Sender-bundled previews:** when sending, our client fetches the
    preview and puts it inside the (encrypted) event (MSC4095,
    `com.beeper.linkpreviews`, with an `mxc://` thumbnail). Readers then
    need no fetch at all, which makes this the privacy-preserving path in
    E2EE rooms. We read those from other clients too.
  - YouTube and Imgur titles come from **oEmbed**
    (`youtube.com/oembed`, `api.imgur.com/oembed`). Thumbnails
    (`i.ytimg.com`) also leak the IP, so in "Via my homeserver" mode they
    go through the homeserver's `preview_url`, or an optional oEmbed proxy
    if one is configured.
  - **SVG files** never render inline (they can embed scripts and remote
    references). They show as a file card with a sanitized, rasterized
    thumbnail, or none.

## Polls
- Created from the composer's `+` → Create poll. A dialog on desktop, a
  sheet on phones (after WhatsApp, Discord and LinkedIn).
- A question, then 2–20 answers: each removable, Enter in the last adds
  one.
- Options: allow multiple answers; hide results until the poll ends
  (undisclosed); runs for 1 hour / 1 day / 3 days / 1 week / no end.
- Matrix: `m.poll.start` (MSC3381, stable in v1.7+), votes as
  `m.poll.response`, ending as `m.poll.end`. Matrix polls have no end time,
  so the creator's client sends `m.poll.end` when the chosen duration is
  up (the end time is stored in the event's content).
- In the timeline: a card with the question and "Select one / Select one or
  more". Each answer has a radio or check box. After voting (or once ended,
  for hidden polls), a bar of each answer's share fills in behind it, with
  the leader's in the accent. The footer shows "N votes · 3h left", plus End
  poll for the creator.
- **Who voted for what:** once results show, "N votes" in the footer is a
  link: brighter and underlined on hover, and it opens a popover (a sheet
  on phones) listing each answer with its count, then its voters (you
  first) with avatars and names in their colours, and "and N more" for
  votes whose senders aren't loaded yet. The voters come from the
  `m.poll.response` relations; Matrix votes are visible to every member
  anyway. While a hidden (undisclosed) poll runs, the count stays plain
  text with a tooltip, because listing voters would give its results away.
- **Interoperable, not a custom modal.** Polls use only the standard
  events, so Element and others show and vote on them:
  - `kind`: `m.poll.disclosed` / `m.poll.undisclosed`.
  - `max_selections` for multiple answers.
  - A text fallback in the event's `m.text` for clients without polls.
  - We read the unstable `org.matrix.msc3381.poll.*` names as well.
  - Only each voter's latest `m.poll.response` before `m.poll.end` counts,
    and votes are tallied from the event relations (`/relations`).

## Custom status
- Set it from your avatar at the foot of the rail, or in settings/profile.
  An emoji (picker) and text up to 80 characters, in the user's own words.
  **No suggested statuses.**
- Clear after: 30 minutes, 1 hour, 4 hours, Today (midnight), This week
  (Monday 00:00), or Don't clear. Expired statuses disappear on every
  client.
- It shows:
  - under the user's name in their DM's header (desktop and phone);
  - as a small thought bubble on the avatar in the chat list (the emoji;
    hovering the bubble opens it to the whole status, and the row's
    preview stays the last message);
  - under the name in the member list;
  - in a soft bubble in the profile card that a user's avatar or name opens.
- Matrix: the user's profile field (MSC4133 extended profiles,
  `org.slopement.status` → `{emoji, text, until}`) where the server supports
  it. Otherwise presence's `status_msg` (`"🎧 Focusing"`), with the expiry
  in account data and cleared by our client when it's up. Servers with
  presence off (matrix.org) show no status to clients that don't read the
  profile field.
- **The expiry is private to its owner in the UI:** others see only the
  emoji and text, never "until …" (it's in the profile field only so their
  clients can hide it once it has passed).
- **Expiry is enforced by clients, not the server.**
  - When reading, a status whose `until` has passed is hidden, so a stale
    one never shows even if its owner's client never came back to clear it.
  - The owner's client clears it at the expiry time, or at the next start.
- **Feature detection:** check `/versions` `unstable_features` for
  `uk.tcpip.msc4133`, and the `m.profile_fields` capability (is the field
  allowed?), before writing the profile field. Presence is unreliable on
  many servers, so it's never the only source.
- **Appear offline** (`set_presence=offline` on sync, plus no typing
  notifications or public read receipts if the user chooses) and an
  **away message** (a status with an auto-reply flavour shown in the DM
  composer to the other person: "Mira is away: 🌴 In Lisbon"; the text is
  the owner's, so they write a return date in it only if they want to).
  **v1.1.**
- **Last seen** is cached locally per user (`last_active_ago` turned into
  an absolute time), so "Last seen 2h ago" survives restarts and servers
  that send presence rarely.

## People: statuses at a glance
- A **People** view of everyone the user has a DM with: on desktop an entry
  at the top of the room list (with "N online"), on phones its own tab
  (Chats / People / Calls / Settings). After Discord's Friends, Riot's
  Social and Campsite's people panel.
- Search, plus All / Online. Sections "Online — N" and "Offline — N";
  within each, people with a status first, then by name.
- Each row: avatar with a presence dot, name, and under it their status
  ("🌴 In Lisbon"), or Away, Online, or "Last seen 2h ago".
  A message button (on hover, always on touch) opens the DM; the row opens
  the profile card. (Others' expiry times are never shown.)
- An "Add person" action (invite by MXID).
- On desktop, People is a grid of tiles; statuses float over the avatars as
  comic thought bubbles (three trailing dots) that open to the whole status
  on hover. Phones keep the list.
- People is **optional**: statuses also show in Chats in every layout (a
  small thought bubble on the avatar that opens on hover; under the name in
  a DM header).

## Favourites
- Starred chats (`m.favourite`) lead the Chats list under "Favourites" in
  every layout (Rail, Tabs, One inbox), toggled from the chat's menu.

## Profile card
- Opens from a user's avatar or name anywhere: timeline names and avatars,
  names over bubbles, People (a popover by the name; a dialog from People;
  a sheet on phones). It shows their picture, name in their colour, MXID, pronouns,
  status, and trust (an unverified user gets a "Verify" nudge), plus Message
  and Call.

## Custom username colours
- 8 AA-safe hues, hashed from the MXID by default. Users set their own (in
  account data) and can override others' colours locally
  (`com.slopement.user_colors`).
- **Per-viewer first:** the core feature is choosing how *others* look to
  *you* (private account data, synced across your devices).
- Publishing your own colour is optional, as a profile field
  (`org.slopement.color`, MSC4133). Only clients that read it will show it,
  and the viewer's own override wins.

## Nicknames, notes & pet names (private)
- **Nicknames** for people: shown instead of their display name everywhere
  (timeline, lists, mentions you type), with the real name in the profile
  card and on hover ("Mum (Barbara Klein)").
- **Notes** on a person: free text, shown in their profile card, editable
  there.
- **Pet names** for rooms and DMs: a local name that overrides
  `m.room.name` for you only.
- All of these are private account data (`com.slopement.user_notes` →
  `{mxid: {nickname, note}}`, `com.slopement.room_names` →
  `{room_id: name}`), synced across your devices and never visible to
  others. Search and the ⌘K switcher match both the real and the private
  names.

## Search with filters
- Message search via the server for unencrypted rooms and a local SQLite FTS
  index for encrypted ones.
- Filters as tokens: in:, from:, has: (image/file/link/GIF), before/after,
  mentions me. Results are grouped by room; opening one jumps to the event
  and flashes it.

## Quick switcher (⌘K / Ctrl+K), a priority feature
- Fuzzy over rooms, DMs and spaces. Ranking: mentions, then unread, then
  recency, then match. ↑↓ / ↵ / Tab scope / Esc. Shows recent rooms when the
  query is empty. "Search messages for …" is the last row. No animation.

## Profile pictures, a priority feature
- Avatars everywhere (authenticated media thumbnails at 2×, cached).
  Initials on the user's colour as the fallback.
- **Animated avatars** (GIF/WebP) are still by default and animate only on
  hover (desktop) or in the profile card. A setting can turn animation on
  everywhere or off entirely.
- Set, change or remove your own picture (crop to a circle). Room and space
  avatars where permitted. Per-room member avatars. A profile sheet.

## Picker: GIFs, stickers, emoji (KLIPY)
- A popover on desktop (opened from the GIF or the smiley button, on that
  tab), a sheet on phones (one smiley button, opening where it was left).
  Tabs: **GIFs / Stickers / Emoji**. Clips and memes are dropped.
- **GIFs:** the front page is Discord's: a **★ Favorites** tile, **Trending
  GIFs**, then KLIPY's categories, all as animated tiles. A ★ on each GIF
  (on hover; always on touch) adds it to the favorites. Typing searches
  (debounced), and more pages load as the end scrolls into view.
- **Import from Discord** (in Favorites): a JSON from
  [discord-gif-exporter](https://github.com/darius-it/discord-gif-exporter)
  (`originalURL → {format, src, width, height, order}`), via the system
  file dialog. Tenor/Giphy MP4s map to their GIF renditions; videos with no
  GIF are skipped; duplicates skipped; a toast reports the count.
- A message that is just a GIF/media link shows as the media, not the link.
- **Stickers:** recently used, then trending; search.
- **Emoji:** **recently used** first, then the categories; search by name.
  The footer shows the hovered emoji's name.
- Provider **KLIPY**, native API (`/api/v1/{key}/{gifs|stickers}/…`,
  verified live), with the key in `.env` (`KLIPY_API_KEY`), ads filtered,
  and an anonymous per-install `customer_id`. Tiles animate the `xs` GIFs
  (~90 px), at most 96 kept, and pause off-screen. Favorites and recents
  persist in the user config dir (`slopement/picker.json`).
- Sent items are uploaded to the media repo (never hotlinked). KLIPY's
  register-share is called on send. "Powered by KLIPY" attribution is shown.
- **Local folders as a source (v1.1):** the user adds folders (Settings →
  GIFs) and their `.gif/.webp/.mp4/.png` files appear as a "My folder" row
  and in search (by file name). They are indexed in the background,
  thumbnailed once and watched for changes.
- **Custom emoji & sticker packs (v1.1):** MSC2545 image packs, as used by
  Cinny, FluffyChat and Nheko. These are personal packs in
  `im.ponies.user_emotes`, room/community packs in `im.ponies.room_emotes`
  state, and enabled room packs in `im.ponies.emote_rooms`.
  - They show as extra categories in Emoji and Stickers, and are sent as
    `<img data-mx-emoticon src="mxc://…" alt=":name:" height="32">`, with
    stickers as `m.sticker`.
  - "Add to my pack" works on any custom emoji or sticker you see. Pack
    editing UI is **later**.

## Calls (MatrixRTC first, Jitsi as the fallback)
Full research and design: [calls-research.md](calls-research.md)
(2026-11-21).

**Decided 2026-11-21, the v1 scope:** **MatrixRTC (Element Call) is the
main call system; Jitsi is the fallback** when MatrixRTC isn't available
or doesn't work. Both are in v1. The rules below are the v1 behaviour,
except where marked v1.1.
- One Call button. Calls start with the camera off and the mic on; the
  camera toggles.
- **Two providers behind one `calls` interface,** because Jitsi and Element
  Call (MatrixRTC, MSC4143 + LiveKit MSC4195) **don't interoperate**:
  - Jitsi has been the more reliable experience for us.
  - Element Call is where the ecosystem is going (Element X, FluffyChat,
    Cinny), and it is the only one with real Matrix E2EE.
- **Which one:**
  - **Join what's already in the room,** never splitting a call: a running
    MatrixRTC call or a running Jitsi widget call.
  - **New calls use MatrixRTC** when the homeserver advertises a MatrixRTC
    transport (LiveKit SFU + authorization service).
  - **Fall back to Jitsi** when it doesn't, so homeservers without LiveKit
    still get calls.
- **Fallback when MatrixRTC fails, before anyone else has joined:**
  - Failures that count: the JWT service refuses, the SFU is unreachable,
    ICE fails, or there's no media within ~8 s.
  - Then we offer **"MatrixRTC isn't working here. Use Jitsi instead?"**
    with the reason, and start a Jitsi call on a click.
  - Errors in the transport discovery itself go straight to Jitsi, with a
    short note.
- Mid-call there's no silent switch: **"Move this call to Jitsi"** (in the
  call menu) starts a Jitsi call and posts a notice in the room
  ("Mira moved the call to Jitsi · Join") so everyone follows deliberately.
- **Implementation (v1):** Element Call's **embedded package** in a web
  view, with us as the **widget API host**. That means:
  - capabilities;
  - call membership state;
  - to-device encryption keys;
  - delayed events (MSC4140);
  - join and hangup.

  Jitsi runs through its iframe API.
- **v1.1:** a **Settings → Calls → Call system** setting (MatrixRTC with
  Jitsi fallback (default) / Always Jitsi) and a per-room override
  (`com.slopement.call_provider`), for rooms where Jitsi works better;
  automatic fallback without asking, if chosen.
- **Later:** native MatrixRTC through LiveKit's Go SDK, with no web view.
- Jitsi: uses the room's widget, or creates one from `.well-known`'s Jitsi
  domain (falling back to meet.element.io).
- Native control bar over the provider's web view (its own UI hidden, the
  prejoin page skipped). A "call in progress · Join" banner.
  Picture-in-picture when navigating away.
- Camera and microphone permissions on phones. WebRTC in WebKitGTK must be
  verified early (Linux).
- Local, private call stats (round-trip time, packet loss, jitter, time to
  first media) in Settings → Calls → Last call, to compare the two
  providers honestly.
- Buttons: **Call** (mic on, camera off, camera toggle available) and an
  explicit **Audio only** in the call menu (`startAudioOnly`: no camera
  control at all, lower bandwidth). There are no separate voice and video
  modes.
- Jitsi config passed via the iframe API: `startWithVideoMuted: true`,
  `startWithAudioMuted: false`, `prejoinConfig.enabled: false`,
  `disableDeepLinking: true`, `toolbarButtons: []`, plus the user's display
  name and avatar URL. We don't send the Matrix ID to the Jitsi server.

## Notifications & settings
- Desktop notifications, plus push via mygo `push` and a gateway (v1.1),
  respecting mutes.
- Settings: account/profile, appearance, notifications, security, media,
  about.
- **Push channels (v1.1):**
  - **iOS:** APNs through a Sygnal gateway, with a Notification Service
    Extension that decrypts E2EE payloads (`event_id_only` pushes, then a
    fetch and decrypt in the extension).
  - **Android:** **UnifiedPush** first, for notifications without Google
    services. The user picks a distributor (ntfy, NextPush…), and the
    pusher's URL goes to the server's UnifiedPush Matrix gateway
    (`/_matrix/push/v1/notify`). FCM is a fallback when Google services
    are present.
  - **Desktop:** notifications come from the running sync (no push
    server). If the app keeps running in the tray or menu bar after its
    window closes, that's an option.
- **Rules** (`m.push_rules`, shared with other clients):
  - **Per-room levels:** All messages / Mentions & keywords / Off, plus
    timed mute.
  - **Keywords:** `content` rules, with highlight.
  - **@room:** on or off.
  - **Mentions:** `m.mentions`-aware (intentional mentions, MSC3952), so
    quoting a name in a reply doesn't ping.
  - **Calls and invites** each get their own switch.
- **Threads:** notifications from threads are opt-in per thread ("Notify
  me about replies"). By default you're notified only for threads you
  started or posted in, and for mentions. Thread subscriptions use MSC4306
  where the server supports it, else a local list in account data.
  **v1.1.**
- Settings → Privacy: send read receipts, send typing notifications, link
  preview source (see above), and appear offline.

## Moderation (basic)
- Kick, ban and unban (with a reason) from the member list or a profile
  card, when the user's power level allows. The power level editor is
  **later**.
- **Report** a message (`POST /rooms/{room}/report/{event}`, reason and
  severity) or a room (`/rooms/{room}/report`, Matrix v1.13).
- **Bulk delete:** "Remove recent messages" from a user (redact their last
  N events in this room, with a preview of the count), rate-limited so it
  doesn't trip the server's limits.
- Ignore user (`m.ignored_user_list`) from the profile card. It hides their
  messages and invites.

## Compatibility with other clients (a release blocker)
- **Render** what others send, correctly:
  - Replies: `m.in_reply_to`, stripping the legacy `<mx-reply>` fallback
    and `> ` quotes.
  - Edits: `m.replace` with `m.new_content`, only from the original
    sender.
  - Threads: `m.thread`, including the reply fallback.
  - Voice messages: `org.matrix.msc3245.voice`, with an
    `org.matrix.msc1767.audio` waveform.
  - Polls (stable and unstable) and stickers.
  - `m.emote` and `m.notice`.
  - Location (`geo:`): a static card with an "Open in maps" link.
  - The full sanitized HTML subset, and SVG and other files as cards.
- **Fixture suite:** `testdata/events/` holds real, anonymized events
  captured from Element Web/X, FluffyChat, Cinny, Nheko, NeoChat and the
  common bridges (WhatsApp, Telegram, Discord, Signal). Each fixture is
  rendered in a golden test (text dump plus snapshot) on every release, and
  every rendering bug found adds a fixture.
- **Account data shared with others, read and written the same way:**
  `m.direct`, `m.tag`, `m.ignored_user_list`, `m.push_rules`,
  `m.marked_unread`, `m.secret_storage.*`, `m.cross_signing.*`,
  `m.megolm_backup.v1` and `im.ponies.*`. We also read Element's
  breadcrumbs (`im.vector.setting.breadcrumbs`) for "recent rooms".
  Switching clients loses nothing; only our own extras use
  `com.slopement.*`.

## Diagnostics & recovery
- **Export logs** with one click (Help → Export logs): a zip of the
  rotating logs (7 days, 20 MB cap) and the app and server versions.
  - Tokens, keys, message bodies and room names are never logged. IDs are
    hashed unless the user ticks "include room IDs".
- **Reset local cache** (Settings → Advanced): drops the state store, sync
  token and media cache, then does a fresh initial sync. It **never
  touches** `crypto.db`, the keychain or the recovery key, and it says so
  in the confirmation.
- **View source** on any event (decrypted and original JSON) for bug
  reports.

## Mobile only
- **Share to Slopement** (v1.1): an iOS Share Extension and an Android
  `ACTION_SEND` / `ACTION_SEND_MULTIPLE` intent, for images, video, files
  and text.
  - They pick a chat with the ⌘K-style switcher, then show the upload
    preview.
  - The extension hands the files to the app through the App Group or the
    intent, so the session is never opened twice (see the session lock).
  - First check mygo's support for share extensions and intents.

## Project practices
- **Scope on purpose:** test with a 1,000-room account and a 50,000-member
  room before each release, so heavy users aren't left out.
- **Known gaps, in public:** `docs/known-gaps.md` lists what isn't
  supported yet (Element Call, LaTeX rendering, pack editing…), with the
  status of each request and links to issues.
- **Compatibility is a release blocker:** a release is held if the fixture
  suite or the interop checklist (verify with Element, vote in an Element
  poll, join an Element X / Element Web MatrixRTC call, join an Element Web
  Jitsi call, read Element threads) fails.
