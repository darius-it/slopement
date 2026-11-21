package kit

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

// Polls are Matrix polls (m.poll.start, MSC3381): a question, 2–20
// answers, one or several choices, results shown as votes come in
// (disclosed) or only once it ends (undisclosed). Matrix polls have no end
// time of their own: the creator's client sends m.poll.end when the chosen
// duration is over.

// PollDurations are the lengths a poll can run (Discord's, shortened).
var PollDurations = []struct {
	Label string
	For   time.Duration // 0: until ended by hand
}{
	{"1 hour", time.Hour},
	{"1 day", 24 * time.Hour},
	{"3 days", 72 * time.Hour},
	{"1 week", 7 * 24 * time.Hour},
	{"No end", 0},
}

// MaxPollAnswers is Matrix's limit.
const MaxPollAnswers = 20

// PollDraft is the poll being written.
type PollDraft struct {
	Question string
	Answers  []string
	Multiple bool // allow several answers
	Hidden   bool // results only once it ends
	Duration int  // index of PollDurations
}

// NewPollDraft returns a draft with two empty answers, lasting a day.
func NewPollDraft() PollDraft { return PollDraft{Answers: []string{"", ""}, Duration: 1} }

// Valid reports whether the poll can be sent: a question and two answers.
func (d *PollDraft) Valid() bool {
	n := 0
	for _, a := range d.Answers {
		if hasText(a) {
			n++
		}
	}
	return hasText(d.Question) && n >= 2
}

// PollEditor is the poll's form: the question, the answers (each with a
// remove button, a new one added with "Add answer"), multiple answers,
// hidden results, and how long it runs. It reports a press of Create.
// Show it in a Dialog on desktops, a Sheet on phones (PollDialog).
func PollEditor(c *ui.Context, d *PollDraft) (create bool) {
	p := Of(c)
	label := func(s string) {
		ui.Text(c, s).FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.TextMuted)
	}
	ui.Column(c).Gap(S4).Children(func() {
		ui.Column(c).Gap(S15).Children(func() {
			label("Question")
			TextField(c.Key("q"), &d.Question, "", "Ask a question").AutoFocus()
		})
		ui.Column(c).Gap(S15).Children(func() {
			label("Answers")
			for i := 0; i < len(d.Answers); i++ {
				ui.Row(c.Key("ans" + strconv.Itoa(i))).Gap(S1).AlignItems(ui.Center).
					Transition(ui.ElementTransition{Duration: DurFast, ExitDuration: DurExit, Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}}).
					Children(func() {
						ui.Column(c).Grow(1).Children(func() {
							in := TextField(c, &d.Answers[i], "", "Answer "+strconv.Itoa(i+1))
							// Enter in the last answer adds another.
							if in.Submitted() && i == len(d.Answers)-1 && len(d.Answers) < MaxPollAnswers {
								d.Answers = append(d.Answers, "")
							}
						})
						x := IconButton(c, Ghost, IconX, "Remove answer").Disabled(len(d.Answers) <= 2)
						if x.Clicked() && len(d.Answers) > 2 {
							d.Answers = append(d.Answers[:i], d.Answers[i+1:]...)
						}
					})
			}
			if len(d.Answers) < MaxPollAnswers {
				add := ui.ButtonBase(c.Key("add")).Height(32).PaddingX(S2).Gap(S15).Radius(RadiusSm).AlignItems(ui.Center).
					AlignSelf(ui.Start).Label("Add answer")
				if add.Hovered() {
					add.Background(p.Hover)
				}
				add.Children(func() {
					ui.Icon(c, IconPlus).Size(16, 16).TextColor(p.AccentStrong)
					ui.Text(c, "Add answer").FontSize(SizeBody).FontWeight(WeightMedium).TextColor(p.AccentStrong)
				})
				if add.Clicked() {
					d.Answers = append(d.Answers, "")
				}
			}
		})
		ui.Column(c).Gap(S15).Children(func() {
			label("Runs for")
			ui.Row(c).Gap(S1 + 2).Wrap().Children(func() {
				for i, du := range PollDurations {
					if Chip(c.Key("dur"+du.Label), du.Label, d.Duration == i).Clicked() {
						d.Duration = i
					}
				}
			})
		})
		ui.Column(c).Radius(RadiusMd).Border(1, p.Border).Children(func() {
			settingRow(c, "Allow multiple answers", "", &d.Multiple)
			ui.Box(c).Height(1).Background(p.Border)
			settingRow(c, "Hide results until it ends", "Votes stay secret until then", &d.Hidden)
		})
		ui.Row(c).Justify(ui.End).Children(func() {
			create = Button(c, Primary, "Create poll", nil).Disabled(!d.Valid()).Clicked()
		})
	})
	return create
}

