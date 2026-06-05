package main

import (
	"errors"
	"flag"
	"io/fs"
	"nirilayout"
	"os"
	"path/filepath"
	"slices"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func main() {
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
