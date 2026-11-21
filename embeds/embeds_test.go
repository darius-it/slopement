package embeds

import (
	"reflect"
	"testing"

	"slopement/format"
)

func TestClassify(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Embed
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", Embed{Kind: YouTube, VideoID: "dQw4w9WgXcQ"}},
		{"https://youtu.be/dQw4w9WgXcQ?t=1m30s", Embed{Kind: YouTube, VideoID: "dQw4w9WgXcQ", Start: 90}},
		{"https://m.youtube.com/watch?v=dQw4w9WgXcQ&t=42", Embed{Kind: YouTube, VideoID: "dQw4w9WgXcQ", Start: 42}},
		{"https://youtube.com/shorts/abcdefghijk", Embed{Kind: YouTube, VideoID: "abcdefghijk", Shorts: true}},
		{"https://www.youtube.com/watch?v=short", Embed{Kind: Generic}},
		{"https://i.imgur.com/AbC123x.gifv", Embed{Kind: Imgur, ImgurID: "AbC123x", MediaURL: "https://i.imgur.com/AbC123x.mp4"}},
		{"https://i.imgur.com/AbC123x.png", Embed{Kind: Imgur, ImgurID: "AbC123x", MediaURL: "https://i.imgur.com/AbC123x.png"}},
		{"https://imgur.com/AbC123x", Embed{Kind: Imgur, ImgurID: "AbC123x", MediaURL: "https://i.imgur.com/AbC123x.jpg"}},
		{"https://imgur.com/a/Xy12AbC", Embed{Kind: Imgur, ImgurID: "Xy12AbC", Album: true}},
		{"https://imgur.com/gallery/funny-cat-Xy12AbC", Embed{Kind: Imgur, ImgurID: "Xy12AbC", Album: true}},
		{"https://example.com/cat.GIF", Embed{Kind: Image, MediaURL: "https://example.com/cat.GIF"}},
		{"https://example.com/clip.mp4", Embed{Kind: Video, MediaURL: "https://example.com/clip.mp4"}},
		{"https://matrix.org/blog", Embed{Kind: Generic}},
		{"ftp://x/y.png", Embed{Kind: Generic}},
	} {
		got := Classify(tc.in)
		tc.want.URL = tc.in
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Classify(%q)\n got %+v\nwant %+v", tc.in, got, tc.want)
		}
	}
}

func TestSoleMedia(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string // the media URL; "" for no
	}{
		{"https://static.klipy.com/ii/abc/8a/b1/uAaLRrFA.gif", "https://static.klipy.com/ii/abc/8a/b1/uAaLRrFA.gif"},
		{"  https://media.tenor.com/x/cat.gif  ", "https://media.tenor.com/x/cat.gif"},
		{"https://giphy.com/gifs/cat-funny-3o7TKSjRrfIPjeiVyM", "https://media.giphy.com/media/3o7TKSjRrfIPjeiVyM/giphy.gif"},
		{"https://i.imgur.com/AbC123x.gifv", "https://i.imgur.com/AbC123x.mp4"},
		{"look https://static.klipy.com/a.gif", ""},
		{"https://matrix.org", ""},
		{"https://imgur.com/a/Xy12AbC", ""},
	} {
		e, ok := SoleMedia(format.Parse(tc.in))
		if got := map[bool]string{true: e.MediaURL}[ok]; got != tc.want {
			t.Errorf("SoleMedia(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLinks(t *testing.T) {
	n := format.Parse("see https://a.com and [b](https://b.com), `https://code.com` ||https://hidden.com|| https://a.com https://c.com https://d.com")
	got := Links(n, 3)
	want := []string{"https://a.com", "https://b.com", "https://c.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Links = %v, want %v", got, want)
	}
}
