package matrix

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// Custom emoji and sticker packs, as MSC2545 has them (Cinny, FluffyChat and
// Nheko use it): the user's own are account data im.ponies.user_emotes, a
// room's are state im.ponies.room_emotes (one pack per state key), and the
// room packs the user has turned on are account data im.ponies.emote_rooms
// ({"rooms": {roomID: {stateKey: {}}}}). Each is
//
//	{"pack": {"display_name", "avatar_url", "usage": ["emoticon","sticker"]},
//	 "images": {shortcode: {"url", "body", "info": {…}, "usage": […]}}}

const (
	userEmotesData = "im.ponies.user_emotes"
	emoteRoomsData = "im.ponies.emote_rooms"
	roomEmotesType = "im.ponies.room_emotes"
)

var (
	userEmotesType = event.Type{Type: userEmotesData, Class: event.AccountDataEventType}
	emoteRoomsType = event.Type{Type: emoteRoomsData, Class: event.AccountDataEventType}
	roomPackType   = event.Type{Type: roomEmotesType, Class: event.StateEventType}
)

// packState is what is known of the packs. The UI thread only.
type packState struct {
	user      map[string]any            // the user's pack as the account has it, kept whole to edit
	userAsked bool                      // the account's packs were asked for
	enabled   map[string]map[string]any // room → pack keys the user turned on (the rows are {})
	rooms     map[string]map[string]model.EmojiPack
	asked     map[string]bool // rooms whose packs were asked for
}

type packBody struct {
	Pack struct {
		Name   string   `json:"display_name"`
		Avatar string   `json:"avatar_url"`
		Usage  []string `json:"usage"`
	} `json:"pack"`
	Images map[string]struct {
		URL  string `json:"url"`
		Body string `json:"body"`
		Info struct {
			W    int    `json:"w"`
			H    int    `json:"h"`
			Mime string `json:"mimetype"`
		} `json:"info"`
		Usage []string `json:"usage"`
	} `json:"images"`
}

// parsePack reads a pack. An image's own usage wins over the pack's; with
// neither it is both an emoji and a sticker. Images with no mxc url are
// skipped. Pure, for the tests.
func parsePack(packID, fallbackName, room string, raw []byte) (model.EmojiPack, bool) {
	var b packBody
	if json.Unmarshal(raw, &b) != nil || len(b.Images) == 0 {
		return model.EmojiPack{}, false
	}
	p := model.EmojiPack{ID: packID, Name: b.Pack.Name, Avatar: b.Pack.Avatar, Room: room}
	if p.Name == "" {
		p.Name = fallbackName
	}
	use := func(usage []string) (emoticon, sticker bool) {
		if len(usage) == 0 {
			return true, true
		}
		for _, u := range usage {
			switch u {
			case "emoticon":
				emoticon = true
			case "sticker":
				sticker = true
			}
		}
		return
	}
	packEmoticon, packSticker := use(b.Pack.Usage)
	for code, im := range b.Images {
		if !strings.HasPrefix(im.URL, "mxc://") || code == "" {
			continue
		}
		e := model.PackEmoji{Shortcode: code, Body: im.Body, URL: im.URL, Mime: im.Info.Mime, W: im.Info.W, H: im.Info.H,
			Emoticon: packEmoticon, Sticker: packSticker}
		if len(im.Usage) > 0 {
			e.Emoticon, e.Sticker = use(im.Usage)
		}
		if e.Body == "" {
			e.Body = code
		}
		if e.Emoticon || e.Sticker {
			p.Emoji = append(p.Emoji, e)
		}
	}
	sort.Slice(p.Emoji, func(i, j int) bool { return p.Emoji[i].Shortcode < p.Emoji[j].Shortcode })
	return p, len(p.Emoji) > 0
}

func rawOf(c *event.Content) []byte {
	if len(c.VeryRaw) > 0 {
		return c.VeryRaw
	}
	b, _ := json.Marshal(c.Raw)
	return b
}

// onUserEmotes is the user's own pack changing.
func (s *Source) onUserEmotes(ctx context.Context, evt *event.Event) {
	var m map[string]any
	if json.Unmarshal(rawOf(&evt.Content), &m) != nil {
		return
	}
	s.post(func() { s.packs.user = m })
}

// onEmoteRooms is the set of room packs turned on changing.
func (s *Source) onEmoteRooms(ctx context.Context, evt *event.Event) {
	enabled := parseEnabled(rawOf(&evt.Content))
	s.post(func() {
		s.packs.enabled = enabled
		s.packs.asked = nil // a pack newly turned on is asked for next time
	})
}