// settingRow is a labeled switch, the whole row toggling it.
func settingRow(c *ui.Context, title, sub string, on *bool) {
	p := Of(c)
	ui.Row(c.Key(title)).MinHeight(44).Padding(S2, S3).Gap(S3).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontSize(SizeBody).TextColor(p.Text)
			if sub != "" {
				ui.Text(c, sub).FontSize(SizeSmall).TextColor(p.TextMuted)
			}
		})
		ui.Switch(c, on).Label(title)
	})
}

// PollDialog shows the editor: a dialog on desktops, a sheet on phones.
func PollDialog(c *ui.Context, st *SheetState, d *PollDraft) (create bool) {
	if Compact(c) {
		Sheet(c, st, "New poll", func() { create = PollEditor(c, d) })
	} else {
		Dialog(c, &st.Open, "New poll", 440, func() { create = PollEditor(c, d) })
	}
	if create {
		st.Open = false
	}
	return create
}

// PollAnswer is an answer with its votes.
type PollAnswer struct {
	Text  string
	Votes int
	Mine  bool
	// Voters are who else voted for it (not the user: Mine says that),
	// from the poll's m.poll.response relations. Fewer than Votes when
	// not all are known yet; the rest show as "and N more".
	Voters []PollVoter
}

// PollVoter is someone who voted.
type PollVoter struct {
	ID, Name string
}

// pollVoters are the open "who voted" lists, by poll ID: the kit's own
// view state, as a popover's open flag.
var pollVoters = map[string]*SheetState{}

func pollVotersState(id string) *SheetState {
	st := pollVoters[id]
	if st == nil {
		st = &SheetState{}
		pollVoters[id] = st
	}
	return st
}

// PollData is a poll in the timeline.
type PollData struct {
	ID       string
	Question string
	Answers  []PollAnswer
	Multiple bool
	Hidden   bool
	Ends     time.Time // zero: no end
	Ended    bool
	Mine     bool // created by the user: may end it
	// Colors colors the voters' names and avatars in "who voted".
	Colors *UserColors
}

// PollAction is what the user did to a poll.
type PollAction struct {
	Vote int // the answer toggled, or -1
	End  bool
}

func (pd PollData) total() (votes int, voted bool) {
	for _, a := range pd.Answers {
		votes += a.Votes
		voted = voted || a.Mine
	}
	return
}

// remaining says how long a poll has left, as "3h left".
func remaining(ends, now time.Time) string {
	d := ends.Sub(now)
	switch {
	case d <= 0:
		return "ending"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())+1) + "m left"
	case d < 48*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h left"
	}
	return strconv.Itoa(int(d.Hours()/24)) + "d left"
}

