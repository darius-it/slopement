package licenses

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed notices.json
var noticesJSON []byte

var (
	once sync.Once
	all  []Notice
)

// All is the open source software slopement is built from (a Go module each),
// by name; internal/licenses/gen writes it from the real build.
func All() []Notice {
	once.Do(func() { _ = json.Unmarshal(noticesJSON, &all) })
	return all
}

// Credit is something slopement uses that is not a Go module in the build: a
// design, a service, data it shows or loads from elsewhere.
type Credit struct {
	Name, License, URL, Note string
}

// Credits are what the generator cannot find in go.mod. Keep it honest: each
// is something the app really uses.
var Credits = []Credit{
	{"Lucide", "ISC", "https://lucide.dev", "The icons (a subset, drawn at a lighter stroke)."},
	{"Element Call", "AGPL-3.0 or Element's commercial license", "https://github.com/element-hq/element-call",
		"Calls run in Element Call, loaded from call.element.io while you are in a call. It is not part of slopement and is not distributed with it."},
	{"KLIPY", "Their terms of service", "https://klipy.com", "GIF and sticker search, with your own key."},
	{"OpenStreetMap", "ODbL", "https://www.openstreetmap.org/copyright", "Shared locations open in OpenStreetMap, © OpenStreetMap contributors."},
	{"Matrix", "Apache-2.0 (the specification)", "https://matrix.org", "The open protocol slopement speaks."},
}
