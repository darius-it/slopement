package matrix

import (
	"context"
	"errors"
	"fmt"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"

	"slopement/model"
)

// Keys: what is kept of the account's encryption on the homeserver and on this
// device, a file of the room keys to take elsewhere, and starting over with a
// new identity. The protocol is mautrix's crypto package; these are the
// questions the app asks of it.

// KeyStatus is what is known of the account's key storage; RefreshKeyStatus
// asks the server and the local crypto store for it.
func (s *Source) KeyStatus() model.KeyStatus { return s.keyStatus }

// RefreshKeyStatus finds out whether the account has a key backup on its
// homeserver, secret storage (a Security Key) and cross-signing keys, whether
// this device is verified, and whether it can read the backup.
func (s *Source) RefreshKeyStatus() {
	if s.verify == nil || s.cli == nil {
		s.keyStatus = model.KeyStatus{Asked: true, Problem: "Encryption is not set up on this account."}
		s.changed()
		return
	}
	v := s.verify
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		st := model.KeyStatus{Asked: true, HaveBackupKey: v.keyed.Load()}
		if ver, err := s.cli.GetKeyBackupLatestVersion(ctx); err == nil {
			st.BackupVersion, st.BackupKeys = string(ver.Version), ver.Count
		} else if !errors.Is(err, mautrix.MNotFound) {
			st.Problem = "Could not ask your homeserver about the backup: " + plain(err)
		}
		if has, verified, err := v.mach.GetOwnVerificationStatus(ctx); err == nil {
			st.CrossSigning, st.Verified = has, verified
		}
		if _, _, err := v.mach.SSSS.GetDefaultKeyData(ctx); err == nil {
			st.SecretStorage = true
		}
		s.post(func() { s.keyStatus = st; s.changed() })
	}()
}

// ExportRoomKeys makes a file of the keys this device holds for reading
// encrypted rooms, locked with passphrase (Element's format: it opens there
// too). done gets the file and how many keys are in it, on the UI thread.
func (s *Source) ExportRoomKeys(passphrase string, done func(data []byte, count int, problem string)) {
	if s.verify == nil {
		done(nil, 0, "Encryption is not set up on this account.")
		return
	}
	if passphrase == "" {
		done(nil, 0, "Choose a passphrase to lock the file with.")
		return
	}
	mach := s.verify.mach
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		sessions, err := mach.CryptoStore.GetAllGroupSessions(ctx).AsList()
		var data []byte
		if err == nil && len(sessions) == 0 {
			s.post(func() { done(nil, 0, "This device holds no room keys yet: there is nothing to export.") })
			return
		}
		if err == nil {
			data, err = crypto.ExportKeys(passphrase, sessions)
		}
		s.post(func() {
			if err != nil {
				done(nil, 0, "Could not export the keys: "+plain(err))
				return
			}
			done(data, len(sessions), "")
		})
	}()
}

// ImportRoomKeys reads a file of room keys (from Element, another device or
// ExportRoomKeys) into this device, and has the messages that were waiting
// for them tried again.
func (s *Source) ImportRoomKeys(passphrase string, data []byte, done func(imported, total int, problem string)) {
	if s.verify == nil {
		done(0, 0, "Encryption is not set up on this account.")
		return
	}
	mach := s.verify.mach
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		imported, total, err := mach.ImportKeys(ctx, passphrase, data)
		s.post(func() {
			if err != nil {
				msg := "Could not import the keys: " + plain(err)
				switch {
				case errors.Is(err, crypto.ErrMismatchingExportHash):
					msg = "That passphrase does not open this file."
				case errors.Is(err, crypto.ErrMissingExportPrefix), errors.Is(err, crypto.ErrMissingExportSuffix):
					msg = "That is not a file of room keys."
				}
				done(0, 0, msg)
				return
			}
			done(imported, total, "")
			if imported > 0 {
				go s.redecryptAll()
			}
		})
	}()
}

