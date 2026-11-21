package klipy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// A response shaped as KLIPY's (trimmed from a live one), with an ad mixed in.
const searchJSON = `{"result":true,"data":{"data":[
 {"id":3060549105033175,"slug":"cat-1","title":"Cat","type":"gif","blur_preview":"data:image/jpeg;base64,xx",
  "file":{"md":{"gif":{"url":"https://s/md.gif","width":349,"height":200,"size":84064}},
          "xs":{"gif":{"url":"https://s/xs.gif","width":87,"height":90,"size":99848},"jpg":{"url":"https://s/xs.jpg","width":87,"height":90,"size":2406}}}},
 {"id":1,"slug":"ad","title":"Buy","type":"ad","file":{}},
 {"id":2,"slug":"broken","title":"No files","type":"gif","file":{}}
],"current_page":1,"per_page":24,"has_next":true}}`

func TestSearchNative(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Path + "?" + r.URL.RawQuery
		w.Write([]byte(searchJSON))
	}))
	defer srv.Close()
	c := New(Config{Key: "k", BaseURL: srv.URL, CustomerID: "cust"})
	p, err := c.Search(context.Background(), GIFs, "cat", 1, 24)
	if err != nil {
		t.Fatal(err)
	}
	if !p.HasNext || len(p.Items) != 1 {
		t.Fatalf("page = %+v (ads and items without files must be skipped)", p)
	}
	it := p.Items[0]
	if it.ID != "3060549105033175" || it.Thumb.URL != "https://s/xs.gif" || it.Still.URL != "https://s/xs.jpg" || it.Full.URL != "https://s/md.gif" || it.Full.Width != 349 {
		t.Errorf("item = %+v", it)
	}
	for _, want := range []string{"/api/v1/k/gifs/search", "q=cat", "page=1", "per_page=24", "customer_id=cust"} {
		if !strings.Contains(got, want) {
			t.Errorf("request %s lacks %s", got, want)
		}
	}
}

func TestNoKey(t *testing.T) {
	t.Setenv("KLIPY_API_KEY", "")
	if _, err := New(Config{}).Trending(context.Background(), GIFs, 1, 0); !errors.Is(err, ErrNoKey) {
		t.Fatalf("err = %v, want ErrNoKey", err)
	}
}

func TestErrorsHideKey(t *testing.T) {
	c := New(Config{Key: "secret-key", BaseURL: "http://127.0.0.1:1", HTTP: &http.Client{Timeout: time.Second}})
	_, err := c.Trending(context.Background(), GIFs, 1, 1)
	if err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("err = %v", err)
	}
}

// TestLive hits the real API when KLIPY_LIVE=1 and a key is set.
func TestLive(t *testing.T) {
	if os.Getenv("KLIPY_LIVE") == "" || os.Getenv("KLIPY_API_KEY") == "" {
		t.Skip("KLIPY_LIVE not set")
	}
	c := New(Config{CustomerID: "slopement-test"})
	ctx := context.Background()
	for _, k := range []Kind{GIFs, Stickers} {
		p, err := c.Trending(ctx, k, 1, 4)
		if err != nil || len(p.Items) == 0 {
			t.Fatalf("trending %v: %v, %d items", k, err, len(p.Items))
		}
		cats, err := c.Categories(ctx, k)
		if err != nil {
			t.Fatalf("categories %v: %v", k, err)
		}
		t.Logf("kind %v: %d items, %d categories, first %q thumb %dx%d", k, len(p.Items), len(cats), p.Items[0].Title, p.Items[0].Thumb.Width, p.Items[0].Thumb.Height)
	}
}