// PollCard is a poll in the timeline: the question, how to answer, the
// answers (a radio or check box each; once results show, a bar of their
// share behind them, the leading one's in the accent), and the votes, the
// time left and, for its creator, End poll. A vote's bars grow in from the
// left.
func PollCard(c *ui.Context, pd PollData, now time.Time, width float32) (a PollAction) {
	p := Of(c)
	a.Vote = -1
	total, voted := pd.total()
	show := pd.Ended || (!pd.Hidden && voted)
	lead := 0
	for _, an := range pd.Answers {
		lead = max(lead, an.Votes)
	}
	ui.Column(c.Key("poll:"+pd.ID)).Width(width).MaxWidthPercent(100).Gap(S3).Padding(S3).Radius(RadiusMd).
		Background(p.Raised).Border(1, p.Border).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, pd.Question).FontSize(SizeLabel).FontWeight(WeightSemibold).TextColor(p.Text)
			how := "Select one"
			if pd.Multiple {
				how = "Select one or more"
			}
			if pd.Ended {
				how = "Final results"
			} else if pd.Hidden {
				how += " · results when it ends"
			}
			ui.Text(c, how).FontSize(SizeSmall).TextColor(p.TextMuted)
		})
		ui.Column(c).Gap(S15).Children(func() {
			for i, an := range pd.Answers {
				b := ui.ButtonBase(c.Key("ans"+strconv.Itoa(i))).MinHeight(40).Padding(S2, S3).Gap(S3).Radius(RadiusSm).
					AlignItems(ui.Center).Justify(ui.Start).Clip().Background(p.BgInset).Disabled(pd.Ended).DimDisabled(false).
					Label(an.Text).Checked(an.Mine)
				if b.Hovered() && !pd.Ended {
					b.Border(1, p.BorderStrong)
				} else if an.Mine {
					b.Border(1, p.Accent.Alpha(0.6))
				}
				share := float32(0)
				if show && total > 0 {
					share = float32(an.Votes) / float32(total)
				}
				fill := p.Text.Alpha(0.06)
				if an.Votes == lead && lead > 0 {
					fill = p.Accent.Alpha(0.22) // the leader reads strongest
				}
				b.Children(func() {
					// The share, behind the row, growing as it changes.
					grow := b.AnimateTo("share", share, ui.Animation{Duration: 320 * time.Millisecond, Ease: EaseOut})
					ui.Box(c.Key("bar")).Absolute().Top(0).Bottom(0).Left(0).WidthPercent(grow * 100).
						Background(fill).PassThrough()
					// The answer, over the bar.
					ui.Row(c.Key("content")).Grow(1).MinWidth(0).Gap(S3).AlignItems(ui.Center).ZIndex(1).Children(func() {
						mark(c, an.Mine, pd.Multiple)
						ui.Text(c, an.Text).FontSize(SizeBody).TextColor(p.Text).Grow(1).MinWidth(0)
						if show {
							pct := 0
							if total > 0 {
								pct = int(share*100 + 0.5)
							}
							ui.Text(c, strconv.Itoa(pct)+"%").FontSize(SizeSmall).FontWeight(WeightMedium).FontFeatures("tnum").
								TextColor(p.TextMuted)
						}
					})
				})
				Pressable(b)
				if b.Clicked() && !pd.Ended {
					a.Vote = i
				}
			}
		})
		ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).AlignItems(ui.Center).Children(func() {
				votesLink(c, pd, total, show)
				if !pd.Ended && !pd.Ends.IsZero() {
					ui.Text(c, " · "+remaining(pd.Ends, now)).FontSize(SizeSmall).FontFeatures("tnum").TextColor(p.TextMuted)
				}
			})
			if pd.Mine && !pd.Ended {
				end := ui.ButtonBase(c.Key("end")).Height(28).PaddingX(S2).Radius(RadiusSm).AlignItems(ui.Center).Label("End poll")
				if end.Hovered() {
					end.Background(p.Hover)
				}
				end.Children(func() {
					ui.Text(c, "End poll").FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.AccentStrong)
				})
				a.End = end.Clicked()
			}
		})
	})
	return a
}

