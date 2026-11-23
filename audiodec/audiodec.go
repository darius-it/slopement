// Package audiodec decodes the audio mygo's audio package cannot — Ogg Opus,
// which is what Element's voice messages are, and AAC in .m4a, which is what
// most phones record — into WAV, which it can play. Both decoders are pure Go
// (no cgo): github.com/kazzmir/opus-go (libopus transpiled to Go) and
// github.com/tphakala/go-m4a with its AAC-LC decoder.
//
// The whole clip decodes at once, to 16-bit PCM in memory; a clip that would
// take more than MaxBytes of it is refused.
package audiodec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/kazzmir/opus-go/ogg"
	"github.com/kazzmir/opus-go/opus"
	"github.com/tphakala/go-m4a/aacm4a"
)

// MaxBytes bounds the PCM of a clip: about 25 minutes of 48 kHz stereo.
const MaxBytes = 288 << 20

// ErrUnsupported is returned for audio this package does not decode.
var ErrUnsupported = errors.New("audiodec: not an audio format it decodes")

// IsOggOpus reports Ogg Opus bytes.
func IsOggOpus(data []byte) bool {
	return len(data) > 40 && string(data[:4]) == "OggS" && bytes.Contains(data[:min(len(data), 128)], []byte("OpusHead"))
}

// IsMP4 reports an ISO base media file (.m4a, .mp4): "ftyp" after its size.
func IsMP4(data []byte) bool { return len(data) > 12 && string(data[4:8]) == "ftyp" }

// ToWAV decodes Ogg Opus or AAC (in .m4a) to a 16-bit PCM WAV file.
func ToWAV(data []byte) ([]byte, error) {
	switch {
	case IsOggOpus(data):
		return opusToWAV(data)
	case IsMP4(data):
		return aacToWAV(data)
	}
	return nil, ErrUnsupported
}

func opusToWAV(data []byte) ([]byte, error) {
	reader, err := ogg.NewOpusReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("audiodec: opus: %w", err)
	}
	head := reader.Head
	channels := int(head.Channels)
	if channels < 1 || channels > 2 {
		return nil, fmt.Errorf("audiodec: opus: %d channels", channels)
	}
	dec, err := opus.NewDecoderFromHead(head)
	if err != nil {
		return nil, fmt.Errorf("audiodec: opus: %w", err)
	}
	var pcm []byte
	skip := int(head.PreSkip) * channels // the decoder's start-up samples are not the clip
	for {
		packet, err := reader.ReadAudioPacket()
		if err != nil {
			break // the end (or the file stops short: what is decoded is played)
		}
		samples, n, err := dec.DecodePacket(packet, nil)
		if err != nil {
			return nil, fmt.Errorf("audiodec: opus: %w", err)
		}
		samples = samples[:min(len(samples), n*channels)]
		if skip > 0 {
			cut := min(skip, len(samples))
			samples, skip = samples[cut:], skip-cut
		}
		if len(pcm)+2*len(samples) > MaxBytes {
			return nil, errors.New("audiodec: the clip is too long")
		}
		for _, s := range samples {
			pcm = append(pcm, byte(s), byte(uint16(s)>>8))
		}
	}
	if len(pcm) == 0 {
		return nil, errors.New("audiodec: opus: no audio")
	}
	return wav(pcm, 48000, channels), nil
}

func aacToWAV(data []byte) ([]byte, error) {
	dec, info, err := aacm4a.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("audiodec: m4a: %w", err)
	}
	if info.Channels < 1 || info.Channels > 2 || info.SampleRate <= 0 {
		return nil, fmt.Errorf("audiodec: m4a: %d channels at %d Hz", info.Channels, info.SampleRate)
	}
	pcm, err := io.ReadAll(io.LimitReader(dec, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("audiodec: m4a: %w", err)
	}
	if len(pcm) > MaxBytes {
		return nil, errors.New("audiodec: the clip is too long")
	}
	if len(pcm) == 0 {
		return nil, errors.New("audiodec: m4a: no audio")
	}
	// The decoder does not apply the file's edit list: the encoder's priming
	// leads the clip, a few hundredths of a second.
	if lead := int(info.EncoderDelay) * info.Channels * 2; lead > 0 && lead < len(pcm) {
		pcm = pcm[lead:]
	}
	return wav(pcm, info.SampleRate, info.Channels), nil
}

// wav wraps 16-bit little-endian interleaved PCM as a WAV file.
func wav(pcm []byte, rate, channels int) []byte {
	var b bytes.Buffer
	b.Grow(44 + len(pcm))
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+len(pcm)))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16)) // the fmt chunk
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))  // PCM
	_ = binary.Write(&b, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&b, binary.LittleEndian, uint32(rate))
	_ = binary.Write(&b, binary.LittleEndian, uint32(rate*channels*2)) // bytes a second
	_ = binary.Write(&b, binary.LittleEndian, uint16(channels*2))      // bytes a frame
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))              // bits a sample
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(pcm)))
	b.Write(pcm)
	return b.Bytes()
}
