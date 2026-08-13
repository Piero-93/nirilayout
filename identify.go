package nirilayout

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

var (
	identifyFlag         = flag.Bool("identify", false, N("show a large overlay on every connected monitor with its name, resolution and scale, then exit"))
	identifyOnOpenFlag   = flag.Bool("identify-on-open", false, N("show the monitor identification overlays automatically whenever the picker opens"))
	identifyDurationFlag = flag.Duration("identify-duration", identifyDefaultDuration, N("how long the monitor identification overlays stay on screen (e.g. 3s, 1500ms)"))
)

const (
	// identifyDefaultDuration is how long the overlays stay up when
	// -identify-duration is not given.
	identifyDefaultDuration = 3 * time.Second
	// identifyMinDuration keeps the overlays on screen long enough to be read.
	identifyMinDuration = 200 * time.Millisecond
	// identifyMaxDuration bounds them from above. The overlays sit on the
	// overlay layer, covering whatever is underneath, so "until dismissed" is
	// deliberately not a reachable state: a wedged overlay would block the
	// screen with no obvious way out.
	identifyMaxDuration = 30 * time.Second
	// identifyMargin is the gap between the surface and the corner it sits in.
	// The card is inset a little further by the window padding that gives its
	// shadow room.
	identifyMargin = 12
)

// IdentifyOnOpen reports whether -identify-on-open was passed. Consumed by the
// picker to pop the overlays as soon as it is up.
func IdentifyOnOpen() bool { return *identifyOnOpenFlag }

// Mode is what the process was asked to do. The three modes are mutually
// exclusive and are resolved once, up front, from the flags.
type Mode int

const (
	// ModePicker shows the layout switcher. The default.
	ModePicker Mode = iota
	// ModeWatch runs the headless recovery daemon (-watch).
	ModeWatch
	// ModeIdentify shows the monitor overlays and exits (-identify).
	ModeIdentify
)

// resolveMode maps the mode flags to the single thing the process will do.
//
// -watch and -identify are not merely redundant together: watch mode is
// headless and never builds a GUI, so it would silently swallow -identify.
// Saying so beats leaving the user wondering why nothing appeared.
func resolveMode(watch, identify bool) (Mode, error) {
	if watch && identify {
		return ModePicker, errors.New("-identify and -watch cannot be combined: watch mode runs headless and never shows an overlay")
	}
	if identify {
		return ModeIdentify, nil
	}
	if watch {
		return ModeWatch, nil
	}
	return ModePicker, nil
}

// ResolveMode reports what the flags asked the process to do.
func ResolveMode() (Mode, error) { return resolveMode(*watchFlag, *identifyFlag) }

// clampIdentifyDuration keeps -identify-duration inside a range where the
// overlays are both readable and guaranteed to go away. A non-positive value
// falls back to the default rather than meaning "forever".
func clampIdentifyDuration(d time.Duration) time.Duration {
	switch {
	case d <= 0:
		return identifyDefaultDuration
	case d < identifyMinDuration:
		return identifyMinDuration
	case d > identifyMaxDuration:
		return identifyMaxDuration
	}
	return d
}

// OutputInfo is what one overlay says about the monitor it sits on.
//
// The numbers come from niri when it knows the connector, because layout files
// are written against niri's view of the world: its modes are in physical
// pixels and its scale is the one the user types into `layout_<name>.kdl`.
// GDK is the fallback, and its geometry is already scaled, which is why
// FromNiri records where the numbers came from.
type OutputInfo struct {
	Index                     int
	Connector, Description    string
	Width, Height, RefreshMHz int
	Scale                     float64
	FromNiri                  bool
}

// Headline is the large first line: the connector name, which is exactly the
// string to write in a layout file. It degrades through the monitor
// description to a bare ordinal, so an overlay is never left blank.
func (i OutputInfo) Headline() string {
	if i.Connector != "" {
		return i.Connector
	}
	if i.Description != "" {
		return i.Description
	}
	return Tf("Monitor %d", i.Index+1)
}

