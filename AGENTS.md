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
- `model/`: the app's data types and the `Source` interface (the Matrix
  layer will implement it). `model/mock`: fake data with simulated replies.
- `login/`: the sign-in screen over `matrix/` (probe a server, the MAS
  browser flow or a password form), handing the session to `main.go`,
  which then swaps the window to the app on the account. Reached with
  `SLOPEMENT_LOGIN=1 go run .`.
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
  (no unread, no preview change).
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
  Matrix clients), `calls-research.md` (Jitsi vs Element Call), `ideas.md`
  (far-future ideas, e.g. activities in calls).
- `main.go`: the app entry: the app on mock data; the gallery behind Debug.
- App snapshots: `SNAP_DIR=… go test -run 'Layouts|SendAndNavigate' ./app/`.

## Secrets
- `KLIPY_API_KEY` lives in `.env` (gitignored). Never commit keys or print them.

## Pending manual cleanup (needs user; do not bypass)
- Delete `matrix/debug_lock_test.go` (an emptied one-off diagnostic).
- Delete the leftover webview scaffold: `index.html`, `src/`, `vite.config.ts`,
  `tsconfig.json`, and the Vite/TS parts of `package.json` / `bun.lock`; then
  drop `devUrl`/`devCommand`/`buildCommand`/`frontendDist`/`bindings` from
  `mygo.config.ts`.
- Rename `kit/palette.go` → `kit/switcher.go` (holds the ⌘K Switcher).
- Delete `app/zz_probe_test.go` (an emptied one-off diagnostic).
- Delete `matrix/sync_test.go.bak` (a stray backup from a shell `sed -i`; the
  edit itself is already in `matrix/sync_test.go`).
