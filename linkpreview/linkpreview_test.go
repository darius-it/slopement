package linkpreview

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestBlockedAddresses(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1": true, "10.1.2.3": true, "192.168.1.1": true, "172.16.0.9": true, "169.254.169.254": true,
		"100.64.1.1": true, "0.0.0.0": true, "::1": true, "fe80::1": true, "fc00::1": true, "::ffff:127.0.0.1": true,
		"224.0.0.1": true, "93.184.216.34": false, "2606:4700:4700::1111": false, "8.8.8.8": false,
	} {
		if got := blocked(netip.MustParseAddr(addr)); got != want {
			t.Errorf("blocked(%s) = %v, want %v", addr, got, want)
		}
	}
}

const page = `<!doctype html><html><head><meta charset="utf-8">
<title>Fallback title</title>
<meta property="og:title" content="  A  great
 page ">
<meta property="og:description" content="About it all">
<meta property="og:site_name" content="Example Site">
<meta property="og:image" content="/img/cover.png">
<meta property="og:image:width" content="640"><meta property="og:image:height" content="360">
</head><body><meta property="og:title" content="from the body: ignored"></body></html>`

// The default client never goes to this machine or the user's network, so a
// link cannot make the app probe them; a name that resolves to one is caught
// as well, and a redirect to one.
func TestDefaultClientRefusesPrivateAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the server on the local machine was reached")
		w.Write([]byte(page))
	}))
	defer srv.Close()
	c := &Client{}
	if _, err := c.Fetch(context.Background(), srv.URL); !errors.Is(err, ErrBlocked) && !strings.Contains(err.Error(), ErrBlocked.Error()) {
		t.Fatalf("fetching a local server: %v", err)
	}
	if _, err := c.Fetch(context.Background(), strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)); err == nil {
		t.Fatal("fetched a name that points at this machine")
	}
	if _, err := c.Image(context.Background(), srv.URL+"/a.png"); err == nil {
		t.Fatal("fetched a picture from this machine")
	}
	for _, link := range []string{"file:///etc/passwd", "ftp://example.org/x", "javascript:alert(1)", "http://", "not a url"} {
		if _, err := c.Fetch(context.Background(), link); err == nil {
			t.Errorf("fetched %q", link)
		}
	}
}

func TestFetchReadsTheHead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(page))
		case "/bare":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>Just a title</title><meta name="description" content="Plain description"></head></html>`))
		case "/latin1":
			w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
			w.Write([]byte("<html><head><title>Caf\xe9 \xfcber</title></head></html>"))
		case "/redirect":
			http.Redirect(w, r, "/page", http.StatusFound)
		case "/to-file":
			http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
		case "/pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write([]byte("%PDF"))
		case "/img":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("\x89PNG-data"))
		case "/notimg":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{AllowPrivate: true}
	ctx := context.Background()

	p, err := c.Fetch(ctx, srv.URL+"/page")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "A great page" || p.Description != "About it all" || p.Site != "Example Site" ||
		p.Image != srv.URL+"/img/cover.png" || p.W != 640 || p.H != 360 {
		t.Fatalf("preview = %+v", p)
	}
	p, err = c.Fetch(ctx, srv.URL+"/bare")
	if err != nil || p.Title != "Just a title" || p.Description != "Plain description" || p.Site == "" || p.Image != "" {
		t.Fatalf("a page with a title only: %+v %v", p, err)
	}
	p, err = c.Fetch(ctx, srv.URL+"/latin1")
	if err != nil || p.Title != "Café über" {
		t.Fatalf("latin-1 title = %q %v", p.Title, err)
	}
	if p, err = c.Fetch(ctx, srv.URL+"/redirect"); err != nil || p.Title != "A great page" {
		t.Fatalf("followed a redirect: %+v %v", p, err)
	}
	if _, err = c.Fetch(ctx, srv.URL+"/to-file"); err == nil {
		t.Fatal("followed a redirect to a file")
	}
	if _, err = c.Fetch(ctx, srv.URL+"/pdf"); err == nil || !strings.Contains(err.Error(), "not a web page") {
		t.Fatalf("a PDF: %v", err)
	}
	if _, err = c.Fetch(ctx, srv.URL+"/missing"); err == nil {
		t.Fatal("a 404 made a preview")
	}
	if b, err := c.Image(ctx, srv.URL+"/img"); err != nil || string(b) != "\x89PNG-data" {
		t.Fatalf("image: %q %v", b, err)
	}
	if _, err := c.Image(ctx, srv.URL+"/notimg"); err == nil {
		t.Fatal("took a web page for a picture")
	}
}

func TestClip(t *testing.T) {
	if got := clip("  a   b\n c ", 10); got != "a b c" {
		t.Fatalf("clip = %q", got)
	}
	if got := clip(strings.Repeat("é", 300), 200); len([]rune(got)) != 200 || !strings.HasSuffix(got, "…") {
		t.Fatalf("clip of a long one = %d runes", len([]rune(got)))
	}
}
