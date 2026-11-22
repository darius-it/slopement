package matrix

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/crypto/signatures"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

func uia(flows ...[]mautrix.AuthType) *mautrix.RespUserInteractive {
	r := &mautrix.RespUserInteractive{Session: "sess", Params: map[mautrix.AuthType]any{}}
	for _, f := range flows {
		r.Flows = append(r.Flows, mautrix.UIAFlow{Stages: f})
	}
	return r
}

// How the app proves who the user is to replace their identity: the password
// where the server takes one, the browser approval where it has an account
// service, nothing where nothing is asked, and no answer otherwise.
func TestUIAAnswer(t *testing.T) {
	const user = "@me:example.org"

	// A password account.
	resp := uia([]mautrix.AuthType{mautrix.AuthTypePassword})
	got, ok := uiaAnswer(user, resp, "pw", nil).(*mautrix.ReqUIAuthLogin)
	if !ok || got.Password != "pw" || got.User != user || got.Type != mautrix.AuthTypePassword || got.Session != "sess" {
		t.Fatalf("password answer = %+v", got)
	}
	if uiaAnswer(user, resp, "", nil) != nil {
		t.Fatal("answered a password request without a password")
	}

	// An account service: both stage names, the page opened and waited on.
	for _, stage := range []mautrix.AuthType{mautrix.AuthTypeOAuth, "org.matrix.cross_signing_reset"} {
		resp := uia([]mautrix.AuthType{stage})
		resp.Params[stage] = map[string]any{"url": "https://account.example.org/reset?x=1"}
		var opened string
		a, ok := uiaAnswer(user, resp, "", func(url string) bool { opened = url; return true }).(*mautrix.BaseAuthData)
		if !ok || a.Type != stage || a.Session != "sess" || opened != "https://account.example.org/reset?x=1" {
			t.Fatalf("%s: answer %+v, opened %q", stage, a, opened)
		}
		if uiaAnswer(user, resp, "", func(string) bool { return false }) != nil {
			t.Fatalf("%s: answered though the user gave up", stage)
		}
		if uiaAnswer(user, resp, "", nil) != nil {
			t.Fatalf("%s: answered with no way to ask", stage)
		}
		// Params without a page can't be approved.
		resp.Params[stage] = map[string]any{}
		if uiaAnswer(user, resp, "", func(string) bool { return true }) != nil {
			t.Fatalf("%s: answered with no page to approve", stage)
		}
	}

	// A server that asks for nothing; one that asks for what we cannot do.
	if a, ok := uiaAnswer(user, uia([]mautrix.AuthType{mautrix.AuthTypeDummy}), "", nil).(*mautrix.BaseAuthData); !ok || a.Type != mautrix.AuthTypeDummy {
		t.Fatalf("dummy answer = %+v", a)
	}
	if uiaAnswer(user, uia([]mautrix.AuthType{mautrix.AuthTypeReCAPTCHA}), "pw", nil) != nil || uiaAnswer(user, nil, "pw", nil) != nil {
		t.Fatal("answered what it cannot")
	}
}

// machineSource is a Source with a crypto machine over an in-memory store.
func machineSource(t *testing.T, url string) *Source {
	t.Helper()
	s, _ := testSynced()
	cli, _ := mautrix.NewClient(url, "@me:example.org", "token")
	cli.DeviceID = "DEV"
	s.cli = cli
	mach := crypto.NewOlmMachine(cli, nil, crypto.NewMemoryStore(nil), nil)
	s.verify = &VerifyDriver{src: s, cli: cli, mach: mach}
	return s
}

