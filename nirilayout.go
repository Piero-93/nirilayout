package nirilayout

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const version = `nirilayout v0.4.0`

//go:embed style.css
var appStylesheet string

func loadStylesheet(content string) *gtk.CSSProvider {
	prov := gtk.NewCSSProvider()
	prov.ConnectParsingError(func(sec *gtk.CSSSection, err error) {
		loc := sec.StartLocation()
		lines := strings.Split(content, "\n")
		log.Printf("CSS error (%v) at line: %q", err, lines[loc.Lines()])
	})
	prov.LoadFromString(content)
	return prov
}

// LoadStyles installs the default stylesheet and the user's overrides. Every
// mode that puts something on screen must call it: style.css hides windows
// until they get the "visible" class, so a window shown without it is mapped
// but invisible.
//
// It runs once per process. A second activation of an already-running instance
// calls it again, and stacking another copy of the same providers on the
// display would only cost work.
var LoadStyles = sync.OnceFunc(func() {
	// load default stylesheet
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(), loadStylesheet(appStylesheet),
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
	)

	// load ~/.config/niri/nirilayout.css if it exists, to allow user overrides of the default stylesheet
	configDir, configErr := GetNiriConfigDir()
	if configErr == nil {
		userStylesheetPath := filepath.Join(configDir, "nirilayout.css")
		if _, err := os.Stat(userStylesheetPath); err == nil {
			content, err := os.ReadFile(userStylesheetPath)
			if err != nil {
				log.Printf("Error reading user stylesheet: %v", err)
			} else {
				gtk.StyleContextAddProviderForDisplay(
					gdk.DisplayGetDefault(), loadStylesheet(string(content)),
					gtk.STYLE_PROVIDER_PRIORITY_USER,
				)
			}
		}
	}
})

