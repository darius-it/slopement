package app

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"slopement/model"
)

// The user's own profile and colors, on a source that can change them
// (model.Profile, model.UserColors): the mock has neither, and the
// settings say what is not possible there.

// profileDone reports a profile change's result as a toast: nothing on
// success, why on a failure.
func (a *App) profileDone(ok string) func(err string) {
	return func(err string) {
		switch {
		case err != "":
			a.toast = err
		case ok != "":
			a.toast = ok
		}
	}
}

// pickPicture opens the file dialog for a picture, off the view, and calls
// then (on the UI thread) with the file chosen.
func (a *App) pickPicture(title string, then func(model.File)) {
	if a.win == nil || a.runUI == nil {
		return
	}
	win, runUI := a.win, a.runUI
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win, Title: title, ButtonLabel: "Use picture",
			Filters: []mygo.FileFilter{{Name: "Pictures", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		info, err := os.Stat(paths[0])
		if err != nil || info.Size() > 20<<20 {
			runUI(func() { a.toast = "Choose a picture under 20 MB" })
			return
		}
		data, err := os.ReadFile(paths[0])
		if err != nil {
			runUI(func() { a.toast = "Could not read " + filepath.Base(paths[0]) })
			return
		}
		runUI(func() { then(model.File{Name: filepath.Base(paths[0]), Data: data}) })
	}()
}

// pickAvatar chooses a new profile picture and sets it.
func (a *App) pickAvatar() {
	prof, ok := a.src.(model.Profile)
	if !ok {
		a.toast = "This account can't change its picture here"
		return
	}
	a.pickPicture("Choose a profile picture", func(f model.File) {
		a.toast = "Setting your picture…"
		prof.SetAvatar(f, a.profileDone("Profile picture changed"))
	})
}

// removeAvatar takes the profile picture away.
func (a *App) removeAvatar() {
	if prof, ok := a.src.(model.Profile); ok {
		prof.RemoveAvatar(a.profileDone("Profile picture removed"))
	}
}

// saveName changes the display name to what was typed.
func (a *App) saveName() {
	prof, ok := a.src.(model.Profile)
	if !ok {
		a.toast = "This account can't change its name here"
		return
	}
	prof.SetDisplayName(a.nameDraft, a.profileDone("Name changed"))
}

// setIgnore ignores someone, or stops ignoring them. It is a state, not a
// toggle: asked twice it does the same.
func (a *App) setIgnore(person string, on bool) {
	ig, ok := a.src.(model.Ignorer)
	if !ok {
		a.toast = "This account can't ignore people here"
		return
	}
	if ig.IsIgnored(person) == on {
		return
	}
	p, _ := a.src.Person(person)
	ig.SetIgnored(person, on, func(err string) {
		switch {
		case err != "":
			a.toast = err
		case on:
			a.toast = "Ignoring " + p.Name + ": their messages are hidden"
		default:
			a.toast = "No longer ignoring " + p.Name
		}
	})
}

// syncColors takes the colors the account has (every session's) into the
// view's resolver. Called each frame: it only points at the source's map.
func (a *App) syncColors() {
	if uc, ok := a.src.(model.UserColors); ok {
		a.colors.Overrides = uc.UserColors()
	}
}

// chooseColor gives a name a color (an index of the palette; -1 for the
// automatic one): kept with the account where the source can, else here.
func (a *App) chooseColor(mxid string, index int) {
	if uc, ok := a.src.(model.UserColors); ok {
		uc.SetUserColor(mxid, index)
		return
	}
	a.colors.Set(mxid, index)
}

// colorOf is the palette index chosen for a name, -1 for automatic.
func (a *App) colorOf(mxid string) int {
	if i, ok := a.colors.Overrides[mxid]; ok {
		return i
	}
	return -1
}