// DetailLine is the small second line, e.g. "1920x1080@60  1×". It is empty
// when the mode is unknown, which is the case for an output that is connected
// but off; the caller then falls back to what GDK knows.
func (i OutputInfo) DetailLine() string {
	if i.Width <= 0 || i.Height <= 0 {
		return ""
	}
	s := fmt.Sprintf("%dx%d", i.Width, i.Height)
	if hz := formatRefreshHz(i.RefreshMHz); hz != "" {
		s += "@" + hz
	}
	if scale := formatScale(i.Scale); scale != "" {
		s += "  " + scale + "×"
	}
	return s
}

// hasMode reports whether the mode is known, i.e. whether DetailLine would say
// anything.
func (i OutputInfo) hasMode() bool { return i.Width > 0 && i.Height > 0 }

// formatRefreshHz renders a milli-Hertz refresh rate the way a monitor's OSD
// would: 60049 -> "60", 143856 -> "143.9". A zero rate yields "", so the
// caller omits the "@…" segment rather than printing "@0".
func formatRefreshHz(mHz int) string {
	if mHz <= 0 {
		return ""
	}
	return trimFloat(float64(mHz)/1000, 1)
}

// formatScale renders a scale factor without the "×", which DetailLine adds:
// 1.0 -> "1", 1.5 -> "1.5". Rounding to two decimals keeps float noise such as
// 1.5000000000000002 from reaching the screen.
func formatScale(s float64) string {
	if s <= 0 {
		return ""
	}
	return trimFloat(s, 2)
}

// trimFloat rounds to at most `places` decimals and formats without trailing
// zeros, so whole numbers print bare.
func trimFloat(v float64, places int) string {
	pow := math.Pow(10, float64(places))
	return strconv.FormatFloat(math.Round(v*pow)/pow, 'f', -1, 64)
}

// niriOutputInfo turns what niri reports into per-connector overlay text. An
// output whose current mode is unknown (it is connected but off, or the index
// is out of range) still gets an entry, with no mode, so the connector name is
// kept and only the numbers fall back to GDK.
func niriOutputInfo(outputs map[string]niriOutput) map[string]OutputInfo {
	if outputs == nil {
		return nil
	}
	infos := make(map[string]OutputInfo, len(outputs))
	for name, o := range outputs {
		info := OutputInfo{Connector: name, FromNiri: true}
		if o.CurrentMode != nil && *o.CurrentMode >= 0 && *o.CurrentMode < len(o.Modes) {
			m := o.Modes[*o.CurrentMode]
			info.Width, info.Height, info.RefreshMHz = m.Width, m.Height, m.RefreshRate
		}
		if o.Logical != nil {
			info.Scale = o.Logical.Scale
		}
		infos[name] = info
	}
	return infos
}

// offOutputs names the outputs niri sees but is not driving. They have no
// wl_output, so GDK cannot see them and there is no screen to draw their
// overlay on; the only useful thing to do is say so in the log.
func offOutputs(outputs map[string]niriOutput) []string {
	var off []string
	for name, o := range outputs {
		if o.Logical == nil {
			off = append(off, name)
		}
	}
	slices.Sort(off)
	return off
}

// gdkMonitors returns the monitors of the default display, in list order.
// GTK's list model hands items back as plain GObjects, so each has to be
// wrapped again — the same thing the layer-shell binding does internally.
func gdkMonitors() []*gdk.Monitor {
	display := gdk.DisplayGetDefault()
	if display == nil {
		return nil
	}
	list := display.Monitors()
	n := list.NItems()
	mons := make([]*gdk.Monitor, 0, n)
	for i := uint(0); i < n; i++ {
		obj := list.Item(i)
		if obj == nil {
			continue
		}
		mons = append(mons, &gdk.Monitor{Object: obj})
	}
	return mons
}

