package matrix

import (
	"context"
	"fmt"
	"os"
	"time"

	"maunium.net/go/mautrix/crypto/backup"
	"maunium.net/go/mautrix/id"
)

// keyWatch keeps the newest messages readable. A new message arrives
// before its key can: the sender's client shares the session with this
// device separately (and not at all when this device was off), and backs
// it up to the server a little later. So, while any message waits, every
// few seconds this asks the server backup for that one session (with the
// backup key kept from the recovery key or the other device), and asks the
// user's other devices to share it, once per session. What it finds
// decrypts the tile in place.
func (s *Source) keyWatch(ctx context.Context) {
	if s.helper == nil || s.verify == nil {
		return
	}
	asked := map[string]bool{} // sessions already requested from devices
	var (
		version id.KeyBackupVersion
		key     *backup.MegolmBackupKey
	)
	debug := os.Getenv("SLOPEMENT_DEBUG") != ""
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-s.keyKick:
			// A locked message just arrived: its key usually follows
			// within a moment, so look shortly, not on the next tick.
			select {
			case <-ctx.Done():
				return
			case <-time.After(1500 * time.Millisecond):
			}
		}
		snaps := s.snapshotRaw()
		if len(snaps) == 0 {
			continue
		}
		if key == nil {
			key = s.verify.savedBackupKey()
		}
		if key != nil && version == "" {
			if info, err := s.cli.GetKeyBackupLatestVersion(ctx); err == nil {
				version = info.Version
			}
		}
		for _, snap := range snaps {
			for evtID, raw := range snap.raw {
				_, err := s.helper.Decrypt(ctx, raw.evt)
				if err == nil {
					s.redecryptOne(ctx, snap, evtID, raw)
					continue
				}
				if debug {
					fmt.Fprintf(os.Stderr, "slopement: %s in %s still locked: %v\n", evtID, snap.id, err)
				}
				enc := raw.evt.Content.AsEncrypted()
				if enc == nil {
					continue
				}
				if key != nil && version != "" {
					s.importSession(ctx, version, key, raw.evt.RoomID, enc.SessionID)
					if _, err := s.helper.Decrypt(ctx, raw.evt); err == nil {
						s.redecryptOne(ctx, snap, evtID, raw)
						continue
					}
				}
				if sid := string(enc.SessionID); !asked[sid] {
					asked[sid] = true
					s.helper.RequestSession(ctx, raw.evt.RoomID, enc.SenderKey, enc.SessionID, raw.evt.Sender, enc.DeviceID)
				}
			}
		}
	}
}

// redecryptOne decrypts one waiting event and puts it in its tile's place.
func (s *Source) redecryptOne(ctx context.Context, snap *rawSnapshot, evtID string, raw *rawEvent) {
	if decrypted, err := s.helper.Decrypt(ctx, raw.evt); err == nil {
		s.applyDecrypted(snap, evtID, decrypted)
	}
}

// importSession fetches one session from the backup and imports it;
// quiet when the backup does not have it (yet).
func (s *Source) importSession(ctx context.Context, version id.KeyBackupVersion, key *backup.MegolmBackupKey, room id.RoomID, session id.SessionID) {
	resp, err := s.cli.GetKeyBackupForRoomAndSession(ctx, version, room, session)
	if err != nil || resp == nil {
		return
	}
	data, err := resp.SessionData.Decrypt(key)
	if err != nil {
		return
	}
	_, _ = s.verify.mach.ImportRoomKeyFromBackup(ctx, version, room, session, data)
}
