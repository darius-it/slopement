package matrix

import "errors"

// lockName is the session lock file in an account's directory.
const lockName = "session.lock"

// ErrLocked is returned when another slopement process holds the
// account's session lock: one process per account, as the second launch
// hands its arguments to the running one and focuses it instead.
var ErrLocked = errors.New("another slopement session holds this account")
