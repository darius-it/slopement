package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// ---- Profile --------------------------------------------------------------

// SetDisplayName changes the user's name: it shows at once; done is called
// on the UI thread with "" or why it did not work (and the name is back).
func (s *Source) SetDisplayName(name string, done func(err string)) {
	name = strings.TrimSpace(name)
	if name == "" {
		done("A name can't be empty.")
		return
	}
	if s.cli == nil {
		done("Not signed in.")
		return
	}
	prev := s.me.Name
	s.me.Name = name
	s.changed()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.cli.SetDisplayName(ctx, name); err != nil {
			s.post(func() {
				s.me.Name = prev
				done("Could not change your name: " + plain(err))
			})
			return
		}
		s.post(func() { done("") })
	}()
}

// SetAvatar changes the user's picture: the middle square of the file,
// scaled down, is uploaded and set on the profile. It shows once uploaded
// (a picture of the size shown is fetched as any avatar is).
func (s *Source) SetAvatar(f model.File, done func(err string)) {
	if s.cli == nil {
		done("Not signed in.")
		return
	}
	go func() {
		data, mime, err := prepareAvatar(f.Data)
		if err != nil {
			s.post(func() { done("Could not use that picture: " + err.Error()) })
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		up, err := s.cli.UploadBytes(ctx, data, mime)
		if err != nil {
			s.post(func() { done("Could not upload the picture: " + plain(err)) })
			return
		}
		if err := s.cli.SetAvatarURL(ctx, up.ContentURI); err != nil {
			s.post(func() { done("Could not set the picture: " + plain(err)) })
			return
		}
		mxc := up.ContentURI.String()
		s.post(func() {
			s.me.Avatar = mxc
			done("")
		})
	}()
}

// RemoveAvatar takes the user's picture away (the initials show again).
func (s *Source) RemoveAvatar(done func(err string)) {
	if s.cli == nil {
		done("Not signed in.")
		return
	}
	prev := s.me.Avatar
	s.me.Avatar = ""
	s.changed()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.cli.SetAvatarURL(ctx, id.ContentURI{}); err != nil {
			s.post(func() {
				s.me.Avatar = prev
				done("Could not remove the picture: " + plain(err))
			})
			return
		}
		s.post(func() { done("") })
	}()
}

var _ model.Profile = (*Source)(nil)

// ---- Username colors ------------------------------------------------------

// The colors the user gave names (their own, and others' for themselves)
// are one account data event, com.slopement.user_colors: MXID → index in the
// palette. Every session of the account sees them.

const userColorsData = "com.slopement.user_colors"

var userColorsType = event.Type{Type: userColorsData, Class: event.AccountDataEventType}

// UserColors is the colors chosen, by MXID. UI thread; read only.
func (s *Source) UserColors() map[string]int { return s.userColors }

// SetUserColor chooses a color (an index of the palette) for a name, or with
// -1 goes back to the automatic one. It shows at once and is saved to the
// account.
func (s *Source) SetUserColor(mxid string, index int) {
	if s.userColors == nil {
		s.userColors = map[string]int{}
	}
	if index < 0 {
		delete(s.userColors, mxid)
	} else {
		s.userColors[mxid] = index
	}
	s.changed()
	if s.cli == nil {
		return
	}
	snapshot := make(map[string]int, len(s.userColors))
	for k, v := range s.userColors {
		snapshot[k] = v
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.cli.SetAccountData(ctx, userColorsData, snapshot)
	}()
}

// onUserColors is the colors as the account has them.
func (s *Source) onUserColors(ctx context.Context, evt *event.Event) {
	var m map[string]int
	if json.Unmarshal(evt.Content.VeryRaw, &m) != nil {
		return
	}
	s.post(func() { s.userColors = m })
}

var _ model.UserColors = (*Source)(nil)

// ---- Sessions -------------------------------------------------------------

// Sessions are the account's devices, as last asked for (RefreshSessions):
// this one first, then the most recently seen.
func (s *Source) Sessions() []model.Session { return s.sessions }

// RefreshSessions asks the homeserver for the account's devices and which
// of them this device trusts.
func (s *Source) RefreshSessions() {
	if s.cli == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resp, err := s.cli.GetDevicesInfo(ctx)
		if err != nil || resp == nil {
			return
		}
		me := s.cli.UserID
		own := s.cli.DeviceID
		var ownVerified bool
		if s.verify != nil && s.verify.mach != nil {
			_, ownVerified, _ = s.verify.mach.GetOwnVerificationStatus(ctx)
		}
		out := make([]model.Session, 0, len(resp.Devices))
		for _, d := range resp.Devices {
			ss := model.Session{
				ID: d.DeviceID.String(), Name: d.DisplayName, IP: d.LastSeenIP,
				Current: d.DeviceID == own,
			}
			if ss.Name == "" {
				ss.Name = d.DeviceID.String()
			}
			if d.LastSeenTS > 0 {
				ss.LastSeen = time.UnixMilli(d.LastSeenTS)
			}
			switch {
			case ss.Current:
				ss.Verified = ownVerified
			case s.verify != nil && s.verify.mach != nil:
				if dev, err := s.verify.mach.GetOrFetchDevice(ctx, me, d.DeviceID); err == nil && dev != nil {
					ss.Verified = s.verify.mach.IsDeviceTrusted(ctx, dev)
				}
			}
			out = append(out, ss)
		}
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Current != out[j].Current {
				return out[i].Current
			}
			return out[i].LastSeen.After(out[j].LastSeen)
		})
		s.post(func() { s.sessions = out })
	}()
}

// SignOutSession signs another device out. Homeservers ask to confirm that
// (user-interactive auth: a password, or on a MAS server their account page):
// then done says so with needsBrowser, and AccountPage is where to do it.
func (s *Source) SignOutSession(deviceID string, done func(err string, needsBrowser bool)) {
	if s.cli == nil {
		done("Not signed in.", false)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := s.cli.DeleteDevice(ctx, id.DeviceID(deviceID), nil)
		var he mautrix.HTTPError
		switch {
		case err == nil:
			s.post(func() {
				kept := s.sessions[:0:0]
				for _, ss := range s.sessions {
					if ss.ID != deviceID {
						kept = append(kept, ss)
					}
				}
				s.sessions = kept
				done("", false)
			})
		case errors.As(err, &he) && he.Response != nil && he.Response.StatusCode == 401:
			s.post(func() { done("", true) })
		default:
			s.post(func() { done("Could not sign that session out: " + plain(err), false) })
		}
	}()
}

// AccountPage is where the homeserver lets the user manage their sessions
// (a MAS server's account management page), or "" when it has none.
func (s *Source) AccountPage() string {
	if s.cli == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	meta, err := s.cli.OAuthGetServerMetadata(ctx)
	if err != nil || meta == nil {
		return ""
	}
	return meta.AccountManagementURI
}

var _ model.SessionManager = (*Source)(nil)
