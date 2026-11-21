// Package embeds decides how a link in a message is previewed: YouTube
// videos as a player card, Imgur posts and direct media as the media
// itself, other pages as an OpenGraph card (fetched by the homeserver's
// preview_url, in package matrix).
package embeds

import (
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"slopement/format"
)

// Kind is how a link previews.
type Kind int

const (
	Generic Kind = iota
	YouTube
	Imgur
	Image // a direct image or GIF
	Video // a direct video
)

// Embed is a link classified.
type Embed struct {
	URL  string
	Kind Kind
	// YouTube.
	VideoID string
	Shorts  bool
	Start   int // seconds
	// Imgur: the media to show (an image, or an MP4 for gifv), and
	// whether the link is an album or gallery (showing its first item).
	MediaURL string
	Album    bool
	ImgurID  string
}

var (
	ytID      = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	imgurID   = regexp.MustCompile(`^[A-Za-z0-9]{5,10}$`)
	imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".avif": true}
	videoExts = map[string]bool{".mp4": true, ".webm": true, ".mov": true, ".gifv": true}
)

// Classify classifies a link.
func Classify(raw string) Embed {
	e := Embed{URL: raw}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return e
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	host = strings.TrimPrefix(host, "m.")
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch host {
	case "youtube.com", "music.youtube.com", "youtube-nocookie.com":
		id := ""
		switch {
		case u.Path == "/watch":
			id = u.Query().Get("v")
		case len(segs) >= 2 && (segs[0] == "shorts" || segs[0] == "embed" || segs[0] == "live" || segs[0] == "v"):
			id, e.Shorts = segs[1], segs[0] == "shorts"
		}
		if ytID.MatchString(id) {
			e.Kind, e.VideoID, e.Start = YouTube, id, startOf(u)
			return e
		}
	case "youtu.be":
		if len(segs) >= 1 && ytID.MatchString(segs[0]) {
			e.Kind, e.VideoID, e.Start = YouTube, segs[0], startOf(u)
			return e
		}
	case "i.imgur.com":
		if len(segs) == 1 {
			ext := strings.ToLower(path.Ext(segs[0]))
			id := strings.TrimSuffix(segs[0], path.Ext(segs[0]))
			if imgurID.MatchString(id) {
				e.Kind, e.ImgurID = Imgur, id
				switch {
				case ext == ".gifv" || ext == ".mp4":
					e.MediaURL = "https://i.imgur.com/" + id + ".mp4"
				case imageExts[ext]:
					e.MediaURL = "https://i.imgur.com/" + segs[0]
				default:
					e.MediaURL = "https://i.imgur.com/" + id + ".jpg"
				}
				return e
			}
		}
	case "imgur.com":
		switch {
		case len(segs) == 2 && (segs[0] == "a" || segs[0] == "gallery" || segs[0] == "t"):
			// /gallery/some-title-ID: the ID is the last part.
			id := segs[1]
			if i := strings.LastIndexByte(id, '-'); i >= 0 {
				id = id[i+1:]
			}
			if imgurID.MatchString(id) {
				e.Kind, e.Album, e.ImgurID = Imgur, true, id
				return e
			}
		case len(segs) == 1 && imgurID.MatchString(segs[0]):
			e.Kind, e.ImgurID = Imgur, segs[0]
			e.MediaURL = "https://i.imgur.com/" + segs[0] + ".jpg"
			return e
		}
	case "giphy.com":
		// giphy.com/gifs/some-title-ID: the GIF is media.giphy.com/media/ID.
		if len(segs) == 2 && segs[0] == "gifs" {
			id := segs[1]
			if i := strings.LastIndexByte(id, '-'); i >= 0 {
				id = id[i+1:]
			}
			if id != "" {
				e.Kind, e.MediaURL = Image, "https://media.giphy.com/media/"+id+"/giphy.gif"
				return e
			}
		}
	}
	ext := strings.ToLower(path.Ext(u.Path))
	switch {
	case imageExts[ext]:
		e.Kind, e.MediaURL = Image, raw
	case videoExts[ext]:
		e.Kind, e.MediaURL = Video, raw
	}
	return e
}

// startOf reads YouTube's t= / start= in seconds ("90", "1m30s").
func startOf(u *url.URL) int {
	t := u.Query().Get("t")
	if t == "" {
		t = u.Query().Get("start")
	}
	if n, err := strconv.Atoi(strings.TrimSuffix(t, "s")); err == nil {
		return n
	}
	total, num := 0, 0
	for _, r := range t {
		switch {
		case r >= '0' && r <= '9':
			num = num*10 + int(r-'0')
		case r == 'h':
			total, num = total+num*3600, 0
		case r == 'm':
			total, num = total+num*60, 0
		case r == 's':
			total, num = total+num, 0
		default:
			return 0
		}
	}
	return total + num
}

// YouTubeThumb is the thumbnail of a video (480×360).
func YouTubeThumb(id string) string { return "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg" }

// YouTubePlayer is the privacy-friendlier embed URL of a video, for the
// inline web view.
func YouTubePlayer(id string, start int) string {
	s := "https://www.youtube-nocookie.com/embed/" + id + "?autoplay=1&rel=0&modestbranding=1"
	if start > 0 {
		s += "&start=" + strconv.Itoa(start)
	}
	return s
}

// Links returns the links of a message to preview: at most max, in
// order, without duplicates, skipping those in code.
func Links(n *format.Node, max int) []string {
	if n == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	var walk func(*format.Node)
	walk = func(n *format.Node) {
		if len(out) >= max {
			return
		}
		switch n.Kind {
		case format.Code, format.CodeBlock, format.Spoiler:
			return // nothing hidden or quoted as code previews
		case format.Link:
			if strings.HasPrefix(n.Href, "http") && !seen[n.Href] {
				seen[n.Href] = true
				out = append(out, n.Href)
			}
			return
		}
		for _, ch := range n.Children {
			walk(ch)
		}
	}
	walk(n)
	return out
}

// SoleMedia reports whether a message is nothing but a link to media (a
// GIF, an image or a video, as people paste from KLIPY, Tenor, Giphy or
// Imgur): it then shows as that media alone, without the link's text.
func SoleMedia(n *format.Node) (Embed, bool) {
	if n == nil || len(n.Children) != 1 || n.Children[0].Kind != format.Paragraph {
		return Embed{}, false
	}
	var link *format.Node
	for _, ch := range n.Children[0].Children {
		switch {
		case ch.Kind == format.Link && link == nil:
			link = ch
		case ch.Kind == format.Text && strings.TrimSpace(ch.Text) == "":
		default:
			return Embed{}, false // other words: a message with a link
		}
	}
	if link == nil {
		return Embed{}, false
	}
	e := Classify(link.Href)
	if e.MediaURL == "" || e.Album {
		return Embed{}, false
	}
	return e, true
}

// IsGIF reports whether a media URL is an animated GIF.
func IsGIF(u string) bool {
	return strings.HasSuffix(strings.ToLower(strings.SplitN(u, "?", 2)[0]), ".gif")
}

// Preview is what a page says about itself (OpenGraph), from the
// homeserver's preview_url.
type Preview struct {
	SiteName    string
	Title       string
	Description string
	ImageURL    string // an mxc:// URL from the server
	ImageW      int
	ImageH      int
}
