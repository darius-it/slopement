package login

import (
	"os"
	"testing"

	"slopement/matrix"
)

// The tests sign in to fake servers: their accounts and tokens must never
// land among the user's real ones.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "slopement-login-test")
	if err != nil {
		panic(err)
	}
	restore := matrix.Isolate(dir)
	code := m.Run()
	restore()
	os.RemoveAll(dir)
	os.Exit(code)
}
