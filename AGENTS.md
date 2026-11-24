# AGENTS.md — slopement

Minimal, low-resource Matrix client (Element-class features, FluffyChat simplicity,
Fluxer-style space rail) for desktop and phones, built with **mygo's native `ui`
package** (Go, GPU-drawn, no webview). Full requirements + design language:
`docs/requirements.md` and `docs/design-language.md`.

## Working rules

### Sandbox / environment
- The sandbox blocks shell file operations such as `mkdir`, `cp`, `mv`, `rm`.
  **Create and edit files with the Write/Edit tools** (Write creates parent
  directories). Don't use shell commands to manipulate files.
- Bitdefender's ransomware protection ("Safe Files") partially blocks zsh as
  a process that writes files: `sed -i`, `>`/`>>` redirects and heredocs into
  project files fail with "operation not permitted". **Edit files only with
  the Edit/Write tools.** Use the shell only for read-only commands and the Go
  toolchain (`go vet`, `go build`, `go test`, `go mod tidy`).
- **Never try to bypass delete/move restrictions** (no tricks such as `find -delete`,
  `git rm`, truncating files, writing scripts that unlink, etc.). If something
  must be deleted or moved, add it to the "Pending manual cleanup" list below
  and carry on.
- **Keep going.** Don't stop and wait for manual user intervention. If a step
  is blocked (permission, missing tool, device), note it, work around it, and
  continue with the next task. Summarize the blocked items at the end.

### Design
- **Use Mobbin heavily.** Before designing or building any screen/component,
  search Mobbin (`mcp__claude_ai_Mobbin__search_screens` / `search_flows`,
  both `ios` and `web`) for several references, and cite the ones used (Mobbin
  links) in the PR/commit or in `docs/design-language.md`.
- Feature references: Element (completeness), FluffyChat (simplicity, phone
  layout), Fluxer (desktop rail/channel density).
- Minimal and quiet: neutral grays carry the UI, with **one light-purple accent** used
  sparingly (selection, primary action, mentions, focus). No gradients, glows,
  or decorative shadows (only floating layers get one soft shadow).
- All colors, sizes, and motion come from `kit/` tokens. Don't use ad-hoc hex values in
  screens.
- Motion follows Emil Kowalski's rules (load the `emil-design-eng` / `animate` skills):
  subtle and interruptible, 100–250 ms, ease-out on enter, faster exits, press
  scale 0.97, never animate from scale 0, popovers scale from their trigger.
  **No animation** on keyboard-driven, high-frequency UI (the Ctrl+K palette,
  room switching, list navigation). Respect reduced motion.
- Accessibility: AA contrast (tested in `kit` tests), labels on icon buttons,
  keyboard reachable, ≥44 DIP touch targets on phones.

### Performance
- Idle means no frames. Long collections use `ui.List` (never a `Scroll` of
  every row). Do I/O off the UI thread and publish results with `Window.Update`.
- Images: decode with `DecodeBitmapScaled` to their display size, use an LRU cache,
  and keep GIFs animating only while visible.

### mygo specifics
- Pinned mygo: see `go.mod`. Source: `go list -m -f '{{.Dir}}' github.com/egoist/mygo`;
  read `docs/ui/*.md` before using a widget. Follow `.agents/skills/mygo-maintenance`.
- **Never build without `-tags goolm`** (mautrix's crypto is pure Go only with
  it; the default pulls libolm via cgo and fails). `mygo dev`, `mygo build` and
  `mygo generate` take it from the environment: `GOFLAGS=-tags=goolm mygo dev`
  (the package.json scripts carry it; a bare `mygo dev` breaks the build).
- Never store `ui.Element` values in state; give stateful widgets keys
  (`c.Key`).

### Verification
- `go build -tags goolm ./... && go vet -tags goolm ./... && CGO_ENABLED=0 go test -tags goolm ./...`
  (the `goolm` tag makes mautrix's crypto pure Go; without it, `matrix/`
  pulls in libolm via cgo and the build fails — never build without it.)
