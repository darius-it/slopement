package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// Threads: a reply in a thread shows in the thread's panel, not among the
// room's messages; the root says how many replies it has, with a chip that
// opens the panel; "Reply in thread" in a message's menu starts one. On a
// source with threads and a window wide enough for the panel (a phone shows
// replies among the messages, as there is no panel).

// threadState is the open thread: where, and what is being written to it.
type threadState struct {
	Room, Root string
	Draft      string
	list       ui.ListState
}

// threadsOn says threads are shown as threads: the source has them and the
// window has room for the panel.
func (a *App) threadsOn() bool {
	_, ok := a.src.(model.Threads)
	return ok && !a.narrow
}

// openThread opens the thread a message is in, or starts one on it, in the
// side panel.
func (a *App) openThread(room string, ev model.Event) {
	root := ev.ThreadRoot
	if root == "" {
		root = ev.ID
	}
	a.thread = threadState{Room: room, Root: root}
	a.thread.list.FollowEnd = true
	a.panel = panelThread
}

// threadEvents are the root and its replies, oldest first. The root may not
// be among what is loaded: then there are only the replies.
func (a *App) threadEvents(room, root string) []model.Event {
	var out []model.Event
	for _, e := range a.src.Timeline(room) {
		if e.ID == root || e.ThreadRoot == root {
			out = append(out, e)
		}
	}
	return out
}

// threadChipOf is the chip for a root's replies: how many, the first three
// to reply, when the last came.
func (a *App) threadChipOf(t *model.Thread) *kit.ThreadChip {
	chip := &kit.ThreadChip{Replies: t.Replies, Last: timeLabel(t.Last, a.now)}
	for _, who := range t.Participants {
		pe, _ := a.src.Person(who)
		chip.Avatars = append(chip.Avatars, a.avatar(pe, 18))
	}
	return chip
}

// threadPanel is the open thread in the side panel: the root and its
// replies, and a line to write one.
func (a *App) threadPanel(c *ui.Context, r model.Room) {
	p := kit.Of(c)
	th := &a.thread
	evs := a.threadEvents(th.Room, th.Root)
	if th.Room != r.ID {
		emptyNote(c, "Open a thread from a message in this room.")
		return
	}
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		a.inThread = true
		ui.List(c.Key("thread:"+th.Root), &th.list, len(evs), func(i int) {
			a.timelineRow(c, r, trow{key: evs[i].ID, ev: &evs[i], first: true})
		}).Grow(1).Justify(ui.End)
		a.inThread = false
		if len(evs) == 0 {
			emptyNote(c, "This thread has no messages yet.")
		}
		// The line to write a reply.
		ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Padding(kit.S2).Shrink(0).BorderWidth(1, 0, 0, 0).BorderColor(p.Border).Children(func() {
			box := ui.Row(c).Grow(1).MinWidth(0).Height(36).PaddingX(kit.S3).Radius(kit.RadiusMd).Background(p.BgInset).
				Border(1, p.Border).AlignItems(ui.Center)
			var in ui.Element
			box.Children(func() {
				in = ui.TextInputBase(c.Key("thread-in"), &th.Draft).Grow(1).Placeholder("Reply in thread").
					PlaceholderColor(p.TextMuted).Label("Reply in thread").FontSize(kit.SizeBody)
			})
			send := kit.IconButton(c, kit.Primary, kit.IconSend, "Send reply").Clicked()
			if (send || in.Submitted()) && strings.TrimSpace(th.Draft) != "" {
				if ts, ok := a.src.(model.Threads); ok {
					ts.SendInThread(th.Room, th.Root, th.Draft)
					th.Draft = ""
					th.list.ScrollToEnd()
				}
			}
		})
	})
}
