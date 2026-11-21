package model

// VerifyPhase is where a verification of this device is.
type VerifyPhase int

const (
	VerifyIdle    VerifyPhase = iota // not verifying
	VerifyWaiting                    // we started it: accept on the other device
	VerifyAsked                      // they started it: we accept or decline
	VerifyEmoji                      // compare the emoji
	VerifyWorking                    // a Security Key is being read
	VerifyDone                       // this device is verified
)

// SASEmoji is one of the seven emoji of a SAS comparison, with its name
// from the spec's table.
type SASEmoji struct {
	Emoji, Name string
}

// VerifyState is the verification as the app shows it. The phase says
// which screen; Err is one line when a try failed, on the screen the
// phase says. Needed says this session is unverified with messages it
// cannot read — the app shows its prompt once.
type VerifyState struct {
	Phase VerifyPhase
	// From is who is on the other side of the emoji, or who asked.
	From  string
	Emoji []SASEmoji
	Err   string
	// Needed: unverified, with encrypted messages waiting for keys.
	Needed bool
}

// Verify is this device's verification, from the Source. A nil Verify
// means the source cannot verify (the mock: a fake that walks the same
// screens; the matrix layer before sync: nil).
type Verify interface {
	// State is read on the UI thread every build.
	State() VerifyState
	// BeginWithDevice starts the emoji flow toward the user's other
	// devices; accept it there.
	BeginWithDevice()
	// Accept agrees to an incoming request; Decline refuses it.
	Accept()
	Decline()
	// Confirm answers the emoji: matching, or not.
	Confirm(match bool)
	// UseKey restores this device with the Security Key or Phrase.
	UseKey(key string)
}