- `go tool mygo vet .` when available.
- Visual check without a window: `SNAP_DIR=<scratch dir> go test -run TestPagesRender ./gallery/`
  writes a PNG of every gallery page (wide/phone × light/dark). Read them.
- Live: `go run .` opens the gallery window.

## Layout
- `app/`: the app itself: the shell in all three layouts (Rail / Tabs / One
  inbox) and the phone layout, room view, side panel, People, Settings and
  every overlay. Built from `kit` on a `model.Source`; never touches Matrix
  directly.
  The app's composer pieces live in `app/files.go` (files waiting as chips,
  paste, drop, GIFs uploaded), `app/suggest.go` (@mentions, `:emoji:`),
  `app/actions.go` (message menu, edit, pins), `app/previews.go` (link
  cards), `app/history.go` (asks for older messages near the top).
  Stage-5 app files: `app/rooms.go` (join/create/explore + invite dialogs, the
  Invitations section), `app/search.go` (⌘⇧F screen), `app/threads.go` (thread
  panel; replies hidden from the timeline only while `threadsOn`), `app/audio.go`,
  `app/notify.go` (`showNotification` is a var for tests), `app/profile.go`.
  Edit tool gotcha: a `<` typed into a tool parameter is decoded before it
  reaches the file; write `\\u003c` inside an interpreted Go string.
  Later stage-5 files: `app/roomedit.go` (edit name/topic/picture of a room or
  community), `app/spaces.go` (drag-to-reorder on the rail, "More to join" from a
  space's hierarchy), per-room notification level (`model.NotifyLevels`),
  ignore a user (`model.Ignorer`, `matrix/ignore.go`, Privacy → Ignored
  people), Appearance settings (accent tint `kit/accent.go`, interface size =
  window zoom, compact timeline = `kit.Dense`), phone equivalents of find / pins
  / thread (room sheet rows + `panelSheetView` in `app/overlays.go`), and
  `model.CacheResetter` (Advanced → Reset local cache). Rail order is account
  data `com.slopement.rail_order`. Picker work (step 7): tiles load only in view
  with a 4-download limit, retry and a skeleton wave (`picker/picker.go`,
  `picker/media.go`, `kit/gifpicker.go`), Discord import (`app/gifimport.go`,
  `picker/discord.go`), KLIPY share (`klipy.Share`), stickers as `m.sticker`
  (`model.StickerSender`, `matrix/sticker.go`, `app/stickers.go`), MSC2545 packs
  (`model.Packs`, `matrix/packs.go`, `app/packs.go`, `kit/packpicker.go`,
  `format.Emoji`), and GIF folders (`picker/folders.go`, `app/gifsettings.go`,
  uiState `GifFolders`). The emoji set is the full Unicode list from
  `github.com/forPelevin/gomoji` (`kit/emojiset.go`: Discord's categories, no
  skin-tone variants, fully qualified, curated words kept for search; the grid
  builds only the rows in view). Read-only rooms (`model.Room.ReadOnly` from
  `m.room.power_levels`, `matrix/power.go`) show `kit.ReadOnlyBar` instead of the
  composer; a profile card's picture opens the viewer (`app/profilepic.go`). Tests that build an `App` with `picker.New(nil)` must
  `t.Setenv("HOME"/"XDG_CONFIG_HOME", t.TempDir())`, or they write the user's
  real `picker.json`. Security (Settings): `matrix/keys.go` + `app/keys.go` —
  key storage status (`model.KeyStorage`), export/import of room keys (Element's
  file format, locked with a passphrase), identity reset
  (`model.IdentityResetter`, UIA by password or by a browser approval, MSC4312
  stages `m.oauth` / `org.matrix.cross_signing_reset`). Not built: creating a
  new key backup (make it in Element, restore here). Logs: `internal/diag`
  (one redacted file a day under `<config>/slopement/logs`, 7 days, 20 MB; an
  export hashes IDs), wired in `logging.go` and `matrix/diagnostics.go`, exported
  from Advanced (`model.LogExporter`). Previews from this device
  (`linkpreview/`, never fetches private addresses; `app/previews.go`, remote
  pictures in `app/media.go`), hide joins/leaves (`model.Event.Membership`,
  `app.rows`), animated GIF avatars (`kit.AvatarSpec.Anim`, `app/avataranim.go`:
  original fetched lazily when it would play). Only the call settings are still
  "Soon" in settings.
  Compatibility: `testdata/events/` holds raw events per client/bridge with
  `.golden` dumps (`matrix/fixtures_test.go` via `matrix.ConvertEvent`; render
  check in `app/fixtures_test.go`). A test process cannot write files here, so
  `UPDATE_GOLDEN=1` prints the dumps and they are saved with Write. Location
  cards (`m.location`, `kit.LocationCard`, opens OpenStreetMap), emote/notice
  kinds, View source (`model.EventSourcer`, `matrix/eventsource.go`,
  `app/source.go`) and the per-row crash guard (`app/saferow.go`: a panicking
  row becomes a "Couldn't show this message" tile; tests assert
  `a.rowPanics` empty).
  File dialogs go through `app/dialogs.go` (tests replace `pickFileFn` /
  `saveFileFn`). Voice recording (and Opus/AAC decode) is in the
  mobile-polish/device-pass scope (plan step 9), not stage 5: mygo has no
  capture API yet.
- `model/`: the app's data types and the `Source` interface (the Matrix
  layer will implement it). `model/mock`: fake data with simulated replies.
