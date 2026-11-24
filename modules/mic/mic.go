// Package mic is a native module: the device's microphone, for voice messages.
// Swift implements it on macOS and iOS (AVFoundation) and Kotlin on Android
// (AudioRecord); on other platforms there is no implementation, and the app
// offers no recording (NewRecorder returns errors.ErrUnsupported).
package mic

import (
	"context"
	"iter"
)

// SampleRate is the rate of what Samples yields: 16 kHz is enough for speech.
const SampleRate = 16000

// Recorder records from the microphone.
//
//mygo:native
type Recorder interface {
	// Samples starts recording when it is ranged over and yields what is heard,
	// in chunks of about 100 ms, as mono 16-bit little-endian PCM at SampleRate,
	// until ctx ends (the recording then stops and the microphone is released).
	// The first time, the system asks the user for the microphone: when it is
	// refused, or there is no microphone, the first value is an error that says
	// so.
	Samples(ctx context.Context) iter.Seq2[[]byte, error]
}
