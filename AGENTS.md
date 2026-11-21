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
- Never store `ui.Element` values in state; give stateful widgets keys
  (`c.Key`).

### Verification
- `go build ./... && go vet ./... && CGO_ENABLED=0 go test ./...`
- `go tool mygo vet .` when available.
- Visual check without a window: `SNAP_DIR=<scratch dir> go test -run TestPagesRender ./gallery/`
  writes a PNG of every gallery page (wide/phone × light/dark). Read them.
- Live: `go run .` opens the gallery window.

## Layout
- `kit/`: design tokens, theme, and components (the design kit).
- `gallery/`: a component gallery window and mock screens built only from `kit`.
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
- `main.go`: the app entry (currently opens the gallery).

## Secrets
- `KLIPY_API_KEY` lives in `.env` (gitignored). Never commit keys or print them.

## Pending manual cleanup (needs user; do not bypass)
- Delete the leftover webview scaffold: `index.html`, `src/`, `vite.config.ts`,
  `tsconfig.json`, and the Vite/TS parts of `package.json` / `bun.lock`; then
  drop `devUrl`/`devCommand`/`buildCommand`/`frontendDist`/`bindings` from
  `mygo.config.ts`.
- Rename `kit/palette.go` → `kit/switcher.go` (holds the ⌘K Switcher).
