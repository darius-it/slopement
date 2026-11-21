# slopement design language

Minimal, quiet, content first. Neutral grays carry the interface; one light
purple accent marks what matters. The tokens live in `kit/tokens.go`. Every
screen uses them, and `kit/contrast_test.go` keeps every text pair at WCAG AA.

## Color

| Token | Light | Dark | Use |
|---|---|---|---|
| Bg | `#FCFCFD` | `#121116` | timeline, main content |
| BgSubtle | `#F5F5F8` | `#18171D` | rail, room list, sidebars |
| BgInset | `#F1F0F5` | `#1E1D24` | fields, composer, quotes |
| Raised | `#FFFFFF` | `#1F1E25` | menus, popovers, sheets, dialogs |
| Border | `#E8E7EE` | `#2A2931` | hairlines |
| Text / Muted / Faint | `#18171D` / `#686676` / `#8E8C9B` | `#ECEBF1` / `#9D9BA9` / `#6B6977` | |
| **Accent** | `#835FCE` | `#BEA8F9` | primary action, mentions, focus |
| AccentSubtle | `#F1EDFF` | `#28223A` | selected row, mention tint |
| AccentStrong | `#6E47B7` | `#C4AFFD` | purple text on surfaces |
| Danger / Warning / Success | `#C92F33` / `#B77610` / `#258651` | `#ED756E` / `#E4AC59` / `#66C189` | |

The accent is OKLCH hue 296 (lavender). On light it sits at L 0.575 so white
text on it passes AA. On dark it is a pale lavender at L 0.78 with dark text.

**Username colours:** 8 hues (red, orange, olive, green, teal, blue, purple,
pink) at one OKLCH lightness per theme (0.52 light, 0.78 dark). Every hue
passes AA on every surface. Users may pick their own colour; others can be
overridden locally (`kit.UserColors`).

## Type

The system UI font (SF on Apple, Segoe on Windows, Cantarell/Inter on Linux),
so there is no embedded font to load. Weights are 400 / 500 / 600 only.

Display 28 · Heading 22 · Title 18 · Label 15 · Body 14 (16 on touch) ·
Small 13 · Caption 11. Body line-height 1.45. Times and counts use tabular
figures.

## Space, shape, elevation

- 4 DIP grid: 2 4 6 8 12 16 20 24 32.
- Radius: 6 controls · 10 cards, menus, popovers · 14 sheets, dialogs · full
  for avatars, chips, badges. Spaces are rounded squares (28% radius).
- Hairline borders. Only floating layers (menus, popovers, dialogs, sheets,
  the switcher) cast a shadow, one soft layer plus a 1px contact shadow.
- Rail 64, room list 280, room row 52 (64 touch), touch targets ≥ 44.
- Below 720 DIP wide the phone layout is used.

## Components (kit)

Button (primary / secondary / ghost / danger), IconButton, RoundButton ·
TextField, SearchField, SearchButton, Kbd · Chip, TokenChip, Tabs · Avatar
(image / initials, presence, trust shield), AvatarEditor · Count, Dot, Pill
· RoomRow, RailItem, SectionHeader · Message, ReactionPill, DateSeparator,
SystemEvent, TypingLine · Composer · Picker (Emoji / GIFs / Stickers / Clips
/ Memes, via KLIPY) · RoomMenu + MuteList · Sheet, Dialog, Adaptive ·
Switcher (⌘K) · VerifyStart, VerifyEmoji, SASGrid · CallBar, CallBanner ·
SwatchPicker.

Gotcha: `ui.Element.Selected(true)` paints the accent fill itself. Kit
components use `AccessSelected` for the a11y state and draw selection
themselves (AccentSubtle).

## Motion

Following Emil Kowalski's rules (`kit/motion.go`):

- Animate only to explain a change. **Never** animate keyboard-driven,
  frequent UI: the ⌘K switcher, switching rooms, arrowing through lists.
