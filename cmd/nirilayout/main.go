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
		fmt.Print("  https://github.com/calico32/nirilayout/blob/main/README.md\n")
	}

	flag.Parse()

	// Apply -lang and -lowercase now that flags are parsed.
	nirilayout.InitI18n()

	var layouts []nirilayout.Layout

	configDir, err := nirilayout.GetNiriConfigDir()
	if err == nil {
		layouts, err = nirilayout.GatherLayouts(configDir)
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
