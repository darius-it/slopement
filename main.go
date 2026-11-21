// slopement is a minimal Matrix client drawn with MyGo's native UI.
//
// Step 1 of the plan: the window shows the design kit's gallery
// (package gallery), built from package kit alone.
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/gallery"
	"slopement/internal/dotenv"
)

func main() {
	// KLIPY_API_KEY and other local secrets, from .env (gitignored).
	if err := dotenv.Load(); err != nil {
		log.Printf("reading .env: %v", err)
	}
	g := gallery.New()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:     "slopement",
			Width:     1280,
			Height:    860,
			MinWidth:  360,
			MinHeight: 480,
			// The kit's Bg, light and dark, so the window opens without a flash.
			BackgroundColor: "light-dark(#FCFCFD, #121116)",
			StateKey:        "main",
			Content:         ui.View(g.View),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
