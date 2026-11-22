package app

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

type fakePlayer struct {
	playing bool
	pos     time.Duration
	total   time.Duration
	closed  bool
}

func (p *fakePlayer) Play()                  { p.playing = true }
func (p *fakePlayer) Pause()                 { p.playing = false }
func (p *fakePlayer) Stop()                  { p.playing, p.pos = false, 0 }
func (p *fakePlayer) Seek(t time.Duration)   { p.pos = t }
func (p *fakePlayer) Position() time.Duration { return p.pos }
func (p *fakePlayer) Duration() time.Duration { return p.total }
func (p *fakePlayer) Playing() bool          { return p.playing }
func (p *fakePlayer) Close()                 { p.closed = true }

type audSource struct {
	timeline
	loads int
}

func (s *audSource) LoadMedia(m model.Media, done func([]byte, error)) {
	s.loads++
	done([]byte("audio bytes of "+m.Name), nil)
}

func audioApp(t *testing.T) (*App, *audSource, model.Room) {
	t.Helper()
	a := testApp()
	room, tl := longTimeline(a, 4)
	tl = append(tl,
		model.Event{ID: "$a1", Room: room.ID, Sender: room.Partner, Kind: model.EventMedia, Body: "song.mp3", Time: time.Now(),
			Media: []model.Media{{Kind: model.MediaFile, Audio: true, Name: "song.mp3", URL: "mxc://x/a1", Length: 42 * time.Second}}},
		model.Event{ID: "$a2", Room: room.ID, Sender: room.Partner, Kind: model.EventMedia, Body: "talk.opus", Time: time.Now(),
			Media: []model.Media{{Kind: model.MediaFile, Audio: true, Name: "talk.opus", URL: "mxc://x/a2"}}},
	)
	src := &audSource{timeline: timeline{a.src, tl}}
	a.src = src
	a.runUI = func(f func()) { f() }
	a.open(room.ID)
	return a, src, room
}

// An audio message has a player: Play loads it (once), plays it, the bar and
// the time follow it, a second press pauses, a press on the bar seeks; one
// the audio package cannot decode says so and still offers the file; and a
// second message playing stops the first.
func TestAudioMessagePlayer(t *testing.T) {
	players := map[string]*fakePlayer{}
	old := openSound
	openSound = func(data []byte) (soundPlayer, error) {
		if string(data) == "audio bytes of talk.opus" {
			return nil, errors.New("unknown format")
		}
		p := &fakePlayer{total: 40 * time.Second}
		players[string(data)] = p
		return p, nil
	}
	defer func() { openSound = old }()

	a, src, _ := audioApp(t)
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	if !tt.HasText("0:42") {
		t.Fatal("the audio message does not show its length")
	}
	if err := tt.Click("Play song.mp3"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	p := players["audio bytes of song.mp3"]
	if p == nil || !p.playing || src.loads != 1 {
		t.Fatalf("not playing after Play: %+v loads %d", p, src.loads)
	}
	p.pos = 10 * time.Second
	settle(tt)
	if !tt.HasText("0:10 / 0:40") {
		t.Fatal("the time does not follow the playing")
	}
	if err := tt.Click("Pause song.mp3"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if p.playing {
		t.Fatal("Pause did not pause")
	}
	if err := tt.Click("Play song.mp3"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !p.playing || src.loads != 1 {
		t.Fatalf("playing again must not load again: loads %d", src.loads)
	}

	// A format that cannot be decoded.
	if err := tt.Click("Play talk.opus"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !tt.HasText("This format can't be played here") {
		t.Fatal("no word on a format that cannot be played")
	}

	// Seeking: a press on the bar goes there.
	a.audioAct(src.tl[len(src.tl)-2], kit.AudioAction{Seek: true, Where: 0.5})
	if p.pos != 20*time.Second {
		t.Fatalf("seek to the middle of 40s: %v", p.pos)
	}

	// A second playing stops the first.
	p2 := &fakePlayer{total: time.Minute}
	a.clips["$other"] = &clip{player: p2}
	a.stopOthers("$other")
	if p.playing {
		t.Fatal("the first kept playing")
	}
	a.closeClips()
	if !p.closed && !p2.closed {
		t.Fatal("players not released")
	}
}
