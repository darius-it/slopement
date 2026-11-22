package app

import (
	"testing"
	"time"

	"slopement/kit"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// bigPictureSource gives a person a profile picture on the media repo.
type bigPictureSource struct {
	model.Source
	loaded chan string
}

func (s *bigPictureSource) Person(id string) (model.Person, bool) {
	p, ok := s.Source.Person(id)
	p.Avatar = "mxc://example.org/" + id
	return p, ok
}

func (s *bigPictureSource) LoadMedia(m model.Media, done func([]byte, error)) {
	s.loaded <- m.URL
	done(nil, nil)
}

// Clicking a profile picture opens it in the viewer, which asks for the
// whole picture, not a thumbnail; a person with none opens nothing.
func TestProfilePictureOpensTheViewer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	src := &bigPictureSource{Source: m, loaded: make(chan string, 4)}
	a := New(src, picker.New(nil))
	a.runUI = func(f func()) { f() }

	if !a.profile("@alex:example.org").Picture {
		t.Fatal("the card does not know there is a picture")
	}
	a.profileActs("@alex:example.org", kit.ProfileAction{OpenPicture: true})
	if !a.viewer || len(a.viewerItems) != 1 || a.viewerAt != 0 {
		t.Fatalf("viewer %v, items %d", a.viewer, len(a.viewerItems))
	}
	it := a.viewerItems[0]
	if it.Full == nil || it.Label == "" {
		t.Fatalf("item = %+v", it)
	}
	it.Full() // the viewer asks for it
	select {
	case url := <-src.loaded:
		if url != "mxc://example.org/@alex:example.org" {
			t.Fatalf("fetched %q", url)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the full picture was not asked for")
	}

	a.viewer = false
	noPic := &plainPersonSource{Source: m}
	b := New(noPic, picker.New(nil))
	b.profileActs("@alex:example.org", kit.ProfileAction{OpenPicture: true})
	if b.viewer {
		t.Fatal("the viewer opened for a person with no picture")
	}
}

type plainPersonSource struct{ model.Source }
