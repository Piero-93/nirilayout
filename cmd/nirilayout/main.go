package main

import (
	"flag"
	"fmt"
	"nirilayout"
	"os"
	"slices"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func main() {
	// Initialize i18n from the system locale first so that flag.Usage (which
	// flag.Parse may invoke on -h or a parse error) is already localized. It
	// is re-initialized after flag.Parse to honor -lang and -lowercase.
	nirilayout.InitI18n()

	flag.Usage = func() {
		fmt.Print(nirilayout.T("nirilayout is a layout switcher for niri.\n\nCommand-line options:\n"))
		// Translate each flag's usage string in place, then let the flag
		// package handle the formatting (types, defaults, alignment).
		flag.VisitAll(func(f *flag.Flag) {
			f.Usage = nirilayout.T(f.Usage)
		})
		flag.PrintDefaults()
		fmt.Print(nirilayout.T("\nTo use nirilayout, create layouts in files called ~/.config/niri/layout_<name>.kdl and run nirilayout.\nSee the README for more details:\n"))
		fmt.Print("  https://github.com/Piero-93/nirilayout/blob/main/README.md\n")
	}

	flag.Parse()

	// Apply -lang and -lowercase now that flags are parsed.
	nirilayout.InitI18n()

	mode, err := nirilayout.ResolveMode()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	// Identify mode is a one-shot overlay naming each monitor. It needs no
	// layouts, so it runs before they are gathered rather than paying for a
	// list nobody will see.
	//
	// It gets its own application id: the picker's is single-instance, so an
	// identify launched while a picker is open would otherwise just tell the
	// running instance to activate — reopening the picker, since flags are not
	// forwarded. Staying unique under its own id means two rapid identifies
	// dedupe into one, whose timer simply restarts.
	if mode == nirilayout.ModeIdentify {
		app := gtk.NewApplication("co.calebc.nirilayout.identify", gio.ApplicationDefaultFlags)
		app.ConnectActivate(func() { nirilayout.RunIdentify(app) })
		if code := app.Run(nil); code > 0 {
			os.Exit(code)
		}
		return
	}

	var layouts []nirilayout.Layout

	configDir, err := nirilayout.GetNiriConfigDir()
	if err == nil {
		layouts, err = nirilayout.GatherLayouts(configDir)
	}

	// Watch mode runs a headless daemon instead of the GUI: it reactivates a
	// working output whenever the active one disappears (cable unplug or boot
	// with no external monitor). It needs the layouts to recover to, so a
	// gather error here is fatal rather than a fall-through to the picker.
	if mode == nirilayout.ModeWatch {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if werr := nirilayout.RunWatch(configDir, layouts); werr != nil {
			fmt.Fprintln(os.Stderr, werr)
			os.Exit(1)
		}
		return
	}

	// Layouts that need a monitor which is not plugged in can be dimmed or
	// hidden, depending on the flags. Off by default: every layout stays
	// selectable, as before. Must run before the current layout is located,
	// since hiding renumbers the list.
	if err == nil {
		layouts = nirilayout.ApplyAvailability(layouts)
	}

	// Preselect the active layout. Detection works whether nirilayout.kdl is a
	// symlink (classic setup) or a regular file (e.g. noctalia 5); if it can't
	// be determined we just fall back to the first layout, never an error.
	index := 0
	if err == nil {
		current := nirilayout.CurrentLayoutPath(configDir, layouts)
		if i := slices.IndexFunc(layouts, func(l nirilayout.Layout) bool {
			return l.Path == current
		}); i != -1 {
			index = i
		}
	}

	app := gtk.NewApplication("co.calebc.nirilayout", gio.ApplicationDefaultFlags)
	app.ConnectActivate(func() {
		nirilayout.Run(app, layouts, index, err)
	})

	if code := app.Run(nil); code > 0 {
		os.Exit(code)
	}
}
