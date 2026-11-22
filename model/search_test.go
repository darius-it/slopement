package model

import (
	"reflect"
	"testing"
	"time"
)

func TestParseSearch(t *testing.T) {
	q := ParseSearch("lunch plans in:design from:kai has:image has:link has:image before:2026-03-01 after:2026-01-15 mentions:me")
	if q.Words != "lunch plans" || q.In != "design" || q.From != "kai" || !q.Mentions {
		t.Fatalf("parsed %+v", q)
	}
	if !reflect.DeepEqual(q.Has, []string{"image", "link"}) {
		t.Fatalf("has = %v (kinds once each, in order)", q.Has)
	}
	if q.Before.Format("2006-01-02") != "2026-03-01" || q.After.Format("2006-01-02") != "2026-01-15" {
		t.Fatalf("dates %v %v", q.Before, q.After)
	}
	if q.Empty() {
		t.Fatal("a full query is empty")
	}

	// What is not a filter stays a word: nothing typed is dropped.
	q = ParseSearch("meet at 10:30 has:sandwich before:soon mentions:you url:x https://a.b/c")
	if q.Words != "meet at 10:30 has:sandwich before:soon mentions:you url:x https://a.b/c" || len(q.Has) != 0 || q.Mentions {
		t.Fatalf("unknown tokens: %+v", q)
	}
	if !ParseSearch("   ").Empty() || ParseSearch("").Words != "" {
		t.Fatal("blank is not empty")
	}
	if ParseSearch("IN:Dev FROM:Ada HAS:GIF").In != "Dev" {
		t.Fatal("keys are not case-insensitive")
	}
}

// What the chips and the typed line say is one thing.
func TestSearchTextRoundTrip(t *testing.T) {
	text := "lunch in:design from:kai has:image has:link after:2026-01-15 before:2026-03-01 mentions:me"
	if got := ParseSearch(text).Text(); got != text {
		t.Fatalf("round trip:\n got %q\nwant %q", got, text)
	}
	q := ParseSearch("hello")
	q = q.Toggle("gif")
	if q.Text() != "hello has:gif" || !q.HasKind("gif") {
		t.Fatalf("toggled on: %q", q.Text())
	}
	q = q.Toggle("gif")
	if q.Text() != "hello" || q.HasKind("gif") {
		t.Fatalf("toggled off: %q", q.Text())
	}
	_ = time.Now
}