// gdkOutputInfo reads what GDK knows about a monitor. Its geometry is the
// logical rectangle, already divided by the scale, so these numbers are not
// the mode — which is why niri wins whenever it can answer.
func gdkOutputInfo(m *gdk.Monitor, index int) OutputInfo {
	info := OutputInfo{
		Index:       index,
		Connector:   m.Connector(),
		Description: m.Description(),
		RefreshMHz:  m.RefreshRate(),
		Scale:       m.Scale(),
	}
	if info.Description == "" {
		info.Description = strings.TrimSpace(m.Manufacturer() + " " + m.Model())
	}
	if g := m.Geometry(); g != nil {
		info.Width, info.Height = g.Width(), g.Height()
	}
	return info
}

// outputInfoFor decides what one monitor's overlay says. niri is preferred,
// because layout files are written against its connector names and its modes
// are the physical ones; GDK fills in whatever niri could not supply, up to
// the entire line when the query failed.
func outputInfoFor(m *gdk.Monitor, index int, niri map[string]OutputInfo) OutputInfo {
	gdkInfo := gdkOutputInfo(m, index)

	info, ok := niri[gdkInfo.Connector]
	if !ok || gdkInfo.Connector == "" {
		return gdkInfo
	}

	info.Index = index
	info.Description = gdkInfo.Description
	if !info.hasMode() {
		// niri could not resolve a current mode, so borrow GDK's numbers but
		// keep niri's name — the name is the part the user needs.
		info.Width, info.Height, info.RefreshMHz = gdkInfo.Width, gdkInfo.Height, gdkInfo.RefreshMHz
		info.FromNiri = false
	}
	if info.Scale <= 0 {
		info.Scale = gdkInfo.Scale
	}
	return info
}

// focusedOutputName asks niri which output has the focus, so that exactly one
// overlay takes the keyboard in standalone mode. An empty result means niri
// could not say, and the caller falls back to the first monitor.
func focusedOutputName() string {
	ctx, cancel := context.WithTimeout(context.Background(), niriQueryTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "niri", "msg", "-j", "focused-output").Output()
	if err != nil {
		return ""
	}
	var o struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &o); err != nil {
		return ""
	}
	return o.Name
}

// overlay is one on-screen card, plus the bit of bookkeeping needed to tear it
// down exactly once. The compositor destroys the window itself when its output
// disappears mid-show (an unplug sends layer_surface.closed), so `destroyed`
// is what keeps the timer from destroying it a second time.
type overlay struct {
	win       *gtk.ApplicationWindow
	destroyed bool
}

// identifySession is one showing of the overlays: the windows, the timer that
// takes them away, and whether it has already been torn down.
type identifySession struct {
	app        *gtk.Application
	overlays   []*overlay
	handle     glib.SourceHandle
	standalone bool
	done       bool
}

// currentIdentify is the one live session, if any. There is deliberately no
// mutex: every function that touches it runs on the GTK main thread.
var currentIdentify *identifySession

// active reports whether the session is still showing. The nil receiver is
// meaningful — it is the "nothing is up" state — so callers can ask
// currentIdentify.active() without a guard.
func (s *identifySession) active() bool { return s != nil && !s.done }

// cancelTimer drops the pending dismissal, if there is one. Removing an
// already-fired source makes GLib log a critical, hence the zero check and the
// zeroing done by the callback itself.
func (s *identifySession) cancelTimer() {
	if s.handle == 0 {
		return
	}
	glib.SourceRemove(s.handle)
	s.handle = 0
}

// rearm restarts the dismissal countdown, so triggering identify again while
// the overlays are up just keeps them around longer instead of stacking a
// second set of windows.
func (s *identifySession) rearm(d time.Duration) {
	if !s.active() {
		return
	}
	s.cancelTimer()
	s.handle = glib.TimeoutAdd(uint(d.Milliseconds()), func() bool {
		s.handle = 0
		s.dismiss()
		return false
	})
}