- Enter ease-out (`cubic-bezier(.23,1,.32,1)`), exit faster: menus 150 ms
  in / 100 ms out, from 0.97 scale and 4 DIPs toward the trigger. Dialogs
  stay centered.
- Buttons press to 0.97. Rows change color (120 ms) instead.
- Sheets move like iOS's Control Center, always on interruptible springs:
  - Fly in: 300 ms, bounce 0.22. Snap back after a drag: 340 ms, bounce
    0.28. Shut: 240 ms, no bounce (exits are faster).
  - The whole sheet drags, 1:1 under the finger. Past the open position it
    rubber-bands (UIScrollView's curve, c = 0.55), never a wall.
  - A sheet caught mid-flight stays under the finger from where it was.
    Closing mid-open, or reopening mid-close, reverses with its velocity.
  - On release, it projects where it would come to rest (position + 0.3 s
    of velocity). Past half its height it closes, however slowly it was
    dragged. Otherwise it springs back, carrying the fling's velocity. A
    flick down closes from anywhere; one up keeps it open.
- Revealing a spoiler (text or media) scatters it as fine dust: grains in
  the veil's colour drift up and out and fade over 700 ms (`kit/dust.go`).
  Skipped under Reduce Motion; it's a rare, playful moment, so it's allowed.
- Thought bubbles open with a short spring (280 ms, bounce 0.12), growing
  from their trail.
- Live messages rise 8 DIPs (180 ms). History and first loads never animate.
- The space-rail indicator grows from an unread pip (8) to a selected bar (24).
- Reduce Motion: mygo drops movement and keeps fades.

## Phone chat (bubbles)

On phones, the conversation is bubbles on a soft wallpaper, like KakaoTalk,
LINE and Signal, with our purple supplying the personality:

| Token | Light | Dark |
|---|---|---|
| Wallpaper | lavender L .955 | L .16 |
| BubbleIn | white | L .245 |
| BubbleOut | light purple L .86 (dark text) | purple L .40 (light text) |

- The first bubble of a run has a 4 DIP "tail" corner toward its sender;
  the others are 18. Gaps are 2 DIPs inside a run and 12 between runs.
- The avatar and name sit over an incoming run. Time and **Read** sit
  *beside* the bubble, at its foot, on the run's last message only.
- Reactions overlap the bubble's bottom edge, ringed in the wallpaper's
  colour.
- Stickers are shown without a bubble. Albums fill the bubble edge to edge.
- Live messages pop in with a small spring from the sender's side (320 ms,
  bounce 0.18). History never animates.
- Composer: `+`, then a white pill holding the text and the smiley, then a
  mic that turns into a purple send button as you type (a scale and
  rotate swap).
- Desktop keeps the flat, dense timeline.

## Platform integration

The window chrome and the menu bar are the platform's own, and the content
is ours (`platform/`, `kit/native.go`). Right-click and ⋯ menus are the
kit's drawn ones on every platform (decided 2026-11-21: native context
menus clashed with the look; `kit.NativeMenus` stays false):

| | macOS | Windows 11 | Linux | Phones |
|---|---|---|---|---|
| Title bar | hidden; traffic lights over the sidebar | hidden; native caption buttons over our toolbar | hidden; GTK buttons styled like libadwaita (round, flat) | system bars |
| Sidebar | system sidebar material (vibrancy) | Mica | our BgSubtle | — |
| Menu bar | the real one: App (About, Settings… ⌘,, Services, Hide, Quit), File, Edit (roles), View (⌘K, Appearance, zoom, full screen), Room (Mute ▸, Favourite ⌘D, Leave…), Window, Help | hidden until Alt / F10 (Settings and Quit under File) | hidden until Alt / F10 | — |
| Context and ⋯ menus | the kit's drawn menus | the kit's drawn menus | the kit's drawn menus | the kit's drawn menus and sheets |
| File dialogs | native | native | native (portal) | native |

- The content area always keeps the kit's opaque `Bg`, accent, type,
  bubbles, cards and motion: that's what keeps it recognisably ours.