- `login/`: the sign-in screen over `matrix/` (probe a server, the MAS
  browser flow or a password form), handing the session to `main.go`,
  which then swaps the window to the app on the account. Reached with
  `go run .` (the app always starts on the sign-in; the fake account of
  `model/mock` opens only with `SLOPEMENT_MOCK=1 go run .`, for interface work).
- Accounts: `main.go` owns the one account on screen (`live`), the last used
  is remembered (`matrix.SetLastUsed`, file `last-account`) and opened on its
  own (`login.Resume`); the app's account menu (`app/accounts.go`, on the
  avatar) switches, adds (`login.BeginAdd`) and removes. `SLOPEMENT_DEV_SECRETS=1`
  (set by `npm run dev`) keeps secrets in `dev-secrets.json` instead of the
  keychain, so ad-hoc dev builds don't re-prompt. Tests that sign in must use
  `matrix.Isolate` (see `login/main_test.go`).
- `matrix/`: the Matrix layer (mautrix-go, `-tags goolm`): one `Account`
  per login — its directory (`<config>/slopement/accounts/<sha256(user)>/`)
  holds `state.db`, `crypto.db` and `cache.db` (the room cache: rooms,
  timelines, waiting encrypted events, people, DMs, sealed with AES-GCM under
  the keychain key, written in one transaction with the sync token so a launch
  resumes the sync; `persist.go`, `cache.go`) and `ui.json` (the app's own
  state: layout, theme, settings, last room, drafts; `app/uistate.go`), a
  session lock enforces one process per
  account, and tokens/pickle keys live in the OS keychain via `mygo.Secrets`.
  Login probes a server (MAS/OIDC first-class) and `NewSynced` runs the
  classic `/v3/sync` loop into `model.Source`: rooms, timeline (capped at
  300/room in memory), presence, typing, reactions, sending and read
  markers, with mautrix's olm machine decrypting through goolm, and
  `newVerify` the verification: SAS emoji with another device or the
  Security Key (SSSS) with the key backup behind it — after which the
  still-encrypted events re-decrypt into their tiles. Sliding sync
  (MSC4186) replaces the loop behind the same constructor later.
  Optional capabilities the app asks of a `Source` through small interfaces
  in `model/` (the mock needs none): `History` (`history.go`: pages older
  events in above what is held, by `/messages`), `Uploader` (`upload.go`:
  sends files, encrypted in encrypted rooms), `Actions` (`actions.go`: edit,
  delete, pin; each shows at once, a refusal is said as a quiet line),
  `Previewer` (`preview.go`: the homeserver's `preview_url`), `MediaLoader`
  (`media.go`: attachments and avatar thumbnails), `ReceiptPrivacy`, and the
  stage-5 ones: `DMCreator`/room tags/mute/mark-unread/leave
  (`roomactions.go`: mute is a room push rule with no actions, timed ends in
  account data `com.slopement.mutes`), `Polls` (`polls.go`, MSC3381 both
  shapes; votes live in `room.polls` and the cache), `Profile`/`UserColors`/
  `SessionManager` (`profile.go`, `avatar.go`; colors are account data
  `com.slopement.user_colors`), `RoomManager` (`rooms.go`: create, join by
  address/link, invite, directory, invitations — kept out of the room handlers
  by `takeInvites`), `Notifier` (`notify.go`), `Searcher` (`search.go`: the held
  timelines, which keeps encrypted rooms searchable without a plaintext index,
  plus the server's `/search` for unencrypted ones; `PageBack` pages older
  history in so old encrypted messages become searchable — ⌘F's find bar
  (`app/find.go`, one room) and the all-chats search's "Search further back"
  both use it; the matching rules are
  `model.SearchQuery.Match`), `Threads` (`SendInThread`; `Event.ThreadRoot`,
  `Thread` summary built in `room.timeline`). Voice-message *recording* is not
  built: mygo's `audio` package only plays (no capture), and has no Opus/AAC
  decoder (`app/audio.go` plays WAV/MP3/Vorbis/FLAC/ALAC). Events
  paged in are marked `sourceHistory` and placed by time by `insertOlder`
  (no unread, no preview change). A limited sync (more events since the last
  one than its 30-event timeline) leaves a hole between the cache and the sync:
  `matrix/gapfill.go` fetches it from `prev_batch` (up to 500 events; a bigger
  hole drops the old cached events and continues history from the fill).
- `kit/`: design tokens, theme, and components (the design kit).
- `gallery/`: the component gallery and mock screens, built only from `kit`.
  It opens from **Debug → Component Gallery** (⌘⌥G) in its own window.
- `klipy/`: the KLIPY client (native API: GIFs, stickers, categories).
  `KLIPY_LIVE=1 go test ./klipy/` hits the real API with the key from `.env`.
- `picker/`: the picker's controller (KLIPY fetches, favorites, recents,
  animated thumbnails with an LRU). Background work reaches the UI only
  through `SetUpdate(win.Update)`.