// uiaAnswer answers a homeserver's request to prove who the user is before
// replacing their cross-signing keys (user-interactive authentication):
//   - the account password, when the server takes one and the user gave it;
//   - an approval in the browser, for servers with an account service (MAS,
//     MSC4312: stages m.oauth or org.matrix.cross_signing_reset): approve is
//     given the page's address, has the user open it, and returns once they say
//     they approved it (false: they gave up);
//   - nothing, for a server that asks for nothing (m.login.dummy).
//
// nil ends the attempt: no way to prove it that the app can do.
func uiaAnswer(user string, resp *mautrix.RespUserInteractive, password string, approve func(url string) bool) any {
	if resp == nil {
		return nil
	}
	oauthStages := []mautrix.AuthType{mautrix.AuthTypeOAuth, "org.matrix.cross_signing_reset"}
	for _, stage := range oauthStages {
		if !resp.HasSingleStageFlow(stage) {
			continue
		}
		params, _ := resp.Params[stage].(map[string]any)
		url, _ := params["url"].(string)
		if url == "" || approve == nil || !approve(url) {
			return nil
		}
		return &mautrix.BaseAuthData{Type: stage, Session: resp.Session}
	}
	if resp.HasSingleStageFlow(mautrix.AuthTypePassword) && password != "" {
		return &mautrix.ReqUIAuthLogin{
			BaseAuthData: mautrix.BaseAuthData{Type: mautrix.AuthTypePassword, Session: resp.Session},
			User:         user, Password: password,
		}
	}
	if resp.HasSingleStageFlow(mautrix.AuthTypeDummy) {
		return &mautrix.BaseAuthData{Type: mautrix.AuthTypeDummy, Session: resp.Session}
	}
	return nil
}

// NeedsPasswordToReset says whether resetting the identity may need the
// account password: it does for a password account; for one with an account
// service the browser approves instead.
func (s *Source) NeedsPasswordToReset() bool {
	// An account signed in through the browser (OIDC) has a refresh token; a
	// password one does not.
	if s.cli == nil {
		return false
	}
	tok, err := TokensOf(s.me.ID, string(s.cli.DeviceID))
	return err != nil || tok.Refresh == ""
}

// ResetIdentity replaces the account's cross-signing keys and its secret
// storage with new ones, and verifies this device with them: other devices and
// the people who verified the account see it as new, and the old Security Key
// stops working. done gets the new Security Key (it must be kept: it is the
// only way to verify another device without a device at hand) or why not. The
// room keys on this device are not touched.
func (s *Source) ResetIdentity(req model.IdentityReset, done func(recoveryKey, problem string)) {
	if s.verify == nil {
		done("", "Encryption is not set up on this account.")
		return
	}
	mach, user := s.verify.mach, s.me.ID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		key, _, err := mach.GenerateAndUploadCrossSigningKeys(ctx, func(resp *mautrix.RespUserInteractive) any {
			return uiaAnswer(user, resp, req.Password, req.Approve)
		}, "")
		if err == nil {
			if err = mach.SignOwnDevice(ctx, mach.OwnIdentity()); err != nil {
				err = fmt.Errorf("signing this device: %w", err)
			} else if err = mach.SignOwnMasterKey(ctx); err != nil {
				err = fmt.Errorf("signing the new identity: %w", err)
			}
		}
		s.post(func() {
			if err != nil {
				msg := "Could not reset the identity: " + plain(err)
				var he mautrix.HTTPError
				if errors.As(err, &he) && he.RespError != nil && he.RespError.ErrCode == "M_FORBIDDEN" {
					msg = "Your homeserver did not accept it: the password was wrong or the approval was not given."
				}
				done("", msg)
				return
			}
			s.RefreshKeyStatus()
			done(key, "")
		})
	}()
}

var (
	_ model.KeyStorage       = (*Source)(nil)
	_ model.IdentityResetter = (*Source)(nil)
)