// votesLink is a poll's "N votes": once its results show, a link (brighter
// and underlined on hover) to who voted for what, in a popover (a sheet on
// phones). While a hidden poll runs, who voted would give its results
// away, so it stays plain text.
func votesLink(c *ui.Context, pd PollData, total int, show bool) {
	p := Of(c)
	label := strconv.Itoa(total) + " vote"
	if total != 1 {
		label += "s"
	}
	if !show || total == 0 {
		t := ui.Text(c.Key("votes"), label).FontSize(SizeSmall).FontFeatures("tnum").TextColor(p.TextMuted)
		if pd.Hidden && !pd.Ended {
			t.Tooltip("Who voted shows when the poll ends")
		}
		return
	}
	st := pollVotersState(pd.ID)
	b := ui.ButtonBase(c.Key("votes")).Radius(RadiusSm).Label("See who voted (" + label + ")").Cursor(ui.CursorPointer)
	hov := b.Hovered() || b.FocusVisible() || st.Open
	col := p.TextMuted
	if hov {
		col = p.Text
	}
	b.Children(func() {
		t := ui.Text(c, label).FontSize(SizeSmall).FontFeatures("tnum").TextColor(b.AnimateColor("c", col, Color))
		if hov {
			t.Underline()
		}
	})
	if b.Clicked() {
		st.Open = !st.Open
	}
	Adaptive(c, b, st, "", 300, func() { pollVoterList(c, pd) })
}

// pollVoterList is who voted for what: each answer with its count, then
// its voters (you first), avatars and names in their colors.
func pollVoterList(c *ui.Context, pd PollData) {
	p := Of(c)
	colors := pd.Colors
	if colors == nil {
		colors = &UserColors{}
	}
	ui.Text(c, pd.Question).FontSize(SizeLabel).FontWeight(WeightSemibold).TextColor(p.Text)
	ui.Scroll(c.Key("voters")).MaxHeight(340).Children(func() {
		ui.Column(c).Gap(S3).Children(func() {
			for i, an := range pd.Answers {
				ui.Column(c.Key("va" + strconv.Itoa(i))).Gap(S1).Children(func() {
					ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(func() {
						ui.Text(c, an.Text).FontSize(SizeSmall).FontWeight(WeightSemibold).TextColor(p.Text).Grow(1).MinWidth(0)
						ui.Text(c, strconv.Itoa(an.Votes)).FontSize(SizeSmall).FontFeatures("tnum").TextColor(p.TextMuted)
					})
					known := 0
					if an.Mine {
						voterRow(c, "me", "You", "", colors)
						known++
					}
					for _, v := range an.Voters {
						voterRow(c, v.ID, v.Name, v.ID, colors)
						known++
					}
					switch {
					case an.Votes == 0:
						ui.Text(c, "No votes").FontSize(SizeSmall).TextColor(p.TextFaint)
					case an.Votes > known:
						ui.Text(c, "and "+strconv.Itoa(an.Votes-known)+" more").FontSize(SizeSmall).TextColor(p.TextFaint)
					}
				})
			}
		})
	})
}

// voterRow is a voter: a small avatar and their name in their color.
func voterRow(c *ui.Context, key, name, mxid string, colors *UserColors) {
	p := Of(c)
	ui.Row(c.Key("v:" + key)).Gap(S2).AlignItems(ui.Center).MinHeight(24).Children(func() {
		id := mxid
		if id == "" {
			id = name
		}
		Avatar(c, AvatarSpec{Name: name, ID: id, Size: 20, Colors: colors})
		col := p.Text
		if mxid != "" {
			col = colors.Color(p, mxid)
		}
		ui.Text(c, name).FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(col).SingleLine().MinWidth(0)
	})
}

// mark is an answer's radio (one choice) or check box (several).
func mark(c *ui.Context, on, multiple bool) {
	p := Of(c)
	r := float32(RadiusFull)
	if multiple {
		r = 5
	}
	box := ui.Box(c).Size(18, 18).Radius(r).Shrink(0).AlignItems(ui.Center).Justify(ui.Center)
	if on {
		box.Background(p.Accent).Children(func() {
			if multiple {
				ui.Icon(c, IconCheck).Size(12, 12).TextColor(p.OnAccent)
			} else {
				ui.Box(c).Size(6, 6).Radius(RadiusFull).Background(p.OnAccent)
			}
		})
	} else {
		box.Border(1.5, p.BorderStrong)
	}
}
