// slopement is a minimal Matrix client drawn with MyGo's native UI.
//
// The window shows the app (package app) on mock data (package
// model/mock) until the Matrix layer lands. The design kit's gallery
// (package gallery) opens from Debug → Component Gallery, in a window of
// its own.
package main

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/app"
	"slopement/gallery"
	"slopement/internal/dotenv"
	"slopement/model/mock"
	"slopement/picker"
	"slopement/platform"
)

func main() {
	// KLIPY_API_KEY and other local secrets, from .env (gitignored).
	if err := dotenv.Load(); err != nil {
		log.Printf("reading .env: %v", err)
	}
	platform.Install()
	pick := picker.NewWithKey()
	a := app.New(mock.New(time.Now()), pick)
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
			Content:         ui.View(a.View),
		}))
		a.Attach(win)

		// Debug → Component Gallery: the kit's gallery, in its own window
		// (one at a time).
		var gwin *mygo.Window
		a.OpenGallery = func() {
			if gwin != nil {
				gwin.Focus()
				return
			}
			g := gallery.NewWith(pick)
			gwin = mygo.NewWindow(platform.Window(mygo.WindowOptions{
				Title: "slopement · Component Gallery", Width: 1280, Height: 860, MinWidth: 360, MinHeight: 480,
				BackgroundColor: "light-dark(#FCFCFD, #121116)", StateKey: "gallery",
				Content: ui.View(g.View),
			}))
			g.Attach(gwin)
			gwin.OnClose(func(*mygo.CloseEvent) { gwin = nil })
		}

		setMenu := func() { mygo.App.SetMenu(platform.MenuBar(a.Actions(), win.Update)) }
		a.MenuChanged = setMenu
		setMenu()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
