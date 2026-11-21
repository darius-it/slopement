package kit

import (
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// abs is |x|.
func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// TestToastStack shows three toasts: they stack, the newest in front, and
// glide as the stack changes. SNAP_DIR writes the stack.
func TestToastStack(t *testing.T) {
	n := 0
	view := func(c *ui.Context) {
		Apply(c)
		ui.Column(c).Padding(12).Children(func() {
			if ui.Button(c, "Notify").Clicked() {
				n++
				c.AddToast(ui.Toast{ID: strconv.Itoa(n), Title: "Saved draft " + strconv.Itoa(n)})
			}
		})
		Toasts(c)
	}
	tt := ui.NewTester(view, 520, 320)
	tt.Frame()
	for i := 1; i <= 3; i++ {
		if err := tt.Click("Notify"); err != nil {
			t.Fatal(err)
		}
		tt.Advance(80 * time.Millisecond)
		tt.Frame()
	}
	for i := 1; i <= 3; i++ {
		if !tt.HasText("Saved draft " + strconv.Itoa(i)) {
			t.Errorf("toast %d is not showing", i)
		}
	}
	tt.Advance(600 * time.Millisecond) // let the stack springs settle
	tt.Frame()
	// The stack's geometry, from what assistive technology sees: the
	// newest S5 above the window's bottom edge, the older ones peeking
	// above it, all bottom-centered.
	byName := map[string]ui.Rect{}
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleStatus {
			byName[a.Name] = a.Bounds
		}
	}
	for i := 1; i <= 3; i++ {
		if _, ok := byName["Saved draft "+strconv.Itoa(i)]; !ok {
			t.Fatalf("toast %d is not showing: %v", i, byName)
		}
	}
	for i, lift := range []float32{48, 34, 20} { // draft 1..3: depth 2..0
		r := byName["Saved draft "+strconv.Itoa(i+1)]
		if got := 320 - (r.Y + r.H); abs(got-lift) > 4 {
			t.Errorf("toast %d sits %v above the bottom, want %v", i+1, got, lift)
		}
		if got := r.X + r.W/2; abs(got-260) > 2 {
			t.Errorf("toast %d is centered at %v, want 260", i+1, got)
		}
	}
	// Clicking a card dismisses it, as Sonner's does. The front toast's
	// center is covered by nothing; a back toast's center is covered by
	// the toast in front of it, whose click that is (as Sonner's stack
	// behaves), so the front one is the one to click.
	front := byName["Saved draft 3"]
	tt.ClickAt(front.X+front.W/2, front.Y+front.H/2)
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if tt.HasText("Saved draft 3") {
		t.Error("the clicked toast is still showing")
	}
	if !tt.HasText("Saved draft 1") || !tt.HasText("Saved draft 2") {
		t.Error("a click dismissed more than its own toast")
	}
	if dir := os.Getenv("SNAP_DIR"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "toast-stack.png"))
		if err == nil {
			defer f.Close()
			png.Encode(f, tt.Image())
		}
	}
}
