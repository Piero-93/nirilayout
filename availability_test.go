package nirilayout

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// dualExternalLayouts models a laptop (eDP-1) with two external monitors
// (DP-1, HDMI-A-2), including layouts that need both externals at once.
func dualExternalLayouts() []Layout {
	return []Layout{
		{Name: "Laptop Only", Outputs: []*Output{{Name: "eDP-1"}}},
		{Name: "External Right", Outputs: []*Output{{Name: "HDMI-A-2"}, {Name: "eDP-1"}}},
		{Name: "Double External 1", Outputs: []*Output{{Name: "DP-1"}, {Name: "HDMI-A-2"}}},
	}
}

func TestMissingOutputs(t *testing.T) {
	cases := map[string]struct {
		layout    Layout
		connected map[string]bool
		want      []string
	}{
		"all connected": {
			layout:    Layout{Outputs: []*Output{{Name: "DP-1"}, {Name: "HDMI-A-2"}}},
			connected: map[string]bool{"DP-1": true, "HDMI-A-2": true, "eDP-1": true},
			want:      nil,
		},
		"one missing": {
			layout:    Layout{Outputs: []*Output{{Name: "DP-1"}, {Name: "HDMI-A-2"}}},
			connected: map[string]bool{"HDMI-A-2": true},
			want:      []string{"DP-1"},
		},
		"all missing, in layout order": {
			layout:    Layout{Outputs: []*Output{{Name: "DP-1"}, {Name: "HDMI-A-2"}}},
			connected: map[string]bool{"eDP-1": true},
			want:      []string{"DP-1", "HDMI-A-2"},
		},
		"nothing connected": {
			layout:    Layout{Outputs: []*Output{{Name: "eDP-1"}}},
			connected: map[string]bool{},
			want:      []string{"eDP-1"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, missingOutputs(c.layout, c.connected)); diff != "" {
				t.Errorf("missingOutputs mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMarkUnavailable(t *testing.T) {
	t.Run("only one external plugged in", func(t *testing.T) {
		layouts := dualExternalLayouts()
		markUnavailable(layouts, map[string]bool{"eDP-1": true, "HDMI-A-2": true})

		want := map[string]bool{
			"Laptop Only":       false,
			"External Right":    false,
			"Double External 1": true, // needs DP-1
		}
		for _, l := range layouts {
			if l.Unavailable != want[l.Name] {
				t.Errorf("%s: Unavailable = %v, want %v", l.Name, l.Unavailable, want[l.Name])
			}
		}
		if diff := cmp.Diff([]string{"DP-1"}, layouts[2].MissingOutputs); diff != "" {
			t.Errorf("MissingOutputs mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("everything plugged in leaves all layouts available", func(t *testing.T) {
		layouts := dualExternalLayouts()
		markUnavailable(layouts, map[string]bool{"eDP-1": true, "DP-1": true, "HDMI-A-2": true})
		for _, l := range layouts {
			if l.Unavailable {
				t.Errorf("%s marked unavailable with every output connected", l.Name)
			}
			if l.MissingOutputs != nil {
				t.Errorf("%s: MissingOutputs = %v, want none", l.Name, l.MissingOutputs)
			}
		}
	})

	t.Run("nothing connected marks every layout", func(t *testing.T) {
		layouts := dualExternalLayouts()
		markUnavailable(layouts, map[string]bool{})
		for _, l := range layouts {
			if !l.Unavailable {
				t.Errorf("%s not marked unavailable with nothing connected", l.Name)
			}
		}
	})

	// A second pass must not leave stale marks behind: the flag and the missing
	// list are recomputed from scratch, not accumulated.
	t.Run("re-marking clears previous results", func(t *testing.T) {
		layouts := dualExternalLayouts()
		markUnavailable(layouts, map[string]bool{})
		markUnavailable(layouts, map[string]bool{"eDP-1": true, "DP-1": true, "HDMI-A-2": true})
		for _, l := range layouts {
			if l.Unavailable || l.MissingOutputs != nil {
				t.Errorf("%s: stale mark after re-marking: %v %v", l.Name, l.Unavailable, l.MissingOutputs)
			}
		}
	})
}