func Run(app *gtk.Application, layouts []Layout, startIndex int, err error) {
	LoadStyles()

	win := gtk.NewApplicationWindow(app)
	win.SetTitle("nirilayout")

	gtk4layershell.InitForWindow(&win.Window)
	gtk4layershell.SetLayer(&win.Window, gtk4layershell.LayerShellLayerOverlay)
	gtk4layershell.SetKeyboardMode(&win.Window, gtk4layershell.LayerShellKeyboardModeExclusive)
	gtk4layershell.SetMargin(&win.Window, gtk4layershell.LayerShellEdgeTop, 10)
	gtk4layershell.SetNamespace(&win.Window, "nirilayout")

	root := gtk.NewBox(gtk.OrientationVertical, 16)
	// The window is a transparent tray; this box is the visible panel, drawn
	// with the same raised card look as the identify overlays.
	root.AddCSSClass("card")
	root.AddCSSClass("panel")
	win.SetChild(root)

	quit := func() {
		// The overlays are separate windows; they must not outlive the picker
		// that put them there.
		DismissIdentify()
		win.RemoveCSSClass("visible")
		glib.TimeoutAdd(75, func() bool {
			app.Quit()
			return false
		})
	}

	var selector *gtk.FlowBox

	if err != nil {
		label := gtk.NewLabel(Tf("Error loading layouts: %v", err))
		label.SetHAlign(gtk.AlignCenter)
		root.Append(label)
	} else if len(layouts) == 0 {
		// With -hide-unavailable the picker can be empty even though layouts
		// exist, so say which of the two situations this is.
		msg := T("No layouts found. Please create layout_<name>.kdl files in ~/.config/niri to use nirilayout.")
		if hiddenLayoutCount > 0 {
			msg = T("No layout matches the connected outputs. Plug a monitor back in, or run without -hide-unavailable to see every layout.")
		}
		label := gtk.NewLabel(msg)
		label.SetHAlign(gtk.AlignCenter)
		root.Append(label)
	} else {
		selector = gtk.NewFlowBox()
		selector.SetColumnSpacing(8)
		selector.SetRowSpacing(8)
		selector.SetMaxChildrenPerLine(5)
		selector.SetOrientation(gtk.OrientationHorizontal)
		selector.SetSelectionMode(gtk.SelectionNone)
		selector.SetActivateOnSingleClick(true)
		root.Append(selector)
	}

	// An unavailable layout (one whose outputs are not all connected, see
	// -dim-unavailable) stays in the grid but cannot be applied: it is dimmed,
	// skipped by the arrow keys, and ignored by the search box.
	selectable := func(i int) bool {
		return i >= 0 && i < len(layouts) && !layouts[i].Unavailable
	}

	// The current layout keeps its marker even when it is unavailable (the
	// monitor was unplugged after it was applied), but the selection must start
	// somewhere Return can act on.
	currentIndex := startIndex
	if !selectable(startIndex) {
		if i := slices.IndexFunc(layouts, func(l Layout) bool { return !l.Unavailable }); i != -1 {
			startIndex = i
		}
	}

	for i, layout := range layouts {
		if selector == nil {
			break
		}
		button := gtk.NewButton()
		b := gtk.NewBox(gtk.OrientationVertical, 8)
		container := gtk.NewCenterBox()
		container.SetSizeRequest(drawingSize, drawingSize)
		preview := drawLayout(layout)
		preview.SetHAlign(gtk.AlignCenter)
		preview.SetVAlign(gtk.AlignCenter)
		container.SetCenterWidget(preview)
		b.Append(container)

		if len(layout.Shortcuts) == 0 {
			b.Append(gtk.NewLabel(layout.Name))
		} else {
			b.Append(gtk.NewLabel(fmt.Sprintf("%v %s", layout.Shortcuts, layout.Name)))
		}

		button.SetChild(b)

		if layout.Unavailable {
			// Say which connector is missing, so a layout that suddenly cannot
			// be picked explains itself instead of just looking broken.
			missing := gtk.NewLabel(Tf("%s not connected", strings.Join(layout.MissingOutputs, ", ")))
			missing.AddCSSClass("missing-outputs")
			b.Append(missing)

			button.AddCSSClass("unavailable")
			button.SetSensitive(false)
		} else {
			button.ConnectClicked(func() {
				SetCurrentLayout(layout)
				app.Quit()
			})

			button.SetCursorFromName("pointer")
		}

		selector.Insert(button, -1)
		if i == startIndex {
			button.AddCSSClass("selected")
		}
		if i == currentIndex {
			button.AddCSSClass("current")
		}
	}

	inputBox := gtk.NewCenterBox()

	input := gtk.NewEntry()
	input.SetSizeRequest(400, 0)
	if *leftAlignFlag {
		input.SetAlignment(0)
	} else {
		input.SetAlignment(0.5)
	}
	input.SetPlaceholderText(T("Name or shortcut…"))
	input.ConnectChanged(func() {
		text := input.Text()
		for _, layout := range layouts {
			if layout.Unavailable {
				continue
			}
			for _, shortcut := range layout.Shortcuts {
				if text == shortcut {
					SetCurrentLayout(layout)
					app.Quit()
				}
			}
			if text == layout.Name {
				SetCurrentLayout(layout)
				quit()
			}
		}
	})
	label := gtk.NewLabel(version)
	label.SetSensitive(false)
	label.SetMarginEnd(16)
	inputBox.SetStartWidget(label)
	label = gtk.NewLabel(T("F1 to identify monitors · Esc to quit"))
	label.SetSensitive(false)
	label.SetMarginStart(16)
	inputBox.SetEndWidget(label)

	inputBox.SetCenterWidget(input)

	root.Append(inputBox)

	win.ConnectShow(func() {
		input.GrabFocus()
		win.AddCSSClass("visible")
	})

	index := startIndex

	setActiveLayout := func(i int) {
		if len(layouts) == 0 {
			return
		}
		for j := 0; selector.ChildAtIndex(j) != nil; j++ {
			button := selector.ChildAtIndex(j).Child().(*gtk.Button)
			if j == i {
				button.AddCSSClass("selected")
			} else {
				button.RemoveCSSClass("selected")
			}
		}
		if len(layouts) == 0 {
			return
		}
	}

	// nextSelectable returns the first selectable layout starting at `from` and
	// walking by `delta`, wrapping around the grid, or -1 when none is
	// selectable. Navigation uses it so the arrow keys skip dimmed layouts
	// instead of parking the selection on one that Return cannot apply.
	nextSelectable := func(from, delta int) int {
		n := len(layouts)
		if n == 0 {
			return -1
		}
		for i := 0; i < n; i++ {
			j := ((from+delta*i)%n + n) % n
			if selectable(j) {
				return j
			}
		}
		return -1
	}

	k := gtk.NewEventControllerKey()
	k.SetPropagationPhase(gtk.PhaseCapture)
	k.ConnectKeyPressed(func(keyval uint, keycode uint, state gdk.ModifierType) bool {
		switch keyval {
		case gdk.KEY_Escape:
			quit()
			return true
		case gdk.KEY_Right:
			if j := nextSelectable(index+1, 1); j != -1 {
				index = j
			}
			setActiveLayout(index)
			return true
		case gdk.KEY_Left:
			if j := nextSelectable(index-1, -1); j != -1 {
				index = j
			}
			setActiveLayout(index)
			return true
		case gdk.KEY_Up:
			// Vertical movement does not wrap; if every layout above the target
			// row is unavailable, the selection stays where it is.
			skip := int(selector.MaxChildrenPerLine())
			for j := index - skip; j >= 0; j-- {
				if selectable(j) {
					index = j
					break
				}
			}
			setActiveLayout(index)
			return true
		case gdk.KEY_Down:
			skip := int(selector.MaxChildrenPerLine())
			for j := index + skip; j < len(layouts); j++ {
				if selectable(j) {
					index = j
					break
				}
			}
			setActiveLayout(index)
			return true
		case gdk.KEY_Return:
			if len(layouts) != 0 && selectable(index) {
				SetCurrentLayout(layouts[index])
			}
			quit()
			return true
		}
		return false
	})
	input.AddController(k)

	// Identify is bound on the window rather than on the search box, in the
	// capture phase, so it fires wherever the focus happens to be.
	//
	// F1 and Ctrl+I, never a bare "i": the controller above swallows the keys
	// it handles, so binding the letter would make it impossible to type any
	// layout name containing an "i".
	ik := gtk.NewEventControllerKey()
	ik.SetPropagationPhase(gtk.PhaseCapture)
	ik.ConnectKeyPressed(func(keyval uint, keycode uint, state gdk.ModifierType) bool {
		switch {
		case keyval == gdk.KEY_F1,
			(keyval == gdk.KEY_i || keyval == gdk.KEY_I) && state&gdk.ControlMask != 0:
			ShowIdentify(app, false)
			return true
		case keyval == gdk.KEY_Escape:
			// While the overlays are up, Esc puts them away; a second Esc
			// closes the picker as before.
			return DismissIdentify()
		}
		return false
	})
	win.AddController(ik)

	win.SetVisible(true)

	if IdentifyOnOpen() {
		// Through an idle callback so the picker maps first and the overlays
		// end up stacked on top of it.
		glib.IdleAdd(func() bool {
			ShowIdentify(app, false)
			return false
		})
	}
}
