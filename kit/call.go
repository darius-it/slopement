package kit

import "github.com/egoist/mygo/ui"

// CallState is the local side of a call. Calls start with the camera off
// and the microphone on: one kind of call, the camera a toggle.
type CallState struct {
	Mic, Camera, Screen bool
}

// NewCall returns the state a call starts in.
func NewCall() CallState { return CallState{Mic: true} }

// CallAction is what the call bar asked for.
type CallAction struct {
	Leave bool
}

// CallBar is the native control bar under a call: microphone, camera,
// screen sharing (desktop) and leave.
func CallBar(c *ui.Context, st *CallState) (a CallAction) {
	p := Of(c)
	ui.Row(c).Gap(S3).Padding(S2, S3).Radius(RadiusFull).Background(p.BgSubtle).Border(1, p.Border).
		AlignSelf(ui.Center).Children(func() {
		mic, micLabel := IconMic, "Mute microphone"
		if !st.Mic {
			mic, micLabel = IconMicOff, "Unmute microphone"
		}
		if RoundButton(c, mic, micLabel, st.Mic, false).Clicked() {
			st.Mic = !st.Mic
		}
		cam, camLabel := IconVideo, "Turn camera off"
		if !st.Camera {
			cam, camLabel = IconVideoOff, "Turn camera on"
		}
		if RoundButton(c, cam, camLabel, st.Camera, false).Clicked() {
			st.Camera = !st.Camera
		}
		if !Touch(c) {
			lbl := "Share screen"
			if st.Screen {
				lbl = "Stop sharing"
			}
			if RoundButton(c, IconScreen, lbl, !st.Screen, false).Clicked() {
				st.Screen = !st.Screen
			}
		}
		if RoundButton(c, IconPhoneOff, "Leave call", true, true).Clicked() {
			a.Leave = true
		}
	})
	return a
}

// CallBanner is the strip shown in a room (and on its row) while a call
// goes on: who is in it, and Join.
func CallBanner(c *ui.Context, participants int, joined bool) (join bool) {
	p := Of(c)
	ui.Row(c).Gap(S3).Padding(S2, S3).Radius(RadiusMd).Background(p.AccentSubtle).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(8, 8).Radius(RadiusFull).Background(p.Success)
		label := "Call in progress"
		if participants > 0 {
			label += " · " + CountText(participants) + " in call"
		}
		ui.Text(c, label).FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.Text).Grow(1)
		if !joined {
			join = Button(c, Primary, "Join", IconPhone).Clicked()
		}
	})
	return join
}
