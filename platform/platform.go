// Package platform fits the app into the desktop it runs on, so it never
// looks out of place, while the content keeps the kit's design language:
//
//   - the window: a hidden title bar whose native controls sit over the
//     app's own chrome (traffic lights over the sidebar on macOS, the
//     caption buttons over the toolbar on Windows, GTK's on Linux), and the
//     system's material behind the sidebar (macOS's sidebar vibrancy,
//     Windows 11's Mica);
//   - the menu bar: the system's, with the standard items and shortcuts
//     (macOS's app menu, Services, ⌘Q; Edit's roles, which text fields
//     rely on), hidden until Alt on Windows and Linux as modern apps do;
//   - menus: the system's context and pop-up menus on desktops (see
//     kit.NativeMenus), the kit's drawn ones on phones;
//   - Linux: GTK's window buttons restyled as libadwaita's, round and flat.
//
// What stays the kit's everywhere: the content, the purple accent, the
// type scale, bubbles, cards and motion.
package platform

import (
	"log"
	"runtime"

	"github.com/egoist/mygo"
)

// OS is the platform family.
type OS int

const (
	MacOS OS = iota
	Windows
	Linux
	Phone
)

// Current returns the platform the app runs on.
func Current() OS {
	switch runtime.GOOS {
	case "darwin":
		return MacOS
	case "windows":
		return Windows
	case "ios", "android":
		return Phone
	}
	return Linux
}

// Window returns o with the platform's chrome: a hidden title bar over the
// app's own (desktops), and the sidebar material where the system has one.
func Window(o mygo.WindowOptions) mygo.WindowOptions {
	switch Current() {
	case MacOS:
		o.TitleBarStyle = mygo.TitleBarHidden
		o.Vibrancy = mygo.VibrancySidebar
		// The traffic lights, lined up with the rail's first item.
		o.TrafficLightPosition = &mygo.Point{X: 20, Y: 20}
	case Windows:
		o.TitleBarStyle = mygo.TitleBarHidden
		o.Vibrancy = mygo.VibrancyMica
		o.AutoHideMenuBar = true
	case Linux:
		o.TitleBarStyle = mygo.TitleBarHidden
		o.AutoHideMenuBar = true
	}
	return o
}

// adwaitaButtons restyles GTK's window buttons as libadwaita's: round,
// flat, transparent until hovered, so they sit on the kit's surfaces in
// either theme.
const adwaitaButtons = `
decoration { border-radius: 12px; }
.maximized decoration, .fullscreen decoration, .tiled decoration { border-radius: 0; }
button.titlebutton {
  min-width: 24px; min-height: 24px; margin: 0 4px; padding: 0;
  border: none; border-radius: 9999px; box-shadow: none;
  background-color: alpha(currentColor, 0.1); background-image: none;
}
button.titlebutton:hover { background-color: alpha(currentColor, 0.15); }
button.titlebutton:active { background-color: alpha(currentColor, 0.3); }
`

// Install applies what is app-wide rather than per window: GTK's style on
// Linux. Call it before App.Run.
func Install() {
	if Current() != Linux {
		return
	}
	if err := mygo.Theme.SetGTKCSS(adwaitaButtons); err != nil {
		log.Printf("platform: GTK style: %v", err)
	}
}