// dismiss fades the overlays out and destroys them. It is idempotent: the
// timer, the Esc key, a click and the picker quitting can all race to call it.
func (s *identifySession) dismiss() {
	if !s.active() {
		return
	}
	s.done = true
	s.cancelTimer()
	if currentIdentify == s {
		currentIdentify = nil
	}

	for _, o := range s.overlays {
		if !o.destroyed {
			o.win.RemoveCSSClass("visible")
		}
	}

	// Same fade-out delay the picker uses before it quits (see quit() in
	// nirilayout.go); destroying right away would skip the transition.
	glib.TimeoutAdd(75, func() bool {
		for _, o := range s.overlays {
			if !o.destroyed {
				o.win.Destroy()
			}
		}
		if s.standalone {
			// The overlays are the only windows, so this is also the process
			// exiting. Say it outright rather than relying on GTK's window
			// counting.
			s.app.Quit()
		}
		return false
	})
}

// newOverlay builds the card for one monitor and puts it on screen.
//
// keyboard must be true for at most one overlay, and never while the picker is
// open: the picker holds an exclusive keyboard grab of its own, and a second
// exclusive layer surface would fight it for input.
func (s *identifySession) newOverlay(mon *gdk.Monitor, info OutputInfo, keyboard bool) *overlay {
	win := gtk.NewApplicationWindow(s.app)
	win.SetTitle("nirilayout")
	win.AddCSSClass("identify")

	gtk4layershell.InitForWindow(&win.Window)
	gtk4layershell.SetLayer(&win.Window, gtk4layershell.LayerShellLayerOverlay)
	gtk4layershell.SetMonitor(&win.Window, mon)
	gtk4layershell.SetNamespace(&win.Window, "nirilayout-identify")
	// Pinned to the top-left corner rather than centred: the picker is centred
	// itself, and a card in the middle of the screen would sit right on top of
	// the layouts you are trying to choose between.
	gtk4layershell.SetAnchor(&win.Window, gtk4layershell.LayerShellEdgeTop, true)
	gtk4layershell.SetAnchor(&win.Window, gtk4layershell.LayerShellEdgeLeft, true)
	gtk4layershell.SetMargin(&win.Window, gtk4layershell.LayerShellEdgeTop, identifyMargin)
	gtk4layershell.SetMargin(&win.Window, gtk4layershell.LayerShellEdgeLeft, identifyMargin)
	// Reserve nothing, but stay clear of what bars and docks have reserved, so
	// the card lands under a top bar instead of behind it.
	gtk4layershell.SetExclusiveZone(&win.Window, 0)
	if keyboard {
		gtk4layershell.SetKeyboardMode(&win.Window, gtk4layershell.LayerShellKeyboardModeExclusive)
	} else {
		gtk4layershell.SetKeyboardMode(&win.Window, gtk4layershell.LayerShellKeyboardModeNone)
	}

	// The window itself is transparent and only leaves room for the drop
	// shadow; the card is what the user actually sees. Text is left-aligned,
	// like a notification, since the card sits in a corner.
	card := gtk.NewBox(gtk.OrientationVertical, 2)
	card.AddCSSClass("card")
	card.AddCSSClass("identify-card")

	name := gtk.NewLabel(info.Headline())
	name.AddCSSClass("identify-name")
	name.SetXAlign(0)
	card.Append(name)

	if details := info.DetailLine(); details != "" {
		label := gtk.NewLabel(details)
		label.AddCSSClass("identify-details")
		label.SetXAlign(0)
		card.Append(label)
	}
	win.SetChild(card)

	o := &overlay{win: win}
	win.ConnectDestroy(func() { o.destroyed = true })

	// Clicking works without any keyboard focus, so it is the dismissal that
	// is always available, including while the picker owns the keyboard.
	click := gtk.NewGestureClick()
	click.ConnectPressed(func(nPress int, x, y float64) { s.dismiss() })
	win.AddController(click)

	if keyboard {
		k := gtk.NewEventControllerKey()
		k.SetPropagationPhase(gtk.PhaseCapture)
		k.ConnectKeyPressed(func(keyval, keycode uint, state gdk.ModifierType) bool {
			if keyval == gdk.KEY_Escape {
				s.dismiss()
				return true
			}
			return false
		})
		win.AddController(k)
	}

	// style.css hides every window until it has the "visible" class; without
	// this the overlay would be mapped but fully transparent.
	win.ConnectShow(func() { win.AddCSSClass("visible") })
	win.SetVisible(true)

	return o
}

