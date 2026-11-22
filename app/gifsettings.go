package app

import (
	"path/filepath"
	"slices"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// gifsPage is Settings → GIFs: the folders on this device whose pictures the
// GIF picker offers beside KLIPY's.
func (a *App) gifsPage(c *ui.Context) {
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
