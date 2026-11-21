package matrix

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/crypto/backup"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/crypto/ssss"
	"maunium.net/go/mautrix/crypto/verificationhelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// VerifyDriver is this device's verification over mautrix's
// verificationhelper: the emoji (SAS) with another device, or the
// Security Key (SSSS) with the key backup behind it. The protocol is
// mautrix's; this is the state the app shows and the buttons it presses.
//
// State changes come back through the Source's update function, on the
// UI thread alone; the buttons run their work off it.
type VerifyDriver struct {
	src    *Source
	cli    *mautrix.Client
	mach   *crypto.OlmMachine
	helper *verificationhelper.VerificationHelper
	ver    *cryptohelper.CryptoHelper

	txn id.VerificationTransactionID // the flow in flight
	st  model.VerifyState
}

// newVerify wires the driver into the sync and the client.
func newVerify(ctx context.Context, src *Source, cli *mautrix.Client, mach *crypto.OlmMachine, ver *cryptohelper.CryptoHelper) (*VerifyDriver, error) {
	v := &VerifyDriver{src: src, cli: cli, mach: mach, ver: ver}
	helper := verificationhelper.NewVerificationHelper(cli, mach, verificationhelper.NewInMemoryVerificationStore(), v, false, false, true)
	if err := helper.Init(ctx); err != nil {
		return nil, err
	}
	cli.Verification = helper
	v.helper = helper
	return v, nil
}

// State is the verification as the app shows it, read on the UI thread:
// Needed is filled in from what is still unreadable.
func (v *VerifyDriver) State() model.VerifyState {
	st := v.st
	// Idle or Done: a stray request or a finished verification that did
	// not bring the history must not hide the way to the Security Key.
	// Only what has waited past the grace period counts: a message that
	// arrived a moment ago usually has its key coming.
	st.Needed = (st.Phase == model.VerifyIdle || st.Phase == model.VerifyDone) && v.src.stuck() > 0
	return st
}

// stuck is how many messages have been waiting for keys past the grace
// period. UI thread.
func (s *Source) stuck() int {
	n := 0
	for _, r := range s.rooms {
		for _, raw := range r.raw {
			if !lockedForNow(raw.tile) {
				n++
			}
		}
	}
	return n
}

// set changes the state on the UI thread.
func (v *VerifyDriver) set(st model.VerifyState) {
	v.src.post(func() { v.st = st })
}

// BeginWithDevice starts the emoji flow toward the user's other devices:
// accept it on one of them, and the emoji appear here.
func (v *VerifyDriver) BeginWithDevice() {
	v.set(model.VerifyState{Phase: model.VerifyWorking, From: "your other devices"})
	go func() {
		txn, err := v.helper.StartVerification(context.Background(), v.cli.UserID)
		if err != nil {
			v.set(model.VerifyState{Phase: model.VerifyIdle, Err: "Could not start the verification: " + err.Error()})
			return
		}
		v.src.post(func() { v.txn = txn })
		v.set(model.VerifyState{Phase: model.VerifyWaiting, From: "your other device"})
	}()
}

// Accept agrees to an incoming request.
func (v *VerifyDriver) Accept() {
	txn := v.txn
	v.set(model.VerifyState{Phase: model.VerifyWorking})
	go func() {
		if err := v.helper.AcceptVerification(context.Background(), txn); err != nil {
			v.set(model.VerifyState{Phase: model.VerifyIdle, Err: "Accepting failed: " + err.Error()})
		}
	}()
}

// Decline refuses the flow in flight, whoever started it.
func (v *VerifyDriver) Decline() {
	txn, phase := v.txn, v.st.Phase
	v.st.Phase = model.VerifyIdle
	go v.helper.CancelVerification(context.Background(), txn, event.VerificationCancelCodeUser, "declined")
	if phase == model.VerifyAsked {
		// The other side's request: it keeps its own copy of the flow.
		v.set(model.VerifyState{Phase: model.VerifyIdle})
	}
}

// Confirm answers the emoji: matching, or not.
func (v *VerifyDriver) Confirm(match bool) {
	txn := v.txn
	if !match {
		v.st.Phase = model.VerifyIdle
		go v.helper.CancelVerification(context.Background(), txn, event.VerificationCancelCodeSASMismatch, "the emoji didn't match")
		v.set(model.VerifyState{Phase: model.VerifyIdle, Err: "The emoji didn't match; nothing was verified."})
		return
	}
	v.set(model.VerifyState{Phase: model.VerifyWorking, From: v.st.From, Emoji: v.st.Emoji})
	go func() {
		if err := v.helper.ConfirmSAS(context.Background(), txn); err != nil {
			v.set(model.VerifyState{Phase: model.VerifyIdle, Err: "Confirming failed: " + err.Error()})
		}
	}()
}

