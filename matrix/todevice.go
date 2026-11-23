package matrix

import (
	"context"
	"encoding/json"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
)

// A call's media keys reach us as encrypted to-device messages (Olm). The
// crypto machine decrypts what it knows (room keys, secrets) and drops the
// rest, so a call page never heard the other people's keys: it could send its
// own media, and not read anyone's. interceptToDevice takes the encrypted
// to-device events of a sync before the machine does, lets the machine decrypt
// and handle them exactly as it would have, and tells the call page of the ones
// it decrypted and did not know (the machine hands those back).
//
// It must run before the machine's own sync listener (register it first).
func (s *Source) interceptToDevice(helper *cryptohelper.CryptoHelper) func(context.Context, *mautrix.RespSync, string) bool {
	return func(ctx context.Context, resp *mautrix.RespSync, _ string) bool {
		mach := helper.Machine()
		if mach == nil || len(resp.ToDevice.Events) == 0 {
			return true
		}
		kept := resp.ToDevice.Events[:0]
		for _, evt := range resp.ToDevice.Events {
			evt.Type.Class = event.ToDeviceEventType
			if evt.Type != event.ToDeviceEncrypted || evt.Content.ParseRaw(evt.Type) != nil {
				kept = append(kept, evt) // for the machine, as before
				continue
			}
			if dec := mach.HandleEncryptedEvent(ctx, evt); dec != nil {
				s.onDecryptedToDevice(dec)
			}
		}
		resp.ToDevice.Events = kept
		return true
	}
}

// onDecryptedToDevice passes a call's media keys on to the call page.
func (s *Source) onDecryptedToDevice(dec *crypto.DecryptedOlmEvent) {
	if dec.Type.Type != "io.element.call.encryption_keys" {
		return
	}
	s.deliverToDevice(dec.Type.Type, dec.Sender.String(), dec.Content.VeryRaw)
}

// deliverToDevice tells the call page of a room (the one the message names) of
// a to-device message, in the widget API's shape.
func (s *Source) deliverToDevice(typ, sender string, content json.RawMessage) {
	var c struct {
		RoomID string `json:"room_id"`
	}
	_ = json.Unmarshal(content, &c)
	msg, err := json.Marshal(map[string]any{"type": typ, "sender": sender, "encrypted": true, "content": json.RawMessage(content)})
	if err != nil {
		return
	}
	s.callMu.Lock()
	var to []func(bool, json.RawMessage)
	for room, fn := range s.callWatch {
		if c.RoomID == "" || c.RoomID == room {
			to = append(to, fn)
		}
	}
	s.callMu.Unlock()
	for _, fn := range to {
		fn(true, msg)
	}
}
