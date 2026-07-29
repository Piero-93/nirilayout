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

func layoutNamesOf(layouts []Layout) []string {
	names := make([]string, 0, len(layouts))
	for _, l := range layouts {
		names = append(names, l.Name)
	}
	return names
}

func TestWithoutUnavailable(t *testing.T) {
	t.Run("keeps applicable layouts in order and counts the rest", func(t *testing.T) {
		hiddenLayoutCount = 0
		layouts := dualExternalLayouts()
		markUnavailable(layouts, map[string]bool{"eDP-1": true, "HDMI-A-2": true})

		got := withoutUnavailable(layouts)
		want := []string{"Laptop Only", "External Right"}
		if diff := cmp.Diff(want, layoutNamesOf(got)); diff != "" {
			t.Errorf("kept layouts mismatch (-want +got):\n%s", diff)
		}
		if hiddenLayoutCount != 1 {
			t.Errorf("hiddenLayoutCount = %d, want 1", hiddenLayoutCount)
		}
	})

	t.Run("nothing connected hides everything", func(t *testing.T) {
		hiddenLayoutCount = 0
		layouts := dualExternalLayouts()
		markUnavailable(layouts, map[string]bool{})

		if got := withoutUnavailable(layouts); len(got) != 0 {
			t.Errorf("kept %v, want none", layoutNamesOf(got))
		}
		// The GUI relies on this to explain an empty picker.
		if hiddenLayoutCount != 3 {
			t.Errorf("hiddenLayoutCount = %d, want 3", hiddenLayoutCount)
		}
	})
}

func TestResolveAvailability(t *testing.T) {
	// Only the laptop and one of the two externals are plugged in, so
	// "Double External 1" (which needs DP-1) is the odd one out.
	connected := map[string]bool{"eDP-1": true, "HDMI-A-2": true}

	t.Run("neither flag leaves the layouts untouched", func(t *testing.T) {
		layouts := dualExternalLayouts()
		got := resolveAvailability(layouts, false, false, connected)
		if len(got) != 3 {
			t.Fatalf("got %v, want all three layouts", layoutNamesOf(got))
		}
		for _, l := range got {
			if l.Unavailable {
				t.Errorf("%s marked unavailable without either flag", l.Name)
			}
		}
	})

	t.Run("dim keeps every layout but marks the unusable one", func(t *testing.T) {
		layouts := dualExternalLayouts()
		got := resolveAvailability(layouts, true, false, connected)
		if len(got) != 3 {
			t.Fatalf("got %v, want all three layouts", layoutNamesOf(got))
		}
		if !got[2].Unavailable {
			t.Errorf("%s not marked unavailable", got[2].Name)
		}
	})

	t.Run("hide drops the unusable one", func(t *testing.T) {
		layouts := dualExternalLayouts()
		got := resolveAvailability(layouts, false, true, connected)
		want := []string{"Laptop Only", "External Right"}
		if diff := cmp.Diff(want, layoutNamesOf(got)); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	// Hiding wins: a dropped layout cannot also be shown dimmed.
	t.Run("both flags hide rather than dim", func(t *testing.T) {
		layouts := dualExternalLayouts()
		got := resolveAvailability(layouts, true, true, connected)
		want := []string{"Laptop Only", "External Right"}
		if diff := cmp.Diff(want, layoutNamesOf(got)); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
		for _, l := range got {
			if l.Unavailable {
				t.Errorf("%s survived hiding while marked unavailable", l.Name)
			}
		}
	})
}