// ShowIdentify pops one overlay per connected monitor and starts the countdown
// that takes them away. Called while overlays are already up, it only restarts
// that countdown. It returns nil when there was nothing to show.
//
// standalone marks -identify mode, where the process exists only for these
// overlays; in picker mode they are an extra on top of a window that already
// owns the keyboard.
func ShowIdentify(app *gtk.Application, standalone bool) *identifySession {
	d := clampIdentifyDuration(*identifyDurationFlag)

	if currentIdentify.active() {
		currentIdentify.rearm(d)
		return currentIdentify
	}

	if !gtk4layershell.IsSupported() {
		log.Print("nirilayout: layer shell is not supported here; cannot identify monitors")
		return nil
	}

	mons := gdkMonitors()
	if len(mons) == 0 {
		log.Print("nirilayout: no monitors to identify")
		return nil
	}

	// A failed query is not fatal: GDK alone can name every monitor it draws
	// on, so degrade to that rather than showing nothing.
	var infos map[string]OutputInfo
	if outputs, err := niriOutputs(); err != nil {
		log.Printf("nirilayout: %v; using what GDK reports instead", err)
	} else {
		infos = niriOutputInfo(outputs)
		if off := offOutputs(outputs); len(off) > 0 {
			log.Printf("nirilayout: not identifying outputs that are connected but off: %s", strings.Join(off, ", "))
		}
	}

	// At most one surface may take the keyboard, and only when no picker is
	// already holding it. Prefer the focused output so Esc lands where the
	// user is looking.
	keyboardOn := -1
	if standalone {
		keyboardOn = 0
		if focused := focusedOutputName(); focused != "" {
			for i, m := range mons {
				if m.Connector() == focused {
					keyboardOn = i
					break
				}
			}
		}
	}

	s := &identifySession{app: app, standalone: standalone}
	for i, m := range mons {
		s.overlays = append(s.overlays, s.newOverlay(m, outputInfoFor(m, i, infos), i == keyboardOn))
	}
	currentIdentify = s
	s.rearm(d)
	return s
}

// DismissIdentify takes the overlays away if any are showing, and reports
// whether it did. The picker uses the answer to make the first Esc close the
// overlays and the second one close the picker.
func DismissIdentify() bool {
	if !currentIdentify.active() {
		return false
	}
	currentIdentify.dismiss()
	return true
}

// RunIdentify is the whole of -identify mode: show the overlays, then let the
// process end when they go away. The overlays are application windows, so the
// application runs exactly as long as they do.
func RunIdentify(app *gtk.Application) {
	LoadStyles()

	if ShowIdentify(app, true) == nil {
		// Nothing to identify, and no screen to say so on. Do not linger.
		app.Quit()
		return
	}

	// Last line of defence. Every other guard lives inside the GLib main loop
	// and dies with it, and the failure this protects against is a surface on
	// the overlay layer covering the user's whole screen with no way to
	// dismiss it. This timer runs on its own thread, so it still fires if the
	// main loop is wedged in a Wayland roundtrip.
	time.AfterFunc(clampIdentifyDuration(*identifyDurationFlag)+5*time.Second, func() {
		log.Print("nirilayout: identify overlays outlived their timer; exiting")
		os.Exit(0)
	})
}
