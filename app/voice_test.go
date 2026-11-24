package app

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
	"slopement/voice"
)

// fakeMic feeds samples when the test says so.
type fakeMic struct {
	on      func([]int16)
	fail    func(error)
	started bool
	stopped int
	failOn  error
}

func (m *fakeMic) Start(f func([]int16), onFail func(error)) error {
	if m.failOn != nil {
		return m.failOn
	}
	m.on, m.fail, m.started = f, onFail, true
	return nil
}
func (m *fakeMic) Stop() { m.stopped++ }

// speak is a second of a tone, delivered as the capture would.
func (m *fakeMic) speak(seconds int) {
	pcm := tone(voice.SampleRate * seconds)
	for i := 0; i < len(pcm); i += 1600 {
		m.on(pcm[i:min(i+1600, len(pcm))])
	}
}

func tone(n int) []int16 {
	out := make([]int16, n)
	for i := range out {
		if i%40 < 20 {
			out[i] = 12000
		} else {
			out[i] = -12000
		}
	}
	return out
}

// voiceSource records what was sent.
type voiceSource struct {
	timeline
	files []model.File
	rooms []string
}

func (s *voiceSource) SendFile(room string, f model.File, reply string) {
	s.files, s.rooms = append(s.files, f), append(s.rooms, room)
}

func voiceApp(t *testing.T, mic *fakeMic) (*App, *voiceSource, model.Room, *ui.Tester) {
	t.Helper()
	old := NewMic
	NewMic = func() (Mic, error) { return mic, nil }
	t.Cleanup(func() { NewMic = old })
	a := testApp()
	a.nav = kit.NavRail
	a.runUI = func(fn func()) { fn() }
	room, tl := longTimeline(a, 3)
	src := &voiceSource{timeline: timeline{a.src, tl}}
	a.src = src
	a.open(room.ID)
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	return a, src, room, tt
}

// With a microphone the empty composer has a mic button; pressing it starts a
// recording and shows the bar with the time; send ends it and sends a WAV
// marked as a voice message, with its length and waveform.
func TestRecordAndSendAVoiceMessage(t *testing.T) {
	mic := &fakeMic{}
	a, src, room, tt := voiceApp(t, mic)
	if !tt.HasText("Record a voice message") && !hasLabel(tt, "Record a voice message") {
		t.Fatal("no mic button")
	}
	if err := tt.Click("Record a voice message"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.rec == nil || !mic.started {
		t.Fatal("the recording did not start")
	}
	mic.speak(3)
	a.rec.takeLevel()
	settle(tt)
	if !tt.HasText("0:03") {
		t.Fatal("the bar does not show the time")
	}
	if hasLabel(tt, "Record a voice message") {
		t.Fatal("the mic button shows while recording")
	}
	if err := tt.Click("Send voice message"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.rec != nil || mic.stopped != 1 {
		t.Fatalf("not stopped: rec=%v stopped=%d", a.rec, mic.stopped)
	}
	if len(src.files) != 1 || src.rooms[0] != room.ID {
		t.Fatalf("sent %d files", len(src.files))
	}
	f := src.files[0]
	if f.Mime != "audio/wav" || f.Voice == nil || len(f.Voice.Waveform) != voice.WaveformBars ||
		f.Voice.Duration < 2900*time.Millisecond || f.Voice.Duration > 3100*time.Millisecond || string(f.Data[:4]) != "RIFF" {
		t.Fatalf("file = %q %q voice=%+v", f.Name, f.Mime, f.Voice)
	}
	if hasLabel(tt, "Send voice message") {
		t.Fatal("the recording bar is still shown after sending")
	}
}

// The bin throws the recording away; a tap is too short to send; leaving the chat
// switches the microphone off; a refusal is said in words, and no bar shows.
func TestVoiceCancelTooShortLeaveAndRefusal(t *testing.T) {
	mic := &fakeMic{}
	a, src, room, tt := voiceApp(t, mic)

	a.startVoice(room.ID)
	mic.speak(2)
	settle(tt)
	if err := tt.Click("Cancel recording"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.rec != nil || mic.stopped != 1 || len(src.files) != 0 {
		t.Fatalf("cancel: rec=%v stopped=%d sent=%d", a.rec, mic.stopped, len(src.files))
	}

	a.startVoice(room.ID)
	a.rec.onSamples(tone(voice.SampleRate / 10)) // a tenth of a second
	a.endVoice(true)
	if len(src.files) != 0 || a.toast == "" {
		t.Fatalf("a tap was sent or not explained: sent=%d toast=%q", len(src.files), a.toast)
	}

	a.toast = ""
	a.startVoice(room.ID)
	mic.speak(2)
	a.view = viewPeople // away from the chat
	settle(tt)
	if a.rec != nil {
		t.Fatal("the microphone stayed on after leaving the chat")
	}
	if len(src.files) != 0 {
		t.Fatal("a recording left behind was sent")
	}
	a.view = viewRoom

	refuse := &fakeMic{failOn: errors.New("permission denied")}
	NewMic = func() (Mic, error) { return refuse, nil }
	a.startVoice(room.ID)
	if a.rec != nil || a.toast == "" {
		t.Fatalf("a refusal: rec=%v toast=%q", a.rec, a.toast)
	}
}

// The system's permission question answered No arrives after Start returned: the
// recording is dropped, the bar goes, and the refusal is said in words.
func TestMicRefusedAfterStart(t *testing.T) {
	mic := &fakeMic{}
	a, src, room, tt := voiceApp(t, mic)
	a.startVoice(room.ID)
	if a.rec == nil {
		t.Fatal("not started")
	}
	mic.fail(errors.New("microphone access was not allowed"))
	said := a.toast // the next frame shows it and clears it
	settle(tt)
	if a.rec != nil || mic.stopped != 1 || len(src.files) != 0 {
		t.Fatalf("rec=%v stopped=%d sent=%d", a.rec, mic.stopped, len(src.files))
	}
	if said == "" || hasLabel(tt, "Send voice message") {
		t.Fatalf("toast=%q, bar shown=%v", said, hasLabel(tt, "Send voice message"))
	}
}

// Where the platform has no microphone code, there is no mic button.
func TestNoMicNoButton(t *testing.T) {
	a, _, _, tt := voiceApp(t, &fakeMic{})
	NewMic = nil
	settle(tt)
	tt.Frame()
	if hasLabel(tt, "Record a voice message") {
		t.Fatal("a mic button without a microphone")
	}
	_ = a
}

func hasLabel(tt *ui.Tester, label string) bool {
	for _, x := range tt.AccessAll() {
		if x.Name == label {
			return true
		}
	}
	return false
}
