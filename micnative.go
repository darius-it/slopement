package main

import (
	"context"

	"slopement/app"
	"slopement/modules/mic"
	"slopement/voice"
)

// The microphone for voice messages is the native module modules/mic (Swift on
// Macs and iPhones, Kotlin on Android). Where it has no implementation
// (Linux, Windows) NewRecorder says so and the app offers no recording.

// nativeMic is the module's recorder as the app's Mic.
type nativeMic struct {
	rec    mic.Recorder
	cancel context.CancelFunc
}

func (m *nativeMic) Start(onSamples func([]int16), onFail func(error)) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go func() {
		// Reading the stream is what starts the capture: the first value may be
		// the refusal of the permission question, which comes later than Start.
		for chunk, err := range m.rec.Samples(ctx) {
			if err != nil {
				if ctx.Err() == nil { // not the end the app asked for
					onFail(err)
				}
				return
			}
			onSamples(voice.PCM(chunk))
		}
	}()
	return nil
}

func (m *nativeMic) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// useNativeMic gives the app the module's microphone, if this platform has one.
func useNativeMic() {
	rec, err := mic.NewRecorder()
	if err != nil {
		return // no microphone code here: no mic button
	}
	app.NewMic = func() (app.Mic, error) { return &nativeMic{rec: rec}, nil }
}
