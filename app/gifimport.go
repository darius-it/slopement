package app

import (
	"os"
	"strconv"

	"github.com/egoist/mygo"
)

// importDiscordGIFs asks for a discord-gif-exporter JSON file and adds its
// GIFs to the favorites. The dialog is the system's, so it runs off the view.
func (a *App) importDiscordGIFs() {
	if a.win == nil || a.pick == nil {
		return // no window (tests)
	}
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "Import favorite GIFs from Discord",
			Message: "Choose the discord-favorite-gifs.json file from discord-gif-exporter.", ButtonLabel: "Import",
			Filters: []mygo.FileFilter{{Name: "Discord GIF export", Extensions: []string{"json"}}}})
		if err != nil || len(paths) == 0 {
			return
		}
		a.importDiscordFile(paths[0])
	}()
}

// importDiscordFile reads the export at path (off the UI thread) and says
// what came of it as a toast.
func (a *App) importDiscordFile(path string) {
	runUI := a.runUI
	if runUI == nil || a.pick == nil {
		return
	}
	go func() {
		data, err := os.ReadFile(path)
		runUI(func() {
			n := 0
			if err == nil {
				n, err = a.pick.ImportDiscordFavorites(data)
			}
			switch {
			case err != nil:
				a.toast = "Import failed: " + err.Error()
			case n == 0:
				a.toast = "No new GIFs to import"
			default:
				a.toast = "Imported " + strconv.Itoa(n) + " GIFs from Discord"
			}
		})
	}()
}
