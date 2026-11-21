package klipy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchParsesTenorShape(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Path + "?" + r.URL.RawQuery
		w.Write([]byte(`{"results":[
			{"id":"1","title":"hi","media_formats":{"gif":{"url":"https://x/1.gif","dims":[480,270],"size":1000},"tinygif":{"url":"https://x/1t.gif","dims":[220,124]}}},
			{"id":"2","title":"","content_description":"no full","media_formats":{"tinygif":{"url":"https://x/2t.gif"}}}
		],"next":"CAgQ"}`))
	}))
	defer srv.Close()
	c := New(Config{Key: "k", BaseURL: srv.URL})
	p, err := c.Search(context.Background(), GIFs, "hello", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Next != "CAgQ" || len(p.Items) != 2 {
		t.Fatalf("page = %+v", p)
	}
	if it := p.Items[0]; it.Full.URL != "https://x/1.gif" || it.Full.Width != 480 || it.Preview.URL != "https://x/1t.gif" {
		t.Errorf("item 0 = %+v", it)
	}
	if it := p.Items[1]; it.Full.URL != "https://x/2t.gif" || it.Title != "no full" {
		t.Errorf("item 1 falls back to tinygif and description: %+v", it)
	}
	if got == "" || !contains(got, "q=hello") || !contains(got, "key=k") || !contains(got, "/v2/search") {
		t.Errorf("request = %s", got)
	}
}

func TestNoKey(t *testing.T) {
	t.Setenv("KLIPY_API_KEY", "")
	c := New(Config{})
	if _, err := c.Trending(context.Background(), GIFs, 0, ""); !errors.Is(err, ErrNoKey) {
		t.Fatalf("err = %v, want ErrNoKey", err)
	}
}

func TestClipsUnsupported(t *testing.T) {
	c := New(Config{Key: "k"})
	if _, err := c.Trending(context.Background(), Clips, 0, ""); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
