# Calls: Jitsi vs Element Call (MatrixRTC) (researched 2026-11-21)

> **Update 2026-11-23: Jitsi is dropped.** slopement supports MatrixRTC
> (Element Call) only, with no fallback provider and no provider setting. This
> file is kept as the research record; everything below about Jitsi, the
> fallback, "Move this call to Jitsi" and the v1.1 provider setting is
> **not planned**. What is built is described in `docs/requirements.md` →
> Calls.

The question: which call system should slopement use? Can we support both,
switch between them, or fall back from one to the other?

Our own experience so far: Jitsi calls have worked best, and Element Call
has felt janky. The research below backs up both halves of that, and also
shows that the ecosystem is moving to Element Call anyway.

**Decision (2026-11-21): MatrixRTC (Element Call) is the main call system,
and Jitsi is the fallback. Both ship in v1.** They sit behind one `calls`
interface. A "prefer Jitsi" setting and per-room overrides come in v1.1.
The phasing table below reflects this; the "Which provider" rules are the
v1 behaviour except where marked.

## The two systems

| | **Jitsi** (widget) | **Element Call** (MatrixRTC) |
|---|---|---|
| What it is | A Jitsi Meet conference attached to the room as a widget (`im.vector.modular.widgets`), running on a Jitsi server (e.g. meet.element.io) | Matrix-native group calls. Signalling is MatrixRTC (MSC4143); media goes through a LiveKit SFU (MSC4195) |
| Signalling | Jitsi's own servers (XMPP/Prosody). Matrix only stores the widget | Matrix room state (`m.rtc.member` / legacy `org.matrix.msc3401.call.member`) plus to-device messages |
| Server needs | A Jitsi deployment (or a public one) | A LiveKit SFU **and** the MatrixRTC authorization service (`lk-jwt-service`). Discovered via the homeserver: older setups use `.well-known` `org.matrix.msc4143.rtc_foci`, newer ones a client-server endpoint for RTC transports |
| Encryption | Jitsi's E2EE uses insertable streams and, per Jitsi, **works only in Chromium-based browsers**. On mobile or other browsers it's off, so media is encrypted only to the Jitsi server | **Real E2EE:** per-participant media keys (insertable streams), rotated when people join or leave, shared over Matrix's Olm channel (to-device) |
| Who can join | Element Web, and anything that renders Jitsi widgets. Element Classic mobile (sunset **31 Dec 2026**) | Element Web, Element X (iOS/Android), FluffyChat, Cinny, Sable, Element Pro. The direction of the ecosystem |
| Interop | **None** with Element Call: the two can't join each other's calls (Element's own docs) | Same |
| Maturity | Old and proven. Quality is generally good | Element calls it the replacement for Jitsi. It is still maturing: open issues in 2026 (below) |
| How we'd embed it | A web view with Jitsi's iframe API (`external_api.js` served by the Jitsi server) | A web view with Element Call's **embedded package**, talking to *us* over the **widget API** (postMessage). We act as the widget host and do the Matrix side |

## Evidence on reliability (our "janky" experience)

Element Call and Element X have had public reliability problems through
2026:
- "Poor call performance": element-call#4206 (Aug 2026).
- "Unable to join calls": element-call#4162 (Aug 2026).
- Audio drops both ways when the app is backgrounded: element-call#4184
  (Aug 2026).
- Calls that hang with no way to end them (#3656). In Nov 2026 a
  maintainer said they had "worked a lot on reliability and updated our
  flows this summer".
- Element X Android 26.10.0: incoming calls ring once, then the answer UI
  disappears (element-x-android#7979). Some devices don't ring at all
  (#8154, Nov 2026).

Jitsi has fewer moving parts on the Matrix side: no to-device key exchange,
no LiveKit JWT service, no delayed-leave events. That fits its feeling more
solid. Its weak spots are E2EE (Chromium only) and the slow shrinking of
the clients that can join Jitsi widgets (Element Classic mobile ends with
2026; Element Web now labels the old stack "Legacy Voice & Video" and can
turn legacy calls off).

**Conclusion:** Element Call is where the people are going, and the only
option with real E2EE. Jitsi is the more reliable experience *today*.
Neither alone is right for every room, so we support both.

## Design: one call interface, two providers

```go
// calls/provider.go
type Provider interface {
    Kind() Kind // Jitsi or ElementCall
    // Active reads the room's state: is a call of this kind going, and who's in it?
    Active(room RoomState) (Call, bool)
    // Supported: can this account start one here? (EC: the homeserver has an RTC
    // transport/LiveKit; Jitsi: a Jitsi domain is configured or the default is reachable.)
    Supported(ctx context.Context, room RoomID) error
    Start(ctx context.Context, room RoomID, o Options) (Session, error)
    Join(ctx context.Context, call Call, o Options) (Session, error)
}
type Session interface {
    SetMic(on bool); SetCamera(on bool); ShareScreen(on bool); Leave()
    Events() <-chan Event // joined, participant changes, quality, error
}
```

Options: camera off by default, plus `AudioOnly`. The native control bar
(mic, camera, screen, leave) is the same for both. Each provider's web
view has its own UI hidden.

### Which provider is used

1. **Join what's already there.** If the room has an active call of
   either kind, the Call button joins *that* one, so we never split a call.
   If both kinds are somehow active, the header shows both, with
   participant counts.
2. **Starting a new call (v1):** MatrixRTC, when the homeserver
   advertises a MatrixRTC transport; otherwise Jitsi straight away.
   - **v1.1:** a call system setting (MatrixRTC with Jitsi fallback
     (default) / Always Jitsi), and a **per-room override** in room
     account data (`com.slopement.call_provider`). Use it for a friends'
     room that always uses Jitsi.
3. **Fallback, only before anyone else has joined.** Starting with
   MatrixRTC can fail: the JWT service refuses, the SFU is unreachable,
   ICE fails, or there's no media within ~8 s. Then we offer
   **"MatrixRTC isn't working here. Use Jitsi instead?"** and say why; one
   click starts Jitsi. In v1.1 the user can make this automatic.
4. **Never switch silently mid-call.** If a running call degrades, the
   call menu offers **"Move this call to Jitsi"** (or the reverse). That
   starts the other kind and posts a notice in the room ("Mira moved the
   call to Jitsi · Join") so everyone follows deliberately.

### Showing a running call

- A room with a MatrixRTC call shows "Call · N people · Join", and a room
  with a Jitsi widget shows "Jitsi call · Join". Both join in place.
- If the embedded Element Call fails to load (e.g. no WebRTC in the web
  view), offer "Open in browser" for the MatrixRTC call, and Jitsi for new
  calls.

## Implementation notes

- **Web views:** both providers need WebRTC in the *system* web view that
  mygo's `ui.WebView` wraps:
  - WKWebView (macOS/iOS) and WebView2 (Windows) support `getUserMedia`.
  - WebKitGTK's WebRTC on Linux is newer and depends on how it was built:
    **test this early**. It may force a "Open in browser" fallback on
    some distros.
  - Camera and microphone permissions go through the web view's
    permission callback (and Info.plist / Android permissions on phones).
- **Jitsi provider (v1, the fallback):**
  - The widget's URL comes from room state. Create one from
    `.well-known` `im.vector.riot.jitsi.preferredDomain`, falling back to
    meet.element.io.
  - Iframe API config: `startWithVideoMuted: true`,
    `startWithAudioMuted: false`, `startAudioOnly` for audio-only,
    `prejoinConfig.enabled: false`, `disableDeepLinking: true`, and an
    empty toolbar (our bar drives it via `executeCommand`).
  - Some start-muted options were once ignored when passed in the URL
    hash (jitsi-meet#8144), so pass them through the constructor's
    `configOverwrite`.
- **Element Call / MatrixRTC provider (v1, the main one):**
  - Bundle Element Call's **embedded package** (built for widget mode,
    recommended by Element for embedding).
  - We implement the **widget host**: the matrix-widget-api postMessage
    protocol subset Element Call needs. That means capability
    negotiation; reading and sending the room's call membership state;
    sending and receiving to-device messages (encryption keys); delayed
    events (MSC4140, so a crashed client's membership expires); and
    io.element.join / hangup actions.
  - matrix-rust-sdk's `widget` module (`VirtualElementCallWidgetProperties`)
    is a good reference for the URL parameters and capabilities.
  - Discovery of LiveKit transports: try the client-server RTC transports
    endpoint first, then `.well-known` `rtc_foci` (legacy). Treat the
    exact paths as moving targets; check Element Call's `self_hosting.md`
    when implementing.
- **Later: native Element Call without a web view.** LiveKit's Go SDK
  (pion-based) can join a LiveKit room as a participant. With MatrixRTC
  signalling and key exchange done in Go, calls could run with no web view
  at all. Camera and microphone capture needs platform code, so this is
  research for after v1.1.
- **Measuring "janky":** keep local, private call stats (WebRTC `getStats`
  through the web view: round-trip time, packet loss, jitter, time to first
  media). Settings → Calls → "Last call" shows them, and they make it easy
  to compare the two providers on the same network. Nothing is sent
  anywhere.

## Phasing

| Version | Calls |
|---|---|
| **v1** | **MatrixRTC as the main system** (embedded Element Call, us as widget host). **Jitsi as the fallback:** used when there's no MatrixRTC transport, offered when MatrixRTC fails before anyone joins, and joined when a room already has a Jitsi call. "Move this call to Jitsi". The native control bar; camera off by default; an Audio only button; local call stats. |
| **v1.1** | The call system setting (MatrixRTC with fallback / Always Jitsi); the per-room override; automatic fallback if chosen. |
| **later** | Native Element Call (LiveKit Go SDK); background audio on phones; PiP polish. |
| **far future** | Activities in calls: Excalidraw whiteboard, Etherpad notes, as call-scoped widgets on the same widget host. See [ideas.md](ideas.md). |

## Open questions to verify before building

- Does Element X join Jitsi widgets at all (we believe not)? With
  MatrixRTC first this matters less, but a fallback Jitsi call may leave
  Element X users out, so the fallback prompt should say so.
- The current, stable discovery endpoint for MatrixRTC transports (the
  sources disagree; `rtc_foci` looks legacy).
- WebRTC in WebKitGTK on common distros (Fedora, Ubuntu, Arch).
- Is Jitsi E2EE possible in WKWebView? Safari has `RTCRtpScriptTransform`,
  but Jitsi documents E2EE as Chromium-only.

## Sources

- [Element Call README (embedded vs full package, widget mode)](https://raw.githubusercontent.com/element-hq/element-call/main/README.md)
- [Element Call self-hosting guide](https://github.com/element-hq/element-call/blob/main/docs/self_hosting.md)
- [lk-jwt-service (MatrixRTC authorization service)](https://github.com/element-hq/lk-jwt-service)
- [Continuwuity: MatrixRTC/Element Call setup](https://continuwuity.org/calls/livekit)
- [Tuwunel: Matrix RTC (Element Call)](https://matrix-construct.github.io/tuwunel/calls/matrix_rtc.html)
- [MatrixRTC aka Element Call setup (blog)](https://sspaeth.de/2024/11/sfu/)
- [Element Call issues](https://github.com/element-hq/element-call/issues)
- [Element Call #3656: End call needed](https://github.com/element-hq/element-call/issues/3656)
- [Element X Android #7979: incoming calls ring once](https://github.com/element-hq/element-x-android/issues/7979)
- [Element X Android issues](https://github.com/element-hq/element-x-android/issues)
- [Element: Classic to Pro mobile apps FAQ (Classic sunset, no interop)](https://docs.element.io/latest/element-server-suite-pro/migration/classic-to-pro-mobile-apps-faq/)
- [Element Web v1.12.24 (legacy calls toggle)](https://newreleases.io/project/github/element-hq/element-web/release/v1.12.24)
- [Introducing native Matrix VoIP with Element Call](https://element.io/blog/introducing-native-matrix-voip-with-element-call/)
- [E2EE voice and video for self-hosted community users](https://element.io/blog/end-to-end-encrypted-voice-and-video-for-self-hosted-community-users/)
- [Secure video conferencing for Matrix](https://element.io/blog/secure-video-conferencing-for-matrix/)
- [Element Call: redefining conferencing (Dec 2025)](https://element.io/blog/element-call-redefining-conferencing-for-privacy-scale-and-sovereignty/)
- [matrix-js-sdk RTCEncryptionManager](https://cdn.jsdelivr.net/npm/matrix-js-sdk@41.3.0/src/matrixrtc/RTCEncryptionManager.ts)
- [MatrixRTC FOSDEM 2024 slides](https://archive.fosdem.org/2024/events/attachments/fosdem-2024-2876-matrixrtc-the-future-of-matrix-calls/slides/22881/MatrixRTC_FOSDEM24_NcGvvs5.pdf)
- [matrix-rust-sdk widget module](https://matrix-org.github.io/matrix-rust-sdk/matrix_sdk/widget/index.html)
- [VirtualElementCallWidgetProperties](https://matrix-org.github.io/matrix-rust-sdk/matrix_sdk/widget/struct.VirtualElementCallWidgetProperties.html)
- [ESS: Setting up Element Call](https://ems-docs.element.io/books/element-server-suite-classic-documentation-lts-2410/page/setting-up-element-call/export/html)
- [Jitsi IFrame API](https://jitsi.org/iframe)
- [Jitsi iframe API functions](https://jitsi.github.io/handbook/docs/dev-guide/dev-guide-iframe-functions)
- [Jitsi configuration (startAudioOnly etc.)](https://jitsi.github.io/handbook/docs/dev-guide/dev-guide-configuration)
- [Start-muted workaround gist (jitsi-meet#8144)](https://Gist.github.com/cketti/f0ed9b722d04618b33a7269d030e1072)
- [Element vs Jitsi (informal comparison)](https://ossalt.com/compare/element-vs-jitsi)
- [Jitsi vs Matrix (informal comparison)](https://jitsi-guide.pages.dev/blog/jitsi-vs-matrix)
