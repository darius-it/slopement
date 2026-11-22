package app

import (
	"github.com/egoist/mygo"
)

// The system's file dialogs, behind two functions the app calls (and tests
// replace): they run the native dialog off the view and hand the chosen path
// back on the UI thread; nothing chosen, nothing called.

// pickFile asks for a file to read.
func (a *App) pickFile(title, ext string, done func(path string)) {
	if a.pickFileFn != nil {
		a.pickFileFn(title, ext, done)
		return
	}
	if a.win == nil || a.runUI == nil {
		return
	}
	win, runUI := a.win, a.runUI
	go func() {
		opts := mygo.OpenDialogOptions{Parent: win, Title: title, ButtonLabel: "Choose"}
		if ext != "" {
			opts.Filters = []mygo.FileFilter{{Name: ext + " files", Extensions: []string{ext}}}
		}
		paths, err := mygo.Dialog.Open(opts)
		if err != nil || len(paths) == 0 {
			return
		}
		runUI(func() { done(paths[0]) })
	}()
}

// saveFile asks where to save a file named name.
func (a *App) saveFile(title, name string, done func(path string)) {
	if a.saveFileFn != nil {
		a.saveFileFn(title, name, done)
		return
	}
	if a.win == nil || a.runUI == nil {
		return
	}
	win, runUI := a.win, a.runUI
	go func() {
		to, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: win, Title: title, DefaultPath: name})
		if err != nil || to == "" {
			return
		}
		runUI(func() { done(to) })
	}()
}