// UseKey verifies this device with the Security Key (or Phrase) and
// pulls the key backup behind it: the history that stays "waiting for
// keys" until then decrypts.
func (v *VerifyDriver) UseKey(key string) {
	if key == "" {
		return
	}
	v.set(model.VerifyState{Phase: model.VerifyWorking})
	go func() {
		ctx := context.Background()
		if err := v.mach.VerifyWithRecoveryKey(ctx, key); err != nil {
			v.set(model.VerifyState{Phase: model.VerifyIdle, Err: "That key did not work: " + plain(err)})
			return
		}
		if version, err := v.restoreBackup(ctx, key); err != nil {
			// The device is verified; only the history needs the backup.
			v.set(model.VerifyState{Phase: model.VerifyDone, Err: "Verified, but the message backup could not be read: " + plain(err)})
		} else if version == "" {
			v.set(model.VerifyState{Phase: model.VerifyDone, Err: "Verified, but your account has no message backup, so older messages cannot be restored."})
		} else {
			v.set(model.VerifyState{Phase: model.VerifyDone})
		}
		v.src.redecryptAll()
		// Said after the retries land (the posts run in order): what the
		// backup did not have stays unreadable, and the user hears so.
		v.src.post(func() {
			if v.st.Phase == model.VerifyDone && v.st.Err == "" && v.src.encrypted > 0 {
				v.st.Err = fmt.Sprintf("%d messages are still unreadable: their keys are not in your backup.", v.src.encrypted)
			}
		})
	}()
}

// restoreBackup reads the megolm backup key from secret storage and
// imports the whole backup into the crypto store, with the recovery key
// that unlocked it.
func (v *VerifyDriver) restoreBackup(ctx context.Context, key string) (id.KeyBackupVersion, error) {
	keyID, keyData, err := v.mach.SSSS.GetDefaultKeyData(ctx)
	if err != nil {
		return "", fmt.Errorf("reading secret storage: %w", err)
	}
	megolmKey, err := keyData.VerifyRecoveryKey(keyID, key)
	if err != nil {
		return "", fmt.Errorf("checking the key: %w", err)
	}
	var enc ssss.EncryptedAccountDataEventContent
	if err = v.cli.GetAccountData(ctx, event.AccountDataMegolmBackupKey.Type, &enc); err != nil {
		return "", fmt.Errorf("reading the backup key: %w", err)
	}
	decrypted, err := enc.Decrypt(event.AccountDataMegolmBackupKey.Type, megolmKey)
	if err != nil {
		return "", fmt.Errorf("unlocking the backup key: %w", err)
	}
	backupKey, err := backup.MegolmBackupKeyFromBytes(decrypted)
	if err != nil {
		return "", err
	}
	// Kept in the keychain so the next boots restore (and keep
	// restoring) the history without asking for the key again.
	_ = secrets.Set(secretKey(string(v.cli.UserID), string(v.cli.DeviceID), "backup-key"), decrypted)
	return v.mach.DownloadAndStoreLatestKeyBackup(ctx, backupKey)
}

// savedBackupKey is the backup key an earlier restore kept, or nil.
func (v *VerifyDriver) savedBackupKey() *backup.MegolmBackupKey {
	raw, err := secrets.Get(secretKey(string(v.cli.UserID), string(v.cli.DeviceID), "backup-key"))
	if err != nil || len(raw) == 0 {
		return nil
	}
	key, err := backup.MegolmBackupKeyFromBytes(raw)
	if err != nil {
		return nil
	}
	return key
}

// restoreSaved imports the key backup with the backup key an earlier
// run saved, then retries what still waits. Quiet when there is none.
func (v *VerifyDriver) restoreSaved(ctx context.Context) {
	raw, err := secrets.Get(secretKey(string(v.cli.UserID), string(v.cli.DeviceID), "backup-key"))
	if err != nil || len(raw) == 0 {
		// No key kept yet: ask the other devices for it, quietly, once
		// the first sync has had time to bring the waiting messages.
		time.Sleep(10 * time.Second)
		v.requestBackupKey(true)
		return
	}
	key, err := backup.MegolmBackupKeyFromBytes(raw)
	if err != nil {
		return
	}
	if _, err := v.mach.DownloadAndStoreLatestKeyBackup(ctx, key); err != nil {
		return
	}
	v.src.redecryptAll()
}