- The material shows only while the theme follows the system (it tracks
  the system's appearance). A forced Light or Dark gets an opaque sidebar.
- Toolbars and empty sidebar space drag the window, and double-clicking
  them zooms it, as native title bars do.
- Shortcuts use ⌘ on macOS and Ctrl elsewhere (`ui.Cmd`), and the menus
  show them.
- The system UI font: SF, Segoe UI Variable, or the desktop's font.
- Phones (shell step): `mygo.NewTabs` for the system tab bar, and a router
  with `SystemStack` for native push/pop and the edge swipe.

## Dialogs & confirmations

- `kit.Dialog`: 400 wide, padding 20, gap 20. A 16 px semibold title that
  takes the width. A 28 px close button hugging the top-right corner
  (12 px in), outside the content's padding.
- `kit.Confirm` (after Relume, Grok and Cloudflare on desktop, and Bluesky
  and Revolut on phones, via Mobbin):
  - Desktop: 380 wide; title and body 6 px apart; Cancel, then the action,
    at the bottom right. Enter takes Cancel when the action is destructive.
  - Phones: a bottom sheet with full-width buttons, 48 tall, stacked: the
    action on top and Cancel below, with no ×.
  - Escape, the scrim and the × all count as Cancel.

## Formatting & media

- Links are **coloured, not underlined**; they underline on hover
  (`ui.Link`'s default). AccentStrong on surfaces; in outgoing bubbles
  `LinkOnBubble` (a deeper purple on light, a pale lavender on dark), at AA
  against the bubble, so they stand apart from its text.
- Spoilers are a muted veil until clicked. Code uses an inset monospace
  chip; blocks get a card with the language and a Copy button. Quotes have
  a 3 DIP bar. Inside an outgoing bubble, links, code and quotes are
  recoloured to read on purple.
- Media keeps its aspect ratio from the first frame. Videos get a play disc
  and a duration pill, GIFs a small "GIF" pill. Spoiler media is blurred
  (sigma 12) until clicked. Uploads show a progress ring.
- A message that is **only** a link to a GIF, image or video (KLIPY, Tenor,
  Giphy, Imgur, a direct file) shows as that media alone; the link text is
  hidden (`embeds.SoleMedia`). With other words around it, it stays a link.
- GIFs show at their **own size and shape**, never cropped or enlarged,
  shrunk only to fit the timeline's width. On hover a star at their
  top-right corner adds them to the picker's favorite GIFs (filled in the
  Warning yellow once starred, as in the picker).
- The media viewer closes on Escape, its ✕, or a click anywhere around the
  picture.
- Window glass (Settings → Appearance): **Tinted** (default) lays the
  sidebar colour at 78% (dark) / 66% (light) over the system material, so
  contrast stays steady over any wallpaper; **Clear** shows the material as
  it is; **Off** is opaque.
- Link and video cards: **no coloured marks**, neither a side bar nor a
  brand rule. The site name over the title says where a card is from
  (option B, chosen 2026-11-21). The card's radius is 8 and its picture's 6,
  tighter than chat media, so a card reads as a quoted document. Then the
  title as a link and the description; the thumbnail sits at the side, or
  below the words when it's large.

## Navigation layouts

- Words: **Chats**, **Communities**, **rooms**. People are never listed
  inside a community.
- The Rail's first item is a Chats bubble (purple when selected) with the
  mention count. Below a divider come the communities, then the dashed `+`.
  A community's list starts with its header card.
- Tabs: the kit's sliding Tabs switch at the top of the list (desktop), or
  the system tab bar (phone). Communities on phones are cards showing their
  first two rooms and "See all N rooms".
- One inbox: filter chips on top; a community is a row with a chevron that
  unfolds its rooms, indented to the text column (collapse transition).
- The Settings picker: a card per layout with a box diagram of it; the
  platform's default carries a "Default" pill; the chosen card is ringed in
  the accent.

## Polls, status, profile

- Poll card: a Raised card with a hairline. Answers are inset rows (40 tall)
  with a radio (one choice) or check box (several). Results are a bar
  behind each row that grows in over 320 ms ease-out: the leader in the
  accent at 22%, the others in text colour at 6%. Your vote gets a purple
  hairline. The footer shows the counts, with tabular digits. "N votes" is
  a quiet link once results show: muted at rest, the text colour (120 ms)
  and underlined on hover; it opens who voted (a 300 DIP popover or a
  sheet): answer and count, then 20 DIP avatars with names in their
  colours.
- Menu items carry 16 DIP Lucide icons in the menu's text colour (template
  icons via `kit.MenuIcon`): the composer's + has a paperclip for Upload a
  file and a bar chart for Create poll.
