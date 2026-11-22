package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"slopement/format"
	"slopement/model"
)

const fixtureDir = "../testdata/events"

// dumpConverted is the text a fixture is compared by: what the timeline
// would be given, one fact a line.
func dumpConverted(cv Converted) string {
	var b strings.Builder
	line := func(k string, v any) { fmt.Fprintf(&b, "%s: %v\n", k, v) }
	switch {
	case cv.Skipped != "":
		line("skipped", cv.Skipped)
	case cv.EditOf != "":
		line("edit-of", cv.EditOf)
		line("new-body", fmt.Sprintf("%q", cv.EditBody))
		line("new-text", fmt.Sprintf("%q", textOfHTML(cv.EditFormatted, cv.EditBody)))
	case cv.ReactionTo != "":
		line("reaction-to", cv.ReactionTo)
		line("key", cv.ReactionKey)
	default:
		e := cv.Event
		kinds := map[model.EventKind]string{model.EventText: "text", model.EventNotice: "notice", model.EventEmote: "emote",
			model.EventMedia: "media", model.EventPoll: "poll", model.EventSystem: "system", model.EventUndecryptable: "undecryptable"}
		line("kind", kinds[e.Kind])
		line("sender", e.Sender)
		line("body", fmt.Sprintf("%q", e.Body))
		if e.Formatted != "" {
			line("text", fmt.Sprintf("%q", textOfHTML(e.Formatted, e.Body)))
		}
		if e.ReplyTo != "" {
			line("reply-to", e.ReplyTo)
		}
		if e.ThreadRoot != "" {
			line("thread-root", e.ThreadRoot)
		}
		if e.Mentions {
			line("mentions-me", true)
		}
		if e.Membership != "" {
			line("membership", e.Membership)
		}
		if l := e.Location; l != nil {
			line("location", fmt.Sprintf("%.6f,%.6f %q", l.Lat, l.Lon, l.Description))
		}
		for _, m := range e.Media {
			line("media", fmt.Sprintf("kind=%d name=%q mime=%s %dx%d size=%d audio=%v sticker=%v url=%s",
				m.Kind, m.Name, m.Mime, m.W, m.H, m.Size, m.Audio, m.Sticker, m.URL))
		}
		if p := e.Poll; p != nil {
			line("poll", fmt.Sprintf("%q multiple=%v hidden=%v", p.Question, p.Multiple, p.Hidden))
			for _, a := range p.Answers {
				line("answer", fmt.Sprintf("%s=%q", a.ID, a.Text))
			}
		}
	}
	return b.String()
}

// textOfHTML is the words a message's HTML says, once sanitized.
func textOfHTML(formatted, body string) string {
	if formatted == "" {
		return body
	}
	return format.Plain(format.ParseHTML(formatted))
}

func fixtureNames(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(fixtureDir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures in %s: %v", fixtureDir, err)
	}
	sort.Strings(files)
	return files
}

// Every fixture converts, and what it comes to is what its golden says.
func TestFixturesGolden(t *testing.T) {
	update := os.Getenv("UPDATE_GOLDEN") != ""
	for _, f := range fixtureNames(t) {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			cv, err := ConvertEvent("@me:example.org", raw)
			if err != nil {
				t.Fatalf("%v", err)
			}
			got := dumpConverted(cv)
			golden := strings.TrimSuffix(f, ".json") + ".golden"
			if update {
				// Printed, not written: the sandbox does not let a test process write
				// project files. Save each block as <name>.golden.
				fmt.Printf("=== %s\n%s", golden, got)
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("no golden for %s (UPDATE_GOLDEN=1 writes it): %v", name, err)
			}
			if got != string(want) {
				t.Fatalf("%s\n--- got\n%s--- want\n%s", name, got, want)
			}
		})
	}
}