// A file of room keys locks with a passphrase, opens only with it, and says so
// for a wrong one or a file that is not one.
func TestExportAndImportRoomKeys(t *testing.T) {
	s := machineSource(t, "http://127.0.0.1:1")
	type exported struct {
		data  []byte
		count int
		why   string
	}
	exp := make(chan exported, 1)
	s.ExportRoomKeys("", func(d []byte, n int, why string) { exp <- exported{d, n, why} })
	if e := <-exp; e.why == "" || e.data != nil {
		t.Fatalf("exported with no passphrase: %+v", e)
	}
	s.ExportRoomKeys("a good passphrase", func(d []byte, n int, why string) { exp <- exported{d, n, why} })
	waitFor(t, "the export", func() bool { return len(exp) == 1 })
	if e := <-exp; e.why == "" || !strings.Contains(e.why, "no room keys") {
		t.Fatalf("exported with nothing to export: %+v", e)
	}

	// A device that holds one key exports it...
	out, err := crypto.NewOutboundGroupSession("!room:example.org", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	in, err := crypto.NewInboundGroupSession("senderkey", "signingkey", "!room:example.org", out.Internal.Key(), 0, 0, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.verify.mach.CryptoStore.PutGroupSession(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	s.ExportRoomKeys("a good passphrase", func(d []byte, n int, why string) { exp <- exported{d, n, why} })
	waitFor(t, "the export", func() bool { return len(exp) == 1 })
	e := <-exp
	if e.why != "" || !strings.HasPrefix(string(e.data), "-----BEGIN MEGOLM SESSION DATA-----") || e.count != 1 {
		t.Fatalf("export = %q… count %d why %q", string(e.data[:min(40, len(e.data))]), e.count, e.why)
	}

	// ...and another one reads it, and can then decrypt for that room.
	other := machineSource(t, "http://127.0.0.1:1")
	done := make(chan [3]any, 1)
	other.ImportRoomKeys("a good passphrase", e.data, func(n, total int, why string) { done <- [3]any{n, total, why} })
	waitFor(t, "the import", func() bool { return len(done) == 1 })
	if r := <-done; r[0] != 1 || r[1] != 1 || r[2] != "" {
		t.Fatalf("import into another device: %v", r)
	}
	if got, err := other.verify.mach.CryptoStore.GetGroupSession(context.Background(), "!room:example.org", in.ID()); err != nil || got == nil {
		t.Fatalf("the key is not in the other device's store: %v", err)
	}

	type imported struct {
		n, total int
		why      string
	}
	imp := make(chan imported, 1)
	try := func(pass string, data []byte) imported {
		s.ImportRoomKeys(pass, data, func(n, total int, why string) { imp <- imported{n, total, why} })
		waitFor(t, "the import", func() bool { return len(imp) == 1 })
		return <-imp
	}
	if r := try("a good passphrase", e.data); r.why != "" {
		t.Fatalf("import with the right passphrase: %+v", r)
	}
	if r := try("a wrong passphrase", e.data); !strings.Contains(r.why, "passphrase") {
		t.Fatalf("import with a wrong passphrase: %+v", r)
	}
	if r := try("x", []byte("just some text")); !strings.Contains(r.why, "not a file of room keys") {
		t.Fatalf("import of a text file: %+v", r)
	}
}

// With no encryption set up (the mock, before the sync) these say so.
func TestKeysWithoutEncryption(t *testing.T) {
	s, _ := testSynced()
	got := make(chan string, 3)
	s.ExportRoomKeys("p", func(_ []byte, _ int, why string) { got <- why })
	s.ImportRoomKeys("p", nil, func(_, _ int, why string) { got <- why })
	s.ResetIdentity(model.IdentityReset{}, func(_, why string) { got <- why })
	for i := 0; i < 3; i++ {
		if why := <-got; why == "" {
			t.Fatal("an action went ahead with no encryption")
		}
	}
	s.RefreshKeyStatus()
	if st := s.KeyStatus(); !st.Asked || st.Problem == "" {
		t.Fatalf("status = %+v", st)
	}
}

// ownDeviceKeysResponse is a /keys/query answer holding the source's own
// device, signed by its own key as a homeserver stores it.
func ownDeviceKeysResponse(t *testing.T, s *Source) []byte {
	t.Helper()
	mach := s.verify.mach
	signing, identity := mach.GetAccount().Keys()
	dk := mautrix.DeviceKeys{
		UserID:     "@me:example.org",
		DeviceID:   "DEV",
		Algorithms: []id.Algorithm{id.AlgorithmMegolmV1, id.AlgorithmOlmV1},
		Keys: mautrix.KeyMap{
			id.NewDeviceKeyID(id.KeyAlgorithmCurve25519, "DEV"): identity.String(),
			id.NewDeviceKeyID(id.KeyAlgorithmEd25519, "DEV"):    signing.String(),
		},
	}
	sig, err := mach.GetAccount().SignJSON(dk)
	if err != nil {
		t.Fatal(err)
	}
	dk.Signatures = signatures.NewSingleSignature("@me:example.org", id.KeyAlgorithmEd25519, "DEV", sig)
	b, err := json.Marshal(map[string]any{
		"device_keys": map[string]any{"@me:example.org": map[string]any{"DEV": dk}},
		"failures":    map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Resetting the identity against a homeserver that asks to prove it: the
// keys go up once the proof is given, this device signs the new identity, and
// the new Security Key comes back. A refusal is said, and nothing is kept.
func TestResetIdentity(t *testing.T) {
	for name, tc := range map[string]struct {
		flow      mautrix.AuthType
		password  string
		approve   bool
		wantKey   bool
		wantOpens string
	}{
		"password":              {mautrix.AuthTypePassword, "pw", false, true, ""},
		"wrong password":        {mautrix.AuthTypePassword, "bad", false, false, ""},
		"approved in a browser": {mautrix.AuthTypeOAuth, "", true, true, "https://account.example.org/reset"},
		"gave up in a browser":  {mautrix.AuthTypeOAuth, "", false, false, "https://account.example.org/reset"},
	} {
		var s *Source // the machine the fake homeserver answers for
		var mu sync.Mutex
		var uploads, signatures int
		var proof map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case strings.Contains(r.URL.Path, "/account_data/"):
				_, _ = w.Write([]byte(`{}`))
			case strings.HasSuffix(r.URL.Path, "/keys/device_signing/upload"):
				var req map[string]any
				_ = json.Unmarshal(body, &req)
				auth, _ := req["auth"].(map[string]any)
				if auth == nil {
					w.WriteHeader(http.StatusUnauthorized)
					params := `{}`
					if tc.flow == mautrix.AuthTypeOAuth {
						params = `{"m.oauth":{"url":"https://account.example.org/reset"}}`
					}
					_, _ = w.Write([]byte(`{"flows":[{"stages":["` + string(tc.flow) + `"]}],"params":` + params + `,"session":"s1"}`))
					return
				}
				proof = auth
				if auth["password"] == "bad" {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"errcode":"M_FORBIDDEN","error":"Invalid password"}`))
					return
				}
				uploads++
				_, _ = w.Write([]byte(`{}`))
			case strings.HasSuffix(r.URL.Path, "/keys/query"):
				// This device, as a homeserver would hand it back, signed.
				_, _ = w.Write(ownDeviceKeysResponse(t, s))
			case strings.HasSuffix(r.URL.Path, "/keys/signatures/upload"):
				signatures++
				_, _ = w.Write([]byte(`{"failures":{}}`))
			default:
				w.WriteHeader(http.StatusNotFound) // e.g. the key backup, asked after the reset
				_, _ = w.Write([]byte(`{"errcode":"M_NOT_FOUND","error":"no"}`))
			}
		}))
		s = machineSource(t, srv.URL)
		if err := s.verify.mach.Load(context.Background()); err != nil {
			t.Fatal(err)
		}
		var opened string
		result := make(chan [2]string, 1)
		s.ResetIdentity(model.IdentityReset{Password: tc.password, Approve: func(url string) bool { opened = url; return tc.approve }},
			func(key, why string) { result <- [2]string{key, why} })
		waitFor(t, name+": the reset to finish", func() bool { return len(result) == 1 })
		r := <-result
		mu.Lock()
		if tc.wantKey {
			if r[1] != "" || len(r[0]) < 40 || uploads != 1 || signatures < 2 {
				t.Errorf("%s: key %q why %q, uploads %d, signatures %d", name, r[0], r[1], uploads, signatures)
			}
		} else if r[1] == "" || r[0] != "" || uploads != 0 {
			t.Errorf("%s: key %q why %q, uploads %d: a refused reset went through", name, r[0], r[1], uploads)
		}
		if opened != tc.wantOpens {
			t.Errorf("%s: opened %q, want %q", name, opened, tc.wantOpens)
		}
		if tc.password == "pw" && proof["password"] != "pw" {
			t.Errorf("%s: the password was not sent: %v", name, proof)
		}
		mu.Unlock()
		srv.Close()
	}
}

// The status says whether the account has a backup and how many keys it
// holds; no backup is not an error.
func TestKeyStatus(t *testing.T) {
	backup := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/room_keys/version"):
			if !backup {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errcode":"M_NOT_FOUND","error":"No backup found"}`))
				return
			}
			_, _ = w.Write([]byte(`{"algorithm":"m.megolm_backup.v1.curve25519-aes-sha2","auth_data":{},"count":42,"etag":"1","version":"7"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errcode":"M_NOT_FOUND","error":"no"}`))
		}
	}))
	defer srv.Close()
	s := machineSource(t, srv.URL)
	s.RefreshKeyStatus()
	waitFor(t, "the status", func() bool { return s.KeyStatus().BackupVersion != "" })
	if st := s.KeyStatus(); !st.Asked || st.BackupVersion != "7" || st.BackupKeys != 42 || st.HaveBackupKey || st.SecretStorage || st.Problem != "" {
		t.Fatalf("with a backup: %+v", st)
	}
	backup = false
	s.keyStatus.BackupVersion = "stale"
	s.RefreshKeyStatus()
	waitFor(t, "the status without a backup", func() bool { return s.KeyStatus().BackupVersion == "" })
	if st := s.KeyStatus(); st.Problem != "" || !st.Asked {
		t.Fatalf("without a backup: %+v", st)
	}
	s.verify.keyed.Store(true)
	s.RefreshKeyStatus()
	waitFor(t, "this device having the key", func() bool { return s.KeyStatus().HaveBackupKey })
}
