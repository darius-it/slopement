package matrix

import (
	"context"
	"testing"
)

// TestMigrateCryptoAccount pins the one-time move of the older builds'
// crypto rows — keyed by the user alone — to the per-device key, with
// the half-created row of a failed boot cleared first.
func TestMigrateCryptoAccount(t *testing.T) {
	openTest(t)
	ctx := context.Background()

	a, err := Open(ctx, testUser, testDevice)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	db := a.cryptoDB

	// The older build's row: keyed by the user alone, marked shared, its
	// device the same as this session's.
	oldRow := func(accountID string, shared bool) {
		if _, err := db.Exec(ctx,
			`INSERT INTO crypto_account (account_id, device_id, shared, sync_token, account)
			 VALUES ($1, $2, $3, 'tok', x'0102')`,
			accountID, testDevice, shared); err != nil {
			t.Fatal(err)
		}
	}
	oldRow(testUser, true)
	if _, err := db.Exec(ctx,
		`INSERT INTO crypto_megolm_inbound_session (account_id, room_id, session_id, sender_key)
		 VALUES ($1, '!r', '$s', 'K')`, testUser); err != nil {
		t.Fatal(err)
	}
	// And the half-created per-device row of a boot that failed the
	// server-keys guard.
	oldRow(testUser+"/"+testDevice, false)

	if err := migrateCryptoAccount(ctx, db, testUser, testUser+"/"+testDevice); err != nil {
		t.Fatal(err)
	}
	count := func(accountID string) int {
		var n int
		if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM crypto_account WHERE account_id=$1", accountID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count(testUser) != 0 {
		t.Error("the old user-keyed row is still there")
	}
	if n := count(testUser + "/" + testDevice); n != 1 {
		t.Fatalf("the per-device row = %d, want 1", n)
	}
	// The session moved with it; the account row is the shared one.
	var shared bool
	if err := db.QueryRow(ctx, "SELECT shared FROM crypto_account WHERE account_id=$1", testUser+"/"+testDevice).Scan(&shared); err != nil {
		t.Fatal(err)
	}
	if !shared {
		t.Error("the moved row lost its shared mark")
	}
	var sessions int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM crypto_megolm_inbound_session WHERE account_id=$1", testUser+"/"+testDevice).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 {
		t.Errorf("the moved row's sessions = %d, want 1", sessions)
	}

	// A second run is quiet: nothing is left to move, nothing half-made.
	if err := migrateCryptoAccount(ctx, db, testUser, testUser+"/"+testDevice); err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if n := count(testUser + "/" + testDevice); n != 1 {
		t.Fatalf("the second run changed the rows: %d", n)
	}
}
