// slopement is a minimal Matrix client drawn with MyGo's native UI.
//
// The window shows the app (package app) on mock data (package
// model/mock); SLOPEMENT_LOGIN=1 boots the sign-in screen (package
// login) over the Matrix backend instead, and the app on the signed-in
// account once it succeeds. The design kit's gallery (package gallery)
// opens from Debug → Component Gallery, in a window of its own.
package main

import (
	"log"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/app"
	"slopement/gallery"
	"slopement/internal/dotenv"
	"slopement/login"
	"slopement/matrix"
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

	// The window's content: the sign-in screen in login mode until a
	// session exists, then the app on the account; the mock boots straight
	// into the app. SLOPEMENT_LOGIN=1 signs in.
	var a *app.App
	var lg *login.Login
	var signed bool
	root := func(c *ui.Context) {
		if lg == nil || signed {
			a.View(c)
		} else {
			lg.View(c)
		}
	}

	if os.Getenv("SLOPEMENT_LOGIN") == "1" {
		lg = login.New()
	} else {
		a = app.New(mock.New(time.Now()), pick)
	}

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
			Content:         ui.View(root),
		}))

		setMenu := func() { mygo.App.SetMenu(platform.MenuBar(a.Actions(), win.Update)) }

		// Debug → Component Gallery: the kit's gallery, in its own window
		// (one at a time).
		var gwin *mygo.Window
		openGallery := func() {
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

		if lg != nil {
			lg.SetUpdate(win.Update)
			lg.OnSignedIn = func(s *matrix.Session) {
				// A fresh app on the account: before sync it shows the
				// signed-in user and the app's empty states.
				a = app.New(matrix.NewSource(s), pick)
				a.OpenGallery = openGallery
				a.Attach(win)
				a.MenuChanged = setMenu
				signed = true
				setMenu()
			}
		} else {
			a.OpenGallery = openGallery
			a.Attach(win)
			a.MenuChanged = setMenu
			setMenu()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
