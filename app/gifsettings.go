package app

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// gifsPage is Settings → GIFs: the KLIPY key, the folders on this computer
// whose pictures the GIF picker offers beside KLIPY's (not on phones), and
// the favorites.
func (a *App) gifsPage(c *ui.Context) {
	a.klipyGroup(c)
	if !kit.PhoneOS() { // a phone's apps cannot read the user's folders
		a.gifFoldersGroup(c)
	}
	a.favoritesGroup(c)
}

// gifFoldersGroup is the list of folders and the button that adds one.
func (a *App) gifFoldersGroup(c *ui.Context) {
	p := kit.Of(c)
	group(c, "GIF folders", "Pictures in these folders (GIF, PNG, JPEG, WebP) show in the GIF picker under My folders, and in its search by file name. They are read from this device; sending one uploads it.", func() {
		for i, dir := range a.gifFolders {
			ui.Row(c.Key("gf" + strconv.Itoa(i))).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, kit.IconFolder).Size(18, 18).TextColor(p.TextMuted)
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, filepath.Base(dir)).FontSize(kit.SizeBody).FontWeight(kit.WeightMedium).SingleLine()
					ui.Text(c, dir).FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
				})
				if kit.IconButton(c, kit.Ghost, kit.IconX, "Remove "+filepath.Base(dir)).Clicked() {
					a.removeGIFFolder(dir)
				}
			})
		}
		if kit.ChoiceRow(c, kit.IconPlus, "Add a folder…", "").Clicked() {
			a.addGIFFolderDialog()
		}
		if n := a.pickCount(); len(a.gifFolders) > 0 {
			ui.Text(c, strconv.Itoa(n)+" pictures found. Looked at again when the picker opens.").
				FontSize(kit.SizeCaption).TextColor(p.TextFaint)
		}
	})
}

// favoritesGroup is Settings → GIFs → Favorites: importing from Discord,
// dropping what is no longer available, and starting over.
func (a *App) favoritesGroup(c *ui.Context) {
	if a.pick == nil {
		return
	}
	note := "Favorites imported from Discord have links that expire after a day, so they are copied to this device as they are imported (and at startup). Reset clears the favorites and those copies; import again right after, while the export's links still work."
	group(c, "Favorite GIFs", note, func() {
		if kit.ChoiceRow(c, kit.IconPlus, "Import from Discord…", "A discord-favorite-gifs.json from discord-gif-exporter").Clicked() {
			a.importDiscordGIFs()
		}
		if n := a.pick.UnavailableCount(); n > 0 {
			label := "Remove " + strconv.Itoa(n) + " unavailable favorite"
			if n != 1 {
				label += "s"
			}
			if kit.ChoiceRow(c, kit.IconX, label, "Their links expired or the picture was deleted").Clicked() {
				a.toast = "Removed " + strconv.Itoa(a.pick.RemoveUnavailable()) + " unavailable favorites"
			}
		}
		if kit.ChoiceRow(c, kit.IconTrash, "Reset favorite GIFs…", strconv.Itoa(a.pick.FavoriteCount())+" favorites, and their copies on this device").Clicked() {
			a.confirmResetFavorites()
		}
	})
}

// confirmResetFavorites asks, in the system's dialog, before clearing every
// favorite GIF.
func (a *App) confirmResetFavorites() {
	if a.win == nil || a.pick == nil {
		return
	}
	win := a.win
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: win, Title: "Reset favorite GIFs?",
			Message: "Every favorite GIF and the copies kept on this device are removed. Import them again afterwards.",
			Buttons: []string{"Reset", "Cancel"}, DefaultButton: 1, CancelButton: 1, DestructiveButtons: []int{0}})
		if err != nil || res.Button != 0 {
			return
		}
		win.Update(func() {
			n := a.pick.ResetFavorites()
			a.toast = "Removed " + strconv.Itoa(n) + " favorites. Import again from here."
		})
	}()
}

// klipyGroup is where the user gives their own KLIPY key, which GIF and
// sticker search needs. It stays on this device.
func (a *App) klipyGroup(c *ui.Context) {
	if a.pick == nil {
		return
	}
	p := kit.Of(c)
	note := "GIF and sticker search runs on KLIPY and needs an API key of your own, which you get from KLIPY (klipy.com). It is kept on this device only, and used only to ask KLIPY for GIFs."
	group(c, "KLIPY key", note, func() {
		ui.Column(c).Gap(kit.S2).Padding(kit.S2).Children(func() {
			if a.pick.KlipyKeyFromEnvironment() {
				ui.Text(c, "A key from this device's environment (KLIPY_API_KEY) is in use.").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
				return
			}
			// The saved key is never shown again: the field takes a new one.
			hint := "Paste your key"
			if a.pick.KlipyKey() != "" {
				hint = "A key is saved. Paste a new one to replace it"
			}
			kit.PasswordField(c.Key("klipy-key"), &a.klipyDraft, "Your KLIPY key", hint)
			ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
				changed := strings.TrimSpace(a.klipyDraft) != ""
				if kit.Button(c, kit.Primary, "Save", nil).Disabled(!changed).Clicked() {
					a.pick.SetKlipyKey(a.klipyDraft)
					a.klipyDraft = ""
				}
				if a.pick.KlipyKey() != "" && kit.Button(c, kit.Secondary, "Remove", nil).Clicked() {
					a.pick.SetKlipyKey("")
					a.klipyDraft = ""
				}
				switch {
				case changed:
				case a.pick.KlipyKey() != "":
					ui.Text(c, "Saved. GIF and sticker search is on.").FontSize(kit.SizeSmall).TextColor(p.Success)
				default:
					ui.Text(c, "No key: search is off. Favorites and folders still work.").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
				}
			})
		})
	})
}

func (a *App) pickCount() int {
	if a.pick == nil {
		return 0
	}
	return a.pick.LocalCount()
}

// addGIFFolderDialog asks for a folder with the system's dialog (it is
// native, so it runs off the view).
func (a *App) addGIFFolderDialog() {
	if a.win == nil || a.runUI == nil {
		return
	}
	win, runUI := a.win, a.runUI
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "Add a folder of GIFs",
			ButtonLabel: "Add", Directory: true})
		if err != nil || len(paths) == 0 {
			return
		}
		runUI(func() { a.addGIFFolder(paths[0]) })
	}()
}

// addGIFFolder adds a folder (once) and has the picker index it.
func (a *App) addGIFFolder(dir string) {
	dir = filepath.Clean(dir)
	if dir == "." || slices.Contains(a.gifFolders, dir) {
		return
	}
	a.gifFolders = append(slices.Clone(a.gifFolders), dir)
	a.syncGIFFolders()
}

func (a *App) removeGIFFolder(dir string) {
	a.gifFolders = slices.DeleteFunc(slices.Clone(a.gifFolders), func(d string) bool { return d == dir })
	a.syncGIFFolders()
}

func (a *App) syncGIFFolders() {
	if a.pick != nil {
		a.pick.SetFolders(a.gifFolders)
	}
}
