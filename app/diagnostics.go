package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// keyRows are the rows of Settings → Security about the keys: where they
// stand, and a file of them to take elsewhere or bring in.
func (a *App) keyRows(c *ui.Context) {
	ks, ok := a.src.(model.KeyStorage)
	if !ok {
		if kit.ChoiceRowSoon(c, kit.IconKey, "Key storage", "Back up your keys on your homeserver").Clicked() {
			a.toast = soonMsg
		}
		if kit.ChoiceRowSoon(c, kit.IconDownload, "Export room keys", "A file encrypted with a passphrase").Clicked() {
			a.toast = soonMsg
		}
		return
	}
	// Asked once each time the page opens, like the sessions.
	if !a.keysAsked {
		a.keysAsked = true
		ks.RefreshKeyStatus()
	}
	if kit.ChoiceRow(c, kit.IconKey, "Key storage", keyStatusLine(ks.KeyStatus())).Clicked() {
		a.openKeys(keysStatus)
	}
	if kit.ChoiceRow(c, kit.IconDownload, "Export room keys", "A file locked with a passphrase, to keep or take to another client").Clicked() {
		a.openKeys(keysExport)
	}
	if kit.ChoiceRow(c, kit.IconDownload, "Import room keys", "From a file made here or in Element").Clicked() {
		a.openKeys(keysImport)
	}
}

// resetIdentityGroup is the one destructive row of the page, apart.
func (a *App) resetIdentityGroup(c *ui.Context) {
	if _, ok := a.src.(model.IdentityResetter); !ok {
		return
	}
	group(c, "If you are locked out", "Use this only when you have lost your Security Key and have no other verified device.", func() {
		if kit.ChoiceRow(c, kit.IconShield, "Reset your identity", "A new encryption identity and Security Key").Clicked() {
			a.openKeys(keysReset)
		}
	})
}

// exportLogs asks what the export keeps, makes it, and asks where to save it.
func (a *App) exportLogs() {
	le, ok := a.src.(model.LogExporter)
	if !ok {
		return
	}
	if a.confirmLogs != nil { // tests answer for the system's dialog
		a.confirmLogs(func(include, ok bool) {
			if ok {
				a.makeLogExport(le, include)
			}
		})
		return
	}
	if a.win == nil {
		return
	}
	win := a.win
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: win, Title: "Export logs",
			Message: "A zip of the app's logs, its version and your homeserver's. Message text, names, topics, tokens and keys are never logged. People, room and event IDs are replaced by short hashes — unless you include room IDs, which helps whoever you send it to find the room.",
			Buttons: []string{"Export", "Export with room IDs", "Cancel"}, DefaultButton: 0, CancelButton: 2})
		if err != nil || res.Button == 2 {
			return
		}
		win.Update(func() { a.makeLogExport(le, res.Button == 1) })
	}()
}

func (a *App) makeLogExport(le model.LogExporter, includeRooms bool) {
	le.ExportLogs(includeRooms, func(data []byte, name, problem string) {
		if problem != "" {
			a.toast = problem
			return
		}
		a.saveFile("Save the logs", name, func(path string) {
			a.writeSaved(path, data, 0o600, func(saved string, err error) {
				switch {
				case err != nil:
					a.toast = "Could not save the logs: " + err.Error()
				case saved != "":
					a.toast = "Saved " + saved
				}
			})
		})
	})
}