// plain strips mautrix's error walls to the one line a user reads.
func plain(err error) string {
	for err != nil {
		if unwrapped, ok := err.(interface{ Unwrap() error }); ok && unwrapped.Unwrap() != nil {
			err = unwrapped.Unwrap()
			continue
		}
		return err.Error()
	}
	return ""
}

// ---- the helper's callbacks, on the sync goroutine -------------------------------------

// VerificationRequested: they started it — their device, or our own
// other one. Our own echo of BeginWithDevice is ignored.
func (v *VerifyDriver) VerificationRequested(ctx context.Context, txn id.VerificationTransactionID, from id.UserID, fromDevice id.DeviceID) {
	if from == v.cli.UserID && fromDevice == v.cli.DeviceID {
		return // ours
	}
	v.src.post(func() {
		v.txn = txn
		v.st = model.VerifyState{
			Phase: model.VerifyAsked,
			From:  fromDevice.String() + " (" + mxidName(from.String()) + ")",
		}
	})
	v.src.changed()
}

// VerificationReady: both sides are in; the emoji come next.
func (v *VerifyDriver) VerificationReady(ctx context.Context, txn id.VerificationTransactionID, otherDevice id.DeviceID, _, _ bool, _ *verificationhelper.QRCode) {
	v.src.post(func() {
		v.txn = txn
		if v.st.Phase != model.VerifyEmoji {
			v.st.Phase = model.VerifyWorking
			v.st.From = otherDevice.String()
		}
	})
}

// VerificationCancelled: the flow is off.
func (v *VerifyDriver) VerificationCancelled(ctx context.Context, txn id.VerificationTransactionID, code event.VerificationCancelCode, reason string) {
	v.set(model.VerifyState{Phase: model.VerifyIdle, Err: "The verification was cancelled: " + reason})
}

// VerificationDone: the device is trusted; the other side begins to
// share its keys, so the waiting events are retried as they arrive.
func (v *VerifyDriver) VerificationDone(ctx context.Context, txn id.VerificationTransactionID, method event.VerificationMethod) {
	v.set(model.VerifyState{Phase: model.VerifyDone})
	go v.requestBackupKey(false)
}

// requestBackupKey asks the user's other (verified) devices for the key
// backup's secret, as Element does after an emoji verification: the
// verification alone moves no keys. With the secret, the whole backup is
// imported and the waiting messages decrypt. Gives up after two minutes
// (the other device may be off) pointing at the Security Key.
func (v *VerifyDriver) requestBackupKey(quiet bool) {
	ctx := context.Background()
	time.Sleep(2 * time.Second) // the other side signs this device first
	fail := func(msg string) {
		if !quiet { // a boot-time try says nothing when it finds no answer
			v.set(model.VerifyState{Phase: model.VerifyDone, Err: msg})
		}
	}
	var key *backup.MegolmBackupKey
	err := v.mach.GetOrRequestSecret(ctx, id.SecretMegolmBackupV1, func(secret string) (bool, error) {
		raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(strings.TrimSpace(secret), "="))
		if err != nil {
			return false, nil // not a key; keep waiting for another answer
		}
		k, err := backup.MegolmBackupKeyFromBytes(raw)
		if err != nil {
			return false, nil
		}
		key = k
		_ = secrets.Set(secretKey(string(v.cli.UserID), string(v.cli.DeviceID), "backup-key"), raw)
		return true, nil
	}, 2*time.Minute)
	if err != nil || key == nil {
		fail("Verified, but your other devices did not share the message backup. Use your Security Key to restore older messages.")
		return
	}
	if _, err := v.mach.DownloadAndStoreLatestKeyBackup(ctx, key); err != nil {
		fail("Verified, but the message backup could not be read: " + plain(err))
		return
	}
	v.src.redecryptAll()
}

// ShowSAS: the emoji to compare.
func (v *VerifyDriver) ShowSAS(ctx context.Context, txn id.VerificationTransactionID, emojis []rune, descriptions []string, _ []int) {
	out := make([]model.SASEmoji, len(emojis))
	for i, e := range emojis {
		name := ""
		if i < len(descriptions) {
			name = descriptions[i]
		}
		out[i] = model.SASEmoji{Emoji: string(e), Name: name}
	}
	v.src.post(func() {
		v.txn = txn
		v.st = model.VerifyState{Phase: model.VerifyEmoji, From: v.st.From, Emoji: out}
	})
	v.src.changed()
}

// The compile-time check that the driver is the helper's callback set.
var _ verificationhelper.RequiredCallbacks = (*VerifyDriver)(nil)
var _ verificationhelper.ShowSASCallbacks = (*VerifyDriver)(nil)
var _ model.Verify = (*VerifyDriver)(nil)
