# Compatibility fixtures

One raw Matrix event per file, in the shape the named client (or bridge) sends.
`matrix/fixtures_test.go` converts each through `matrix.ConvertEvent`, writes a
text dump and compares it with the `.golden` file beside it; `app/fixtures_test.go`
renders each in the real timeline and fails if a row panics.

These were **written from the event shapes each client documents or is known
to send**, not captured from live accounts: names and IDs are made up
(`@ada:example.org`, `!fix:example.org`). When a real event from any client
renders wrongly, add it here (anonymized) as a new fixture, with the `_from`
field saying where it came from, and fix the bug.

`_from` and `_note` are not Matrix fields; Matrix ignores unknown ones.

After a deliberate change, `UPDATE_GOLDEN=1 go test -tags goolm ./matrix/ -run
FixturesGolden -v` prints each new dump (a test process cannot write project
files in the dev sandbox): save each block as `<name>.golden` and read the diff.