- Status: always one quiet line, "emoji text · until 17:00", in muted text.
  Chats show it in every layout, so People is an optional extra: a 20 DIP
  thought bubble with the emoji at the avatar's top-right corner. Hovering
  **the bubble itself** opens it to the right, over the row, to the whole
  status; the preview line always stays the last message. Only the cloud's
  width moves (ease-out, 200 ms open, 120 ms close, no overshoot): the emoji
  stays put in a fixed slot and the text, always laid out, is clipped and
  faded, so nothing inside shifts or goes back and forth.
  A DM header shows it under the name. In the profile card, a soft inset
  bubble.
- **When a status clears is private.** "· until 17:00" shows only on your
  own status (the editor and your own line); everyone else sees just the
  emoji and text.
- People on desktop: a grid of 168 DIP tiles (avatar 64, name, Online /
  Away / last seen). A status floats over the avatar as a comic thought
  bubble (a Raised pill and a trail of three shrinking dots): the emoji at
  rest, opening to the right, over the neighbours, to the whole status on
  hover. Message appears at the tile's corner on hover. Phones keep the list.
- Status editor: the preview is one fixed line inside the editor (nothing
  shifts as you type); the emoji button opens the full emoji picker
  (search, categories), no suggested presets.
- Names and avatars in the timeline (and above incoming bubbles) are links
  to the person's card: pointer cursor, underline on hover, the card in a
  popover under the name. Clicking a person in People opens the card as a
  dialog (a sheet on phones).
- Favourites: starred chats lead the Chats list under a small ☆ Favourites
  header in every layout (Rail, Tabs, One inbox); star and unstar from the
  chat's right-click menu.
