package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"nirilayout"
	"os"
	"path/filepath"
	"slices"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func main() {
	flag.Usage = func() {
		fmt.Printf("nirilayout is a layout switcher for niri.\n\nCommand-line options:\n")
		flag.PrintDefaults()
		fmt.Printf("\nTo use nirilayout, create layouts in files called ~/.config/niri/layout_<name>.kdl and run nirilayout.\nSee the README for more details:\n")
		fmt.Printf("  https://github.com/calico32/nirilayout/blob/main/README.md\n")
	}

	flag.Parse()

	var layouts []nirilayout.Layout
	var current string

	configDir, err := nirilayout.GetNiriConfigDir()
	if err == nil {
		layouts, err = nirilayout.GatherLayouts(configDir)
	}

	if err == nil {
		current, err = os.Readlink(filepath.Join(configDir, "nirilayout.kdl"))
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
	}

	index := slices.IndexFunc(layouts, func(l nirilayout.Layout) bool {
		return l.Path == current
	})
	if index == -1 {
		index = 0
	}

	app := gtk.NewApplication("co.calebc.nirilayout", gio.ApplicationDefaultFlags)
	app.ConnectActivate(func() {
		nirilayout.Run(app, layouts, index, err)
	})

	if code := app.Run(nil); code > 0 {
		os.Exit(code)
	}
}