- `format/`: Discord-flavoured Markdown ↔ tree ↔ Matrix HTML, plus the
  sanitizer. Pure Go, table-tested.
- `embeds/`: classifies links (YouTube, Imgur, direct media, generic).
- Live picker screenshots: `KLIPY_LIVE=1 SNAP_DIR=… go test -run LivePicker ./gallery/`.
- `internal/dotenv/`: loads `.env` (gitignored; template `.env.example`).
- `platform/`: native window chrome (hidden title bar, vibrancy/Mica), the
  system menu bar (`MenuBar(Actions, win.Update)`), GTK styling. Use
  `kit.ContextMenu` / `kit.MenuButton` (drawn kit menus everywhere; only
  the menu bar and file dialogs are native) rather than raw `PopoverMenu`, and `c.TitleBar()` + `DragWindow` in any toolbar or
  sidebar that touches the window's top edge.
- `docs/`: `requirements.md`, `design-language.md`, `landscape.md` (other
  Matrix clients), `calls-research.md` (Jitsi vs Element Call; Jitsi dropped 2026-11-23), `ideas.md`
  (far-future ideas, e.g. activities in calls).
- `main.go`: the app entry: the app on mock data; the gallery behind Debug.
- App snapshots: `SNAP_DIR=… go test -run 'Layouts|SendAndNavigate' ./app/`.

