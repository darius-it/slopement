package mock

import (
	"slopement/model"
)

// fakeVerify walks the same screens as the crypto layer's real one, on
// made-up emoji: the app's flow and tests run on it.
type fakeVerify struct {
	st model.VerifyState
}

// Verify is this (mock) device's verification.
func (s *Source) Verify() model.Verify { return s.verify }

func (v *fakeVerify) State() model.VerifyState { return v.st }

func (v *fakeVerify) BeginWithDevice() {
	v.st = model.VerifyState{Phase: model.VerifyEmoji, From: "your other device", Emoji: demoSAS}
}

func (v *fakeVerify) Accept() {
	v.st = model.VerifyState{Phase: model.VerifyEmoji, From: "your other device", Emoji: demoSAS}
}

func (v *fakeVerify) Decline() { v.st = model.VerifyState{Phase: model.VerifyIdle} }

func (v *fakeVerify) Confirm(match bool) {
	if match {
		v.st = model.VerifyState{Phase: model.VerifyDone}
	} else {
		v.st = model.VerifyState{Phase: model.VerifyIdle, Err: "The emoji didn't match."}
	}
}

func (v *fakeVerify) UseKey(string) { v.st = model.VerifyState{Phase: model.VerifyDone} }

// demoSAS is a comparison that never matches anything real.
var demoSAS = []model.SASEmoji{
	{Emoji: "🐶", Name: "Dog"}, {Emoji: "🔑", Name: "Key"}, {Emoji: "🌵", Name: "Cactus"}, {Emoji: "🎸", Name: "Guitar"},
	{Emoji: "🚀", Name: "Rocket"}, {Emoji: "🍄", Name: "Mushroom"}, {Emoji: "⚓", Name: "Anchor"},
}