- GIF picker → Favorites: "Import from Discord" reads a JSON from
  [discord-gif-exporter](https://github.com/darius-it/discord-gif-exporter)
  (Tenor/Giphy MP4s become their GIFs), newest first, skipping duplicates.
- Rail +: a dashed 44 tile under the spaces, filling in with the accent on
  hover.

## References (Mobbin)

- Polls: [WhatsApp create](https://mobbin.com/screens/b8955f9d-1adc-440e-be93-273c66d13306), [Discord duration](https://mobbin.com/screens/9d7bfe95-b746-4008-9eaf-10a92f201f72), [LinkedIn](https://mobbin.com/screens/34eca806-f9ba-4b4a-9371-0e4febf090fe), [Discord in chat](https://mobbin.com/screens/2adbcb98-7214-4e69-8100-5347ac24baf3), [WhatsApp in chat](https://mobbin.com/screens/7cb91cb0-89b4-4299-8955-dad4d3feca27)
- Status: [Discord](https://mobbin.com/screens/69d20cc9-bd41-40db-9882-31e678f7d1ea), [Campsite](https://mobbin.com/screens/ad1431c1-dfa0-4550-adba-3d9be21c35a6), [ClickUp](https://mobbin.com/screens/7339030f-2aba-4f8a-b67f-0f076f9e0ebe)

- Phone chat: [Signal](https://mobbin.com/screens/a3af7969-157a-41b7-a553-bca6e767c9b0), [Signal with a wallpaper](https://mobbin.com/screens/0ad7f30d-b41f-4e34-a4a7-e07438fdbe98), [LINE](https://mobbin.com/screens/10c94da2-93ad-4923-96b0-5d49c247a257), [KakaoTalk](https://mobbin.com/screens/4bc5eabf-0b02-44fc-8617-4cb1200b9070), [KakaoTalk sticker sheet](https://mobbin.com/screens/727b1ebe-5a41-4fe1-bda8-6eacbdcb45df)

- Desktop 3-pane + rail: [Discord](https://mobbin.com/screens/b7b4fcc2-1a55-436e-911b-232f0df05df3), [Slack](https://mobbin.com/screens/de9d3a84-c9d5-41ff-88a2-9e78c96d0969), [hilos](https://mobbin.com/screens/29033e86-48e7-4549-bd13-72819aba1675), [Threads DMs](https://mobbin.com/screens/6344257e-afc6-4bc3-868e-4b78592d25b8)
- Phone chat list: [WhatsApp](https://mobbin.com/screens/14fcc4ed-fecf-4f61-b99a-2b2b76b7332b), [Behance](https://mobbin.com/screens/e293f8e5-cc43-4616-8574-37b65251f8f6)
- Mute durations: [Discord](https://mobbin.com/screens/4461d5d6-e667-4d90-bbad-b00315386583), [WhatsApp](https://mobbin.com/screens/e05de48f-29dc-429f-8cf6-7765c375717d), [OpenPhone](https://mobbin.com/screens/1d7fc0d7-feeb-4043-b8e4-723343942c06)
- Room context menu: [Slack](https://mobbin.com/screens/3f11b305-42f6-4e72-beaa-38be7b86fcc3), [Slite](https://mobbin.com/screens/ee47887f-d41a-4c57-8c9e-9b22d532a000), [Dropbox Dash](https://mobbin.com/screens/ee4a3806-4b05-469c-aab1-6555cfab2347)
- Quick switcher: [Devin](https://mobbin.com/screens/e37a4e69-9a61-4f36-803d-edbd89c1229d), [StackAI](https://mobbin.com/screens/bbcc94bb-f535-4dca-8532-56b135fce5c3), [Claude](https://mobbin.com/screens/34fc112f-9de9-44ed-9f79-0808c005fca0)
- GIF picker: [Discord](https://mobbin.com/screens/a5cbf61d-26ad-436c-b99d-8884c994d1c4), [Tinder](https://mobbin.com/screens/ba3bb9b2-614c-4634-8c6c-66e6ccdf8a1e)
- Search filters: [Front](https://mobbin.com/screens/de479d33-58b7-4b13-b96e-7be6520dcf9e), [Upwork](https://mobbin.com/screens/290953d1-c5fa-44d4-abc8-a3e677deb2ae)
- Profile picture: [Remote](https://mobbin.com/screens/d0d7fd26-3f17-4a3e-a4cc-18fc55620984), [CREME](https://mobbin.com/screens/0ea9ec59-7840-4b05-85a9-2a778798a1b0)
- Conversation: [Feeld](https://mobbin.com/screens/1092b7c8-61ac-449d-971a-869e9d107ec0), [Badoo](https://mobbin.com/screens/1c10debf-44e2-499d-a408-c21c41ef50b4)
- Login (centered single-column sign-ins, one primary action per step, errors as one quiet line): [Nutmeg](https://mobbin.com/screens/2d0e03fb-6e8e-4232-a4c4-6cd1a0b06c0d), [Centr](https://mobbin.com/screens/bf438ccb-6402-47bf-8ef2-e3b9c6ff2ab7), [Tana](https://mobbin.com/screens/1fa0617c-01bc-4d19-91d7-cce7dd7ba445), [Plain](https://mobbin.com/screens/b9ff7441-defd-4155-a73e-ec088c14eff3); the collection: [slopement login references](https://mobbin.com/collections/0c4f397e-8995-412d-9e85-b9369d420b35)