## Voice messages and dev builds
- Voice recording: `voice/` (WAV, waveform, level; no platform code), `app/voice.go`
  (the `Mic` seam, `NewMic` nil where there is no microphone, the recording
  state, send as a voice message via `model.File.Voice` and `matrix/upload.go`'s
  `voiceContent`), `kit/voicebar.go` (the bar), and the native module
  `modules/mic` (Swift for macOS+iOS, Kotlin for Android; `mic_native.go` and the
  `*.mygo.*` files are generated: never edit). `micnative.go` hands it to the app.
  **Building the module runs `swift build` inside `modules/mic/swift`, which macOS
  refuses under ~/Documents for some processes** ("You don't have permission to
  save the file “repositories” in the folder “.build”"): check the Swift code with
  `swift build --package-path modules/mic/swift --scratch-path <scratch>` (add
  `--sdk $(xcrun --sdk iphonesimulator --show-sdk-path) --triple
  arm64-apple-ios16.0-simulator` for iOS).
- A development build (`SLOPEMENT_DEV_SECRETS=1`, which `npm run dev` sets) keeps
  its accounts in `<config>/slopement-dev`, apart from the installed app's
  `<config>/slopement`: sharing one record made the two builds sign in over each
  other, a new device each time (`defaultConfigDir`).
- Android APK: `SLOPEMENT_ANDROID=1 GOFLAGS=-tags=goolm mygo build -platform android/arm64`
  (add `SLOPEMENT_VERSION=x.y.z` to name the version), written to
  `build/android/slopement-<version>.apk` (and `.aab`), signed with the debug key
  unless `android.keystore` is set. `SLOPEMENT_ANDROID=1` is needed: Android's
  package is `io.github.dariusit.slopement` (no hyphens allowed), while desktop
  and iOS keep `io.github.darius-it.slopement` (the keychain service: never
  change it). Keep the Android one once an APK is shared. A stale Gradle daemon
  once failed aapt2 on the launcher icons; stopping it fixed that.
- Do not `go build .` for another OS in the repo root: it overwrites the tracked
  `slopement` binary. Use `-o /dev/null`.

## Newer pieces
- `animwebp/`: animated WebP decoder (a `ui.VideoDecoder`); `app/media.go`
  plays them like GIFs (the player comes from the fetched bytes, by URL).
- `audiodec/`: Ogg Opus and AAC-in-m4a to WAV (pure-Go libraries
  `kazzmir/opus-go`, `tphakala/go-m4a`), used by `app/audio.go`'s `openSound`.
- Private nicknames and notes: `model.PrivateNotes`, `matrix/notes.go` (account
  data `com.slopement.person_notes`), `app/notes.go`, `app/names.go`
  (`nameFor`: nickname, else username/display name by the setting).
- Custom emoji reactions: `model.CustomReactor` (`ReactCustom`), key = mxc URI
  plus `shortcode` (MSC4027); kit pills draw the picture.
- Runs of joins/leaves are one line (`app/membership.go`); communities reorder
  by drag on the rail, the tabs' cards and the inbox (`reorderable`).
- A DM without a chosen name is named by its partner's display name now
  (`Source.dmName`, `room.nameSet`).
- Account data a resumed sync does not resend is fetched at start
  (`matrix/accountdata.go`): add new `com.slopement.*` account data there.

## Updates
Opt-in, never automatic (`app/updates.go`): Settings → About switch (off by
default, kept per device in `<config>/slopement/updates.json`), one check at
startup per run, "Check for updates now" beside it, a dialog, Install only on
a press. Built on `mygo.Updater` (signed archives; `theUpdater` is the test
seam). `mygo.config.ts` turns the updater on when `mygo-update.pub` exists
(`mygo keygen`; commit the .pub, keep the .key out of the repo and in the
Actions secret `MYGO_UPDATER_PRIVATE_KEY`). The version is the tag without `v`
(`SLOPEMENT_VERSION`), and all releases are pre-releases, so the check
asks for pre-releases. Do not lose the secret key: installed apps trust only it.

## Performance rules learned
- The sidebar lists (Chats in every layout, One inbox) are `ui.List`s of
  `listLine`s (`app/lists.go`, `showLines`): they used to be a `Scroll` with a
  row built for every chat, every frame, which made typing sluggish on Linux
  (a friend's frame log: GPU present 0.3 ms, our build 8–9 ms). Never build an
  element per item of a long collection.
- `app/framecache.go` keeps the source's room, community and online-count lists
  while the source has not changed (`OnChange` bumps `srcGen`; at most 30 s),
  and the contacts for one frame. A source that changes state must call its
  change listeners, or the lists stay as they were.
- Measure with `go test -tags goolm ./app -run XXX -bench TypingFrame
  -cpuprofile <scratch>/cpu.prof` (a heavy account via `mock.AddBulk`), and a
  user's `MYGO_FRAME_STATS=1` log: "build" is ours, "present" is the GPU.
- Still not virtual: a community's room list (`communityRooms`), the People grid.

## Releases
`.github/workflows/release.yml`: push a `v*` tag (e.g. `v0.1.0-alpha.1`) to
build macOS, Windows and Linux with `mygo build` and publish the installers as
a pre-release (unsigned: no certificates or updater key yet). `docs/known-gaps.md`
lists what is missing; keep it current. The app starts on the sign-in; the mock
account is `SLOPEMENT_MOCK=1` only. `identifier` in `mygo.config.ts` is the
keychain service: do not change it after a release.

### Licenses
Settings → About lists the open source software in the build with its license
text (`internal/licenses`, embedded `notices.json`), and `THIRD_PARTY_NOTICES.md`
goes with each release. Both are made from go.mod: **after adding or updating a
dependency, run `go run ./internal/licenses/gen` and commit the result** (the
release workflow fails on a stale list). The command prints any license it does
not recognise and any copyleft one; read those before shipping. Something the
app uses that is not a Go module (an icon set, a service) goes in
`licenses.Credits`. Do not add a dependency with a license stronger than MPL-2.0
without asking: `TestOnlyKnownCopyleftIsInTheBuild` says which are known.

### Changelog
`CHANGELOG.md` is the release notes' first section: the workflow takes the
section named after the tag (`## v0.1.0-alpha.6`), or else `## Unreleased`, and
puts it above the standing warning. **When you add or change something a user
would notice, add a line under Unreleased** (plain words: what it does, not how).
When tagging, rename Unreleased to the tag and start a new Unreleased above it.

## Secrets
- `KLIPY_API_KEY` lives in `.env` (gitignored). Never commit keys or print them.
  Releases carry no key: users add their own in Settings → GIFs
  (`picker.SetKlipyKey`, in `picker.json`; the environment's key wins).

## Pending manual cleanup (needs user; do not bypass)
- Delete `matrix/debug_lock_test.go` (an emptied one-off diagnostic).
- Delete the leftover webview scaffold: `index.html`, `src/`, `vite.config.ts`,
  `tsconfig.json`, and the Vite/TS parts of `package.json` / `bun.lock`
  (`mygo.config.ts` no longer names them; the release workflow runs a plain
  `bun install`, not `--frozen-lockfile`, until `bun.lock` is regenerated).
- Rename `kit/palette.go` → `kit/switcher.go` (holds the ⌘K Switcher).
- Delete `app/zz_probe_test.go` and `app/zz_dbg_test.go` (emptied one-off
  diagnostics).
- Delete `kit/zz_gomoji_probe_test.go` (an emptied one-off probe).
- Delete `app/zz_dbg_updates_test.go` (an emptied one-off diagnostic).
- Remove the 40 MB compiled binary `slopement` at the repository root from git
  (`git rm --cached slopement`, then the file): it was committed by accident in
  "Prepare for alpha release and builds" (4b04d9f) and is in the history; `.gitignore`
  now keeps a new one out, but it stays tracked until removed. A history rewrite
  would shrink the repository (the .git is ~31 MB); that is the owner's call.
- Delete `slopement.exe` at the repository root (a Windows build made to check
  the code compiles; it is ignored, not tracked).
- Delete `matrix/sync_test.go.bak` (a stray backup from a shell `sed -i`; the
  edit itself is already in `matrix/sync_test.go`).
