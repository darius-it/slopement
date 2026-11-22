// slopement is a minimal Matrix client drawn with MyGo's native UI.
//
// The window shows the sign-in screen (package login) over the Matrix
// backend, then the app (package app) on the signed-in account. For
// interface work only, SLOPEMENT_MOCK=1 boots the app on fake data (package
// model/mock) instead. The design kit's gallery (package gallery) opens
// from Debug → Component Gallery, in a window of its own.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
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

// live is the account on screen: its source (the sync), its stores, and the
// cancel that ends the sync.
type live struct {
	src    *matrix.Source
	acc    *matrix.Account
	cancel context.CancelFunc
	user   string
}

func main() {
	// KLIPY_API_KEY and other local secrets, from .env (gitignored).
	if err := dotenv.Load(); err != nil {
		log.Printf("reading .env: %v", err)
	}
	platform.Install()
	startLogging()
	pick := picker.NewWithKey()

	// The window's content: the sign-in screen in login mode until a
	// session exists, then the app on the account.
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

	// The app signs in. The fake account (model/mock) is for developing the
	// interface only, and only with SLOPEMENT_MOCK=1: a build never starts on it.
	if os.Getenv("SLOPEMENT_MOCK") == "1" {
		a = app.New(mock.New(time.Now()), pick)
	} else {
		lg = login.New()
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

		// A click on a desktop notification brings the window forward on the
		// room it was about (its ID).
		mygo.App.OnNotificationClick(func(id string) {
			win.Update(func() {
				if a != nil && (lg == nil || signed) {
					a.OpenFromNotification(id)
				}
				win.Focus()
			})
		})

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

			// The account on screen, if any: what is stopped to switch.
			var cur *live
			var entries []app.AccountEntry // the menu's list, kept so it is not read from disk per frame
			refresh := func() {
				entries = entries[:0]
				sessions, _ := matrix.Sessions()
				for _, s := range sessions {
					entries = append(entries, app.AccountEntry{
						User: string(s.UserID), Server: s.ServerName,
						Current: cur != nil && cur.user == string(s.UserID),
					})
				}
				sort.Slice(entries, func(i, j int) bool { return entries[i].User < entries[j].User })
			}
			// stop puts the account on screen away: its sync ends, what it
			// learned is on disk, its stores and lock are released.
			stop := func() {
				if cur == nil {
					return
				}
				cur.cancel()
				cur.src.Flush()
				a.SaveState()
				a.Detach()
				cur.acc.Close()
				cur = nil
			}
			var openAccount func(s *matrix.Session)
			find := func(user string) *matrix.Session {
				sessions, _ := matrix.Sessions()
				for _, s := range sessions {
					if string(s.UserID) == user {
						return s
					}
				}
				return nil
			}
			accounts := &app.Accounts{
				List: func() []app.AccountEntry { return entries },
				Switch: func(user string) {
					if s := find(user); s != nil {
						openAccount(s)
					}
				},
				Add: func() {
					// The sign-in over the running app, with a way back.
					lg.BeginAdd(func() { signed = cur != nil })
					signed = false
				},
				Remove: func(user string) {
					s := find(user)
					if s == nil {
						return
					}
					wasCurrent := cur != nil && cur.user == user
					if wasCurrent {
						stop()
					}
					if err := s.Delete(); err != nil {
						log.Printf("removing %s: %v", user, err)
					}
					refresh()
					if wasCurrent {
						// On to another account if there is one, else the sign-in.
						lg.EndAdd()
						signed = false
						lg.Reload()
						lg.Resume()
					}
				},
			}

			// openAccount shows an account: the stores, the client and the
			// sync loop are opened off the UI thread, then the window swaps
			// to the app on the account, with the loop running.
			openAccount = func(s *matrix.Session) {
				lg.EndAdd()
				stop()
				signed = false
				go func() {
					ctx := context.Background()
					acc, err := matrix.Open(ctx, s.UserID, s.DeviceID)
					if err != nil {
						if errors.Is(err, matrix.ErrLocked) {
							lg.Fail("This account is already open in another slopement window. Quit that one and sign in here again.")
							return
						}
						lg.Fail("Opening the account failed: " + err.Error())
						return
					}
					cli, err := matrix.Dial(ctx, s)
					if err != nil {
						acc.Close()
						lg.Fail("Reaching the server failed: " + err.Error())
						return
					}
					src, err := matrix.NewSynced(ctx, acc, cli)
					if err != nil {
						acc.Close()
						lg.Fail("Preparing the sync failed: " + err.Error())
						return
					}
					win.Update(func() {
						a = app.New(src, pick)
						// The app's own state (layout, settings, where the
						// user was, drafts) comes back with the account.
						if dir, err := matrix.AccountDir(string(s.UserID)); err == nil {
							a.SetStatePath(filepath.Join(dir, "ui.json"))
						}
						a.OpenGallery = openGallery
						a.Attach(win)
						a.MenuChanged = setMenu
						// The next launch opens this account on its own.
						_ = matrix.SetLastUsed(string(s.UserID))
						syncCtx, cancel := context.WithCancel(context.Background())
						cur = &live{src: src, acc: acc, cancel: cancel, user: string(s.UserID)}
						refresh()
						a.SetAccounts(accounts)
						lg.Reload()
						signed = true
						setMenu()
						src.Start(syncCtx)
					})
				}()
			}
			lg.OnSignedIn = openAccount
			// What the sync learned is on disk before the window goes (the
			// cache also writes a second after every change, so a crash
			// loses at most that).
			win.OnClose(func(*mygo.CloseEvent) {
				if cur != nil {
					cur.src.Flush()
					a.SaveState()
				}
			})
			// The account used last opens on its own: the sign-in shows only
			// when there is none, or it cannot be opened.
			lg.Resume()
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
