package app

import (
	"bytes"
	"time"

	"github.com/egoist/mygo/audio"
	_ "github.com/egoist/mygo/audio/flac"
	_ "github.com/egoist/mygo/audio/mp3"
	_ "github.com/egoist/mygo/audio/mp4" // .m4a with Apple Lossless
	_ "github.com/egoist/mygo/audio/vorbis"

	"slopement/audiodec"
	"slopement/kit"
	"slopement/model"
)

// Audio messages play in their card. What the audio package decodes is what
// plays: WAV, MP3, Ogg Vorbis, FLAC, and Apple Lossless in .m4a, and — decoded
// to WAV first by package audiodec — Ogg Opus (what voice messages are: Element
// Web, Desktop and X send it). AAC in .m4a is not played. Anything else: the
// card says so, and the file card beside it saves the file. One message plays at a
// time: starting another stops the first.

// soundPlayer is what the app needs of a player (audio.Player's, so tests
// need no sound device).
type soundPlayer interface {
	Play()
	Pause()
	Stop()
	Seek(t time.Duration)
	Position() time.Duration
	Duration() time.Duration
	Playing() bool
	Close()
}

// openSound decodes audio bytes into a player: a variable, for the tests. What
// the audio package does not decode — Ogg Opus — is decoded to WAV first
// (audiodec), which it plays.
var openSound = func(data []byte) (soundPlayer, error) {
	p, err := audio.OpenStream(bytes.NewReader(data))
	if err == nil {
		return p, nil
	}
	if wavData, derr := audiodec.ToWAV(data); derr == nil {
		if p, perr := audio.OpenStream(bytes.NewReader(wavData)); perr == nil {
			return p, nil
		}
	}
	return nil, err
}

// clip is one audio message's playing state.
type clip struct {
	player  soundPlayer
	loading bool
	note    string // why it cannot play
}

// audioOf is an event's audio, if it is an audio message.
func audioOf(ev model.Event) (model.Media, bool) {
	for _, m := range ev.Media {
		if m.Audio {
			return m, true
		}
	}
	return model.Media{}, false
}

// audioData is the card for an audio message.
func (a *App) audioData(ev model.Event, m model.Media) *kit.AudioData {
	d := &kit.AudioData{ID: ev.ID, Name: m.Name}
	if m.Length > 0 {
		d.Length = kit.Clock(int(m.Length / time.Second))
	}
	cl := a.clips[ev.ID]
	if cl == nil {
		return d
	}
	d.Loading, d.Note = cl.loading, cl.note
	if cl.player != nil {
		total := cl.player.Duration()
		pos := cl.player.Position()
		d.Playing = cl.player.Playing()
		if total > 0 {
			d.Length = kit.Clock(int(total / time.Second))
			d.Progress = float32(pos) / float32(total)
		}
		if pos > 0 {
			d.Elapsed = kit.Clock(int(pos / time.Second))
		}
	}
	return d
}

// audioAct carries out what an audio card asked for.
func (a *App) audioAct(ev model.Event, act kit.AudioAction) {
	m, ok := audioOf(ev)
	if !ok {
		return
	}
	cl := a.clips[ev.ID]
	switch {
	case act.Toggle:
		switch {
		case cl != nil && cl.player != nil && cl.player.Playing():
			cl.player.Pause()
		case cl != nil && cl.player != nil:
			a.stopOthers(ev.ID)
			cl.player.Play()
			a.tickWhilePlaying(ev.ID)
		case cl != nil && cl.loading:
			// still on its way
		default:
			a.loadClip(ev, m)
		}
	case act.Seek && cl != nil && cl.player != nil:
		if total := cl.player.Duration(); total > 0 {
			cl.player.Seek(time.Duration(float64(total) * float64(act.Where)))
		}
	}
}

// stopOthers stops every clip but one: a second message starts, the first
// stops where it was.
func (a *App) stopOthers(keep string) {
	for id, cl := range a.clips {
		if id != keep && cl.player != nil && cl.player.Playing() {
			cl.player.Pause()
		}
	}
}

// loadClip downloads (and decrypts) an audio message, off the UI thread,
// and plays it once it is decoded.
func (a *App) loadClip(ev model.Event, m model.Media) {
	loader, ok := a.src.(model.MediaLoader)
	if !ok || a.runUI == nil {
		return
	}
	if a.clips == nil {
		a.clips = map[string]*clip{}
	}
	cl := &clip{loading: true}
	a.clips[ev.ID] = cl
	runUI, id := a.runUI, ev.ID
	loader.LoadMedia(m, func(data []byte, err error) {
		var p soundPlayer
		if err == nil {
			p, err = openSound(data)
		}
		runUI(func() {
			cl.loading = false
			if err != nil {
				cl.note = "This format can't be played here. Save the file to listen to it."
				return
			}
			a.stopOthers(id)
			cl.player = p
			p.Play()
			a.tickWhilePlaying(id)
		})
	})
}

// tickWhilePlaying asks for a frame five times a second while a clip plays:
// the bar moves, and nothing runs when nothing plays.
func (a *App) tickWhilePlaying(id string) {
	if a.win == nil {
		return
	}
	win := a.win
	var tick func()
	tick = func() {
		win.Update(func() {
			if cl := a.clips[id]; cl != nil && cl.player != nil && cl.player.Playing() {
				time.AfterFunc(200*time.Millisecond, tick)
			}
		})
	}
	time.AfterFunc(200*time.Millisecond, tick)
}

// closeClips releases the players (the window's account is changing, or it
// is closing).
func (a *App) closeClips() {
	for _, cl := range a.clips {
		if cl.player != nil {
			cl.player.Stop()
			cl.player.Close()
		}
	}
	a.clips = nil
}
