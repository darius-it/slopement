package app

import (
	"sync"
	"time"

	"slopement/kit"
	"slopement/model"
	"slopement/voice"
)

// Voice messages. The microphone is a platform's own code (a native module: Swift
// on Macs and iPhones, Kotlin on Android), which main hands to the app as NewMic;
// where there is none, nil, and the mic button is not offered. The rest is here:
// the recording bar, the samples, the WAV file and the send.

// Mic is a microphone capture: while started it calls onSamples, from any
// goroutine, with 16-bit mono samples at voice.SampleRate, in order. Start
// returns at once; a refusal that only comes later (the system's permission
// question is answered No) or a microphone that stops is said to onFail, from
// any goroutine, once.
type Mic interface {
	Start(onSamples func(pcm []int16), onFail func(err error)) error
	Stop()
}

// NewMic makes a microphone capture, or says why it cannot (permission, no
// device). It is nil on platforms with no recording code. The device is not
// touched until the user presses the mic button: Start is what asks permission.
var NewMic func() (Mic, error)

// minVoice is the shortest message sent: a tap is a mistake, not a message.
const minVoice = 600 * time.Millisecond

// recording is one voice message being recorded.
type recording struct {
	room    string
	mic     Mic
	started time.Time
	stop    chan struct{}

	mu      sync.Mutex
	pcm     []int16
	fresh   []int16 // since the last level was taken
	levels  []float32
	stopped bool
}

// onSamples is the capture's callback (any goroutine).
func (r *recording) onSamples(s []int16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped || len(r.pcm)+len(s) > voice.SampleRate*int(voice.MaxDuration/time.Second) {
		return
	}
	r.pcm = append(r.pcm, s...)
	r.fresh = append(r.fresh, s...)
}

// takeLevel turns the samples since the last call into one more level bar.
func (r *recording) takeLevel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.levels = append(r.levels, voice.Level(r.fresh))
	r.fresh = r.fresh[:0]
	if len(r.levels) > 4*kit.VoiceBars {
		r.levels = r.levels[len(r.levels)-2*kit.VoiceBars:]
	}
}

// snapshot is what the bar shows now.
func (r *recording) snapshot() kit.VoiceState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return kit.VoiceState{Active: true, Elapsed: voice.Duration(len(r.pcm)),
		Levels: append([]float32(nil), r.levels...)}
}

// canRecord says a voice message can be started in this room.
func (a *App) canRecord(r model.Room) bool {
	if NewMic == nil || r.ReadOnly {
		return false
	}
	_, ok := a.src.(model.Uploader)
	return ok
}

// voiceStep hands the composer what it needs to show of voice messages, each
// frame: the mic button, and the bar while this room is being recorded.
func (a *App) voiceStep(r model.Room, st *kit.ComposerState) {
	st.CanRecord = a.canRecord(r) && a.rec == nil
	if a.rec != nil && a.rec.room == r.ID {
		st.Voice = a.rec.snapshot()
	} else {
		st.Voice = kit.VoiceState{}
	}
}

// voiceGuard throws a recording away when the user leaves the chat it belongs
// to (another chat, People, Settings): the microphone is never left on behind
// the screen the user sees.
func (a *App) voiceGuard() {
	if a.rec != nil && (a.view != viewRoom || a.room != a.rec.room) {
		a.endVoice(false)
	}
}

// startVoice begins recording in a room. The first time, the system asks for
// the microphone; a refusal is said in words.
func (a *App) startVoice(room string) {
	if a.rec != nil || NewMic == nil {
		return
	}
	mic, err := NewMic()
	if err != nil {
		a.toast = "Couldn't use the microphone: " + err.Error()
		return
	}
	rec := &recording{room: room, mic: mic, started: time.Now(), stop: make(chan struct{})}
	runUI := a.runUI
	onFail := func(err error) {
		if runUI == nil {
			return
		}
		runUI(func() {
			if a.rec == rec { // still the recording on screen
				a.endVoice(false)
				a.toast = "Couldn't use the microphone: " + err.Error()
			}
		})
	}
	if err := mic.Start(rec.onSamples, onFail); err != nil {
		a.toast = "Couldn't use the microphone: " + err.Error()
		return
	}
	a.rec = rec
	if runUI == nil {
		return
	}
	// While recording, ten frames a second keep the timer and the bars moving; the
	// ticker ends with the recording, and nothing else wakes the window.
	go func() {
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-rec.stop:
				return
			case <-t.C:
				rec.takeLevel()
				runUI(func() {
					if a.rec == rec && voice.Duration(len(rec.pcm)) >= voice.MaxDuration {
						a.endVoice(true) // the longest message: sent as it is
					}
				})
			}
		}
	}()
}

// endVoice stops the recording and sends it, or throws it away.
func (a *App) endVoice(send bool) {
	rec := a.rec
	if rec == nil {
		return
	}
	a.rec = nil
	rec.mic.Stop()
	close(rec.stop)
	rec.mu.Lock()
	rec.stopped = true
	pcm := rec.pcm
	rec.mu.Unlock()
	if !send {
		return
	}
	d := voice.Duration(len(pcm))
	if d < minVoice {
		a.toast = "Too short: a voice message needs a moment of speech"
		return
	}
	up, ok := a.src.(model.Uploader)
	if !ok {
		a.toast = "This account cannot send files"
		return
	}
	reply := a.replyTo[rec.room]
	up.SendFile(rec.room, model.File{
		Name:  "Voice message " + rec.started.Format("2006-01-02 15.04") + ".wav",
		Mime:  "audio/wav",
		Data:  voice.EncodeWAV(pcm),
		Voice: &model.VoiceNote{Duration: d, Waveform: voice.Waveform(pcm)},
	}, reply)
	if reply != "" {
		delete(a.replyTo, rec.room)
		a.draft(rec.room).Reply = nil
	}
	a.listState(rec.room).ScrollToEnd()
}
