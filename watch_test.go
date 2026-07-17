package nirilayout

import (
	"encoding/json"
	"testing"
)

func outputsFromJSON(t *testing.T, s string) map[string]niriOutput {
	t.Helper()
	var m map[string]niriOutput
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("bad test fixture: %v", err)
	}
	return m
}

func TestAnyActive(t *testing.T) {
	cases := map[string]struct {
		json string
		want bool
	}{
		"one active":       {`{"eDP-1":{"logical":{"x":0,"y":0}}}`, true},
		"one off":          {`{"eDP-1":{"logical":null}}`, false},
		"empty":            {`{}`, false},
		"off plus active":  {`{"eDP-1":{"logical":null},"HDMI-A-2":{"logical":{"x":0,"y":0}}}`, true},
		"both off (black)": {`{"eDP-1":{"logical":null},"HDMI-A-2":{"logical":null}}`, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := anyActive(outputsFromJSON(t, c.json)); got != c.want {
				t.Errorf("anyActive = %v, want %v", got, c.want)
			}
		})
	}
}

func TestConnectedNames(t *testing.T) {
	got := connectedNames(outputsFromJSON(t, `{"eDP-1":{"logical":null},"HDMI-A-2":{"logical":{"x":0}}}`))
	if !got["eDP-1"] || !got["HDMI-A-2"] {
		t.Errorf("connectedNames missing a connector: %v", got)
	}
	if got["DP-3"] {
		t.Errorf("connectedNames reported a disconnected output as connected: %v", got)
	}
}

// layouts modelling the user's real setup: eDP-1 is the laptop panel, HDMI-A-2
// the external monitor. parseLayoutFromConfig strips `off` outputs, so
// Outputs holds only the enabled ones.
func testLayouts() []Layout {
	return []Layout{
		{Name: "External Only", Path: "/ext_only", Outputs: []*Output{{Name: "HDMI-A-2"}}},
		{Name: "Laptop Only", Path: "/laptop_only", Outputs: []*Output{{Name: "eDP-1"}}},
		{Name: "External Right", Path: "/ext_right", Outputs: []*Output{{Name: "eDP-1"}, {Name: "HDMI-A-2"}}},
	}
}

func TestPickSafeLayout_AutoPick(t *testing.T) {
	layouts := testLayouts()

	t.Run("only laptop connected picks Laptop Only", func(t *testing.T) {
		l, ok := pickSafeLayout(layouts, map[string]bool{"eDP-1": true}, "")
		if !ok || l.Name != "Laptop Only" {
			t.Fatalf("got (%q, %v), want Laptop Only", l.Name, ok)
		}
	})

	t.Run("both connected prefers the dual layout", func(t *testing.T) {
		l, ok := pickSafeLayout(layouts, map[string]bool{"eDP-1": true, "HDMI-A-2": true}, "")
		if !ok || l.Name != "External Right" {
			t.Fatalf("got (%q, %v), want External Right", l.Name, ok)
		}
	})

	t.Run("only external connected picks External Only", func(t *testing.T) {
		l, ok := pickSafeLayout(layouts, map[string]bool{"HDMI-A-2": true}, "")
		if !ok || l.Name != "External Only" {
			t.Fatalf("got (%q, %v), want External Only", l.Name, ok)
		}
	})

	t.Run("nothing connected finds nothing", func(t *testing.T) {
		if _, ok := pickSafeLayout(layouts, map[string]bool{}, ""); ok {
			t.Fatal("expected no applicable layout")
		}
	})
}

func TestPickSafeLayout_ExplicitFallback(t *testing.T) {
	layouts := testLayouts()

	t.Run("matches by name case-insensitively", func(t *testing.T) {
		l, ok := pickSafeLayout(layouts, nil, "laptop only")
		if !ok || l.Name != "Laptop Only" {
			t.Fatalf("got (%q, %v), want Laptop Only", l.Name, ok)
		}
	})

	t.Run("returns applicable-or-not as named, ignoring connected set", func(t *testing.T) {
		// The user is trusted to name a sensible fallback; the connected set is
		// not consulted for an explicit fallback.
		l, ok := pickSafeLayout(layouts, map[string]bool{}, "External Only")
		if !ok || l.Name != "External Only" {
			t.Fatalf("got (%q, %v), want External Only", l.Name, ok)
		}
	})

	t.Run("unknown name is reported as not found", func(t *testing.T) {
		if _, ok := pickSafeLayout(layouts, nil, "Nonexistent"); ok {
			t.Fatal("expected unknown fallback to be not found")
		}
	})
}
