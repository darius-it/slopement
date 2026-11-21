# Ideas for the far future

Not planned, not scheduled: things worth keeping in mind so today's design
doesn't rule them out. Each comes with a first technical sketch.

## Activities in calls (after Discord Activities)

Shared things that call participants open together, beside the video:

- **Whiteboard** (Excalidraw).
- **Shared notes** (Etherpad).
- Later perhaps: a watch-together player, small games, a shared poll or
  timer.

### How it could work

- **An activity is a Matrix widget scoped to the call.** It lives as room
  state (`im.vector.modular.widgets` or the newer `m.widget`), plus a
  marker linking it to the call session (`call_id`), so it opens for
  everyone in the call and closes when the call ends. Other clients see an
  ordinary widget they can open too.
- **Opening one:** the call's control bar gets an **Activities** button
  (shapes icon) with a small gallery. The first person to open one
  creates the widget. Others see "Mira opened Whiteboard · Join" in the
  call, and a pill in the room's header.
- **Layout:** desktop puts the activity big with the video as a strip;
  phones go full screen with the video as a floating PiP. The same web view
  machinery as the call itself, with us as the **widget API host**: the
  Element Call work in v1.1 (see [calls-research.md](calls-research.md))
  builds most of this.
- **Excalidraw:**
  - Its collaboration mode keeps the drawing end-to-end encrypted with a
    key in the room link's `#fragment`, so the relay server never sees
    the content.
  - We'd put that key in the widget's **encrypted** room event, so only
    members can open it.
  - The relay is `excalidraw-room`, self-hostable, or excalidraw.com's.
  - "Save to the room" exports a `.excalidraw` file plus a PNG as a normal
    message when the call ends.
- **Etherpad:**
  - Self-hosted Etherpad behind the widget, one pad per activity.
  - Etherpad isn't end-to-end encrypted, so the server sees the text. Say
    so in the gallery, and prefer the room's own configured instance.
  - "Save to the room" posts the final text as a Markdown file and message.
- **Matrix-native alternative (no extra servers):** sync a CRDT (Yjs or
  Automerge) over Matrix events. Updates would go as to-device messages
  between participants during the call, or as room events for persistence.
  That gives real Matrix E2EE and works on any homeserver.
  - Something like the matrix-crdt experiments, but for a native
    whiteboard and notes drawn by mygo instead of a web page.
  - This is the most "us" option: low-resource and native, though far
    more work.
- **Rules:**
  - Activities never auto-open for someone who didn't click them.
  - They respect Reduce Motion, have no sound unless the activity's
    purpose is sound, and keep idle frames at zero when nothing changes.
- **Permissions:** creating a widget needs the power level for widget
  state. In rooms where members lack it, fall back to a personal activity
  link shared in the call chat.

## Smaller far-future ideas
- Collect more here as they come up.
