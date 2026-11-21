// Package matrix is slopement's Matrix layer: the client, its stores and
// the sync loop behind model.Source. The app never talks to a homeserver
// through anything else.
//
// The design (docs/requirements.md):
//   - mautrix-go with the goolm build tag: pure Go, no cgo, one codebase
//     for every platform. Always build with -tags goolm.
//   - Multi-account from day one: each login gets its own directory
//     <config>/slopement/accounts/<sha256(user_id)>/ holding state.db and
//     crypto.db; nothing is global.
//   - One process per account, enforced by a session lock on the account
//     directory.
//   - Access tokens (and the crypto pickle key) live in the OS keychain
//     through mygo's Secrets, keyed by user ID and device ID. The
//     databases never hold a token.
package matrix