func parseEnabled(raw []byte) map[string]map[string]any {
	var c struct {
		Rooms map[string]map[string]any `json:"rooms"`
	}
	if json.Unmarshal(raw, &c) != nil {
		return nil
	}
	return c.Rooms
}

// onRoomEmotes is a room's pack changing (state, one per key).
func (s *Source) onRoomEmotes(ctx context.Context, evt *event.Event) {
	if evt.StateKey == nil {
		return
	}
	room, key, raw := evt.RoomID.String(), *evt.StateKey, rawOf(&evt.Content)
	s.post(func() { s.setRoomPack(room, key, raw) })
}

func (s *Source) setRoomPack(room, key string, raw []byte) {
	if s.packs.rooms == nil {
		s.packs.rooms = map[string]map[string]model.EmojiPack{}
	}
	p, ok := parsePack(room+"|"+key, s.roomLabel(room), room, raw)
	if !ok {
		delete(s.packs.rooms[room], key) // emptied: gone
		return
	}
	if s.packs.rooms[room] == nil {
		s.packs.rooms[room] = map[string]model.EmojiPack{}
	}
	s.packs.rooms[room][key] = p
}

// roomLabel is what a room's pack with no name is called: the room's.
func (s *Source) roomLabel(room string) string {
	if r, ok := s.rooms[room]; ok && r.name != "" {
		return r.name
	}
	return "Room emoji"
}

