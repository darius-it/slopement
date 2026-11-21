// slopement is a minimal Matrix client drawn with MyGo's native UI.
//
// For now the window shows the design kit's gallery (package gallery),
// built from package kit alone, in the platform's chrome (package
// platform): its menu bar, title bar and sidebar material.
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/gallery"
	"slopement/internal/dotenv"
	"slopement/platform"
)

func main() {
	// KLIPY_API_KEY and other local secrets, from .env (gitignored).
	if err := dotenv.Load(); err != nil {
		log.Printf("reading .env: %v", err)
	}
	platform.Install()
	g := gallery.New()
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(platform.Window(mygo.WindowOptions{
			Title:     "slopement",
			Width:     1280,
			Height:    860,
			MinWidth:  360,
			MinHeight: 480,
			// The kit's Bg, light and dark, so the window opens without a flash.
			BackgroundColor: "light-dark(#FCFCFD, #121116)",
			StateKey:        "main",
			Content:         ui.View(g.View),
		}))
		g.Attach(win)
		setMenu := func() { mygo.App.SetMenu(platform.MenuBar(g.Actions(), win.Update)) }
		g.MenuChanged = setMenu
		setMenu()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
