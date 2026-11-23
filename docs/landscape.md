# The Matrix client landscape (researched 2026-11-21)

Question: does any Matrix client already come close to slopement's features
and polish? **No.** A few cover a third to a half of the list. None pairs
that with a native, low-resource app on desktop *and* phones. That gap is
slopement's reason to exist.

How far to trust this: the best structured source is an unofficial guide
dated October 2026 that covers only five clients. Another is one person's
opinion gist, which contradicts the others in places (Cinny's
verification and calls). Several clients (Zam, NeoChat, Robrix) had no
feature data. Treat this as a map of where to look; recheck each project's
release notes before relying on a claim.

## Closest to our feature set

| Client | Has, from our list | Falls short on | Stack, status |
|---|---|---|---|
| **Sable** (Cinny fork) | Spaces and sub-spaces, polls, custom emoji and sticker packs, GIF search, calls, multi-account, verification | URL previews partial; no real phone app (iOS sideloading only) | React web + Linux desktop; beta |
| **Commet** | Spaces, threads, custom emoji and stickers, GIF search, multi-account, encrypted room search | Sends GIFs as uploaded files; calls unclear; no macOS or iOS yet | Flutter; beta |
| **Element Web/Desktop** | The most complete: polls, threads, verification, link previews, spaces | No GIF button, weak custom emoji (sometimes shows `mxc://` URLs), single account, sub-spaces hard to navigate | TypeScript/React, Electron (moving onto matrix-rust-sdk); stable |
| **FluffyChat** | Polls, threads (since 2.3.0), multi-account, strong on phones, MatrixRTC calls | Not dense or Discord-like on desktop; no GIF search | Flutter (matrix-dart-sdk); stable |
| **Cinny** | The most Discord-like layout, custom emoji, calls (4.12+), voice/video rooms (4.11+) | No polls; threads partial; a report that muting doesn't clear the unread state | React, Tauri desktop; stable; no mobile |
| **Element X** | Spaces (relaunched March 2026), sliding sync, Element Call | Threads behind Labs; no desktop or web app | Kotlin/Swift on matrix-rust-sdk; stable |
| **Zam** | Discord-style, Android share sheet, per-device mic/camera selection | No feature data | Electron + PWA; beta |

## Native and low-resource (our approach)

| Client | Notes |
|---|---|
| **Robrix** | Rust on Makepad (GPU-drawn), mobile and desktop, an IDE-like dock of room tabs. Pre-alpha, and needs native sliding sync. The closest in *architecture*; far behind on features. |
| **Nheko** | C++/Qt, desktop only, 1:1 calls only; beta. |
| **Fractal** | Rust/GTK, Linux only; beta. |
| **gomuks** | **Go on mautrix-go, our own stack.** A Go backend with a web frontend (a terminal frontend is coming). **Nexus** is a Flutter UI on that backend. |
| **Komai / Lightning** | Qt on matrix-rust-sdk; beta. Lightning's calls reach Element. |

## Features no client seems to have (our differentiators)

None of the sources mentions any client with these:
- **Custom status with expiry.** MSC4133 extended profiles is in the spec
  since **v1.16** (custom profile fields, the `m.profile_fields`
  capability). It deliberately defines **no** status or free-text field,
  over moderation concerns (MSC4440 proposes a biography field on top). So
  our `org.slopement.status` is a custom field that only slopement shows.
  That fits "per-viewer first", but it's worth knowing.
- Timed mute (15 min to 1 week).
- Private nicknames, notes and per-viewer username colours. (Cinny and
  Sable have Discord-style role colours, which is a different thing.)
- Importing Discord GIF favourites; sole-link GIFs shown as GIFs.
- Link previews that work in encrypted rooms (sender-bundled, MSC4095).
- A Ctrl+K switcher plus a thought-bubble people view, in one native app.

## What this changes in our plan

1. **Calls.** Most of the ecosystem is moving to Element Call (MatrixRTC):
   Element, Element X, FluffyChat and Cinny list it, and only Element Web
   still lists Jitsi as well. A Jitsi-only slopement would mostly call
   itself. **Decided 2026-11-23:** MatrixRTC is the only call system; Jitsi
   is dropped. See [calls-research.md](calls-research.md).
2. **Matrix core.** gomuks's backend is Go on mautrix-go. Check whether
   mautrix-go ships it as a reusable package (believed to be `hicli`: SQLite
   store, sync loop, crypto, a high-level timeline) **before** writing our
   own `matrix/` layer in step 3. It could save much of that layer.
   (Not verified yet.)
3. **Borrow, don't copy:**
   - Sable and Cinny for Discord-style desktop interaction;
   - Commet for GIF and custom emoji pack handling;
   - FluffyChat for the phone layout and onboarding;
   - Element for completeness and edge cases (verification, recovery,
     notification rules).

## Sources

- [Matrix client comparison, matrixdocs (Oct 2026)](https://matrixdocs.github.io/docs/clients/comparison)
- [Alternative Clients, matrixdocs](https://matrixdocs.github.io/docs/clients/alternatives)
- [Matrix.org – Clients](https://matrix.org/ecosystem/clients/)
- [Matrix Clients and their pros and cons (opinion gist, Aug 2026)](https://gist.github.com/FireMario211/1bdbee07a5d56dc27891ae0c362d1e3d)
- [Sable on GitHub](https://github.com/SableClient/Sable)
- [Sable on matrix.org](https://www.matrix.org/ecosystem/clients/sable/)
- [Sable changelog](https://www.mintlify.com/7w1/sable/development/changelog)
- [Commet on GitHub](https://github.com/commetchat/commet)
- [Commet on matrix.org](https://www.matrix.org/ecosystem/clients/commet/)
- [Commet on LinuxLinks](https://www.linuxlinks.com/commet-chat-client-matrix/)
- [Cinny on matrix.org](https://matrix.org/ecosystem/clients/cinny/)
- [This Week in Matrix, 2026-10-02 (Cinny 4.12.7)](https://matrix.org/blog/2026/10/02/this-week-in-matrix-2026-10-02/)
- [Zam on matrix.org](https://matrix.org/ecosystem/clients/zam/)
- [Zam releases](https://github.com/az4521/Zam/releases)
- [gomuks on GitHub](https://github.com/gomuks/gomuks)
- [gomuks on matrix.org](https://www.matrix.org/ecosystem/clients/gomuks/)
- [Robrix README](https://github-redirect.dependabot.com/project-robius/robrix)
- [Robrix talk, Matrix Conference 2026](https://cfp.2026.matrix.org/matrix-conference-2026/talk/PUQNED/)
- [Spaces has landed on Element X](https://element.io/blog/spaces-has-landed-on-element-x/)
- [New Element Web and Desktop apps](https://element.io/blog/new-element-web-and-desktop-apps-have-distinct-element-x-vibes)
- [FluffyChat 2.2.0](https://newreleases.io/project/github/krille-chan/fluffychat/release/v2.2.0)
- [FluffyChat on Flathub](https://flathub.org/en/apps/im.fluffychat.Fluffychat)
- [Matrix v1.16 release](https://matrix.org/blog/2025/09/17/matrix-v1.16-release/)
- [MSC4133 extended profiles](https://git.sudo.is/matrix/matrix-doc/src/branch/main/proposals/4133-extended-profiles.md)
- [MSC4440 biography field](https://github.zerozr99.workers.dev/matrix-org/matrix-spec-proposals/pull/4440/files)