// EmojiPacks are the packs on offer in room: the user's own, then the room's,
// then those turned on in other rooms. What has not been asked for yet is
// asked for now, in the background, and shows when it arrives.
func (s *Source) EmojiPacks(room string) []model.EmojiPack {
	s.askPacks(room)
	var out []model.EmojiPack
	if p, ok := parsePackMap(s.packs.user); ok {
		p.ID, p.Name, p.Mine = "user", orDefault(p.Name, "My emoji"), true
		out = append(out, p)
	}
	seen := map[string]bool{}
	add := func(rm string, only map[string]any) {
		keys := make([]string, 0, len(s.packs.rooms[rm]))
		for k := range s.packs.rooms[rm] {
			if only == nil {
				keys = append(keys, k)
			} else if _, on := only[k]; on {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			if p := s.packs.rooms[rm][k]; !seen[p.ID] {
				seen[p.ID] = true
				out = append(out, p)
			}
		}
	}
	add(room, nil)
	rooms := make([]string, 0, len(s.packs.enabled))
	for rm := range s.packs.enabled {
		rooms = append(rooms, rm)
	}
	sort.Strings(rooms)
	for _, rm := range rooms {
		add(rm, s.packs.enabled[rm])
	}
	return out
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func parsePackMap(m map[string]any) (model.EmojiPack, bool) {
	if m == nil {
		return model.EmojiPack{}, false
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return model.EmojiPack{}, false
	}
	return parsePack("user", "", "", raw)
}

// askPacks fetches what has not been fetched: the account's packs once, and
// each room's (the shown one, and those turned on) when first wanted.
func (s *Source) askPacks(room string) {
	if s.cli == nil {
		return
	}
	if !s.packs.userAsked {
		s.packs.userAsked = true
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var user map[string]any
			var rooms map[string]map[string]any
			if s.cli.GetAccountData(ctx, userEmotesData, &user) != nil {
				user = nil
			}
			var er struct {
				Rooms map[string]map[string]any `json:"rooms"`
			}
			if s.cli.GetAccountData(ctx, emoteRoomsData, &er) == nil {
				rooms = er.Rooms
			}
			s.post(func() {
				if s.packs.user == nil {
					s.packs.user = user
				}
				if s.packs.enabled == nil {
					s.packs.enabled = rooms
				}
				s.packs.asked = nil
				s.changed()
			})
		}()
	}
	want := []string{room}
	for rm := range s.packs.enabled {
		want = append(want, rm)
	}
	for _, rm := range want {
		if rm == "" || s.packs.asked[rm] {
			continue
		}
		if s.packs.asked == nil {
			s.packs.asked = map[string]bool{}
		}
		s.packs.asked[rm] = true
		go s.fetchRoomPacks(rm)
	}
}

// fetchRoomPacks reads the room's pack events from its state.
func (s *Source) fetchRoomPacks(room string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	state, err := s.cli.State(ctx, id.RoomID(room))
	if err != nil {
		s.post(func() { delete(s.packs.asked, room) }) // asked again next time
		return
	}
	type row struct {
		key string
		raw []byte
	}
	var rows []row
	for key, evt := range state[roomPackType] {
		rows = append(rows, row{key, rawOf(&evt.Content)})
	}
	s.post(func() {
		for _, r := range rows {
			s.setRoomPack(room, r.key, r.raw)
		}
		s.changed()
	})
}

// shortcodeRE is a ":name:" in a message that may be a custom emoji.
var shortcodeRE = regexp.MustCompile(`:([A-Za-z0-9_+\-]{1,64}):`)

// withCustomEmoji turns ":name:" into the image of the emoji of that name in
// the packs on offer in room, in an HTML body: <img data-mx-emoticon …>, as
// other clients send. Names that are not in a pack stay as typed. Returns
// html unchanged when none matched.
func (s *Source) withCustomEmoji(room, html string) string {
	if !strings.Contains(html, ":") {
		return html
	}
	var by map[string]model.PackEmoji
	for _, p := range s.EmojiPacks(room) {
		for _, e := range p.Emoji {
			if e.Emoticon {
				if by == nil {
					by = map[string]model.PackEmoji{}
				}
				if _, dup := by[e.Shortcode]; !dup {
					by[e.Shortcode] = e
				}
			}
		}
	}
	if by == nil {
		return html
	}
	return replaceOutsideTags(html, func(text string) string {
		return shortcodeRE.ReplaceAllStringFunc(text, func(m string) string {
			e, ok := by[m[1:len(m)-1]]
			if !ok {
				return m
			}
			return `<img data-mx-emoticon src="` + e.URL + `" alt=":` + e.Shortcode + `:" title=":` + e.Shortcode + `:" height="32" />`
		})
	})
}

// replaceOutsideTags applies fn to the text between a body's tags, not to
// code (inside <code> or <pre>) or to attributes.
func replaceOutsideTags(html string, fn func(string) string) string {
	var out strings.Builder
	code := 0
	for i := 0; i < len(html); {
		if html[i] == '<' {
			j := strings.IndexByte(html[i:], '>')
			if j < 0 {
				out.WriteString(html[i:])
				break
			}
			tag := strings.ToLower(html[i : i+j+1])
			switch {
			case strings.HasPrefix(tag, "<code"), strings.HasPrefix(tag, "<pre"):
				code++
			case strings.HasPrefix(tag, "</code"), strings.HasPrefix(tag, "</pre"):
				if code > 0 {
					code--
				}
			}
			out.WriteString(html[i : i+j+1])
			i += j + 1
			continue
		}
		j := strings.IndexByte(html[i:], '<')
		if j < 0 {
			j = len(html) - i
		}
		if code == 0 {
			out.WriteString(fn(html[i : i+j]))
		} else {
			out.WriteString(html[i : i+j])
		}
		i += j
	}
	return out.String()
}

// AddToPack puts an image in the user's own pack (creating it): the
// "Add to my pack" of an emoji or sticker seen anywhere. A shortcode already
// taken by a different picture gets a number after it. The pack shows at once
// and is saved to the account.
func (s *Source) AddToPack(e model.PackEmoji, done func(problem string)) {
	if s.cli == nil || !strings.HasPrefix(e.URL, "mxc://") {
		done("That picture can't be added.")
		return
	}
	user := map[string]any{}
	for k, v := range s.packs.user {
		user[k] = v
	}
	images, _ := user["images"].(map[string]any)
	next := map[string]any{}
	for k, v := range images {
		next[k] = v
		if im, _ := v.(map[string]any); im != nil && im["url"] == e.URL {
			done("") // the same picture is already in the pack
			return
		}
	}
	code := strings.Trim(strings.ToLower(strings.Join(strings.Fields(e.Shortcode), "_")), ":")
	if code == "" {
		code = "emoji"
	}
	for n := 2; ; n++ {
		if _, taken := next[code]; !taken {
			break
		}
		code = strings.TrimRight(code, "0123456789_") + "_" + strconv.Itoa(n)
	}
	img := map[string]any{"url": e.URL, "body": orDefault(e.Body, code)}
	info := map[string]any{}
	if e.Mime != "" {
		info["mimetype"] = e.Mime
	}
	if e.W > 0 && e.H > 0 {
		info["w"], info["h"] = e.W, e.H
	}
	if len(info) > 0 {
		img["info"] = info
	}
	next[code] = img
	user["images"] = next
	if _, ok := user["pack"]; !ok {
		user["pack"] = map[string]any{"display_name": "My emoji"}
	}
	s.packs.user = user
	s.changed()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := s.cli.SetAccountData(ctx, userEmotesData, user)
		s.post(func() {
			if err != nil {
				done("Could not save your emoji: " + plain(err))
				return
			}
			done("")
		})
	}()
}

var _ model.Packs = (*Source)(nil)
