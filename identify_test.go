package nirilayout

import (
	"testing"
	"time"
)

func TestFormatRefreshHz(t *testing.T) {
	cases := map[int]string{
		144000: "144",
		60049:  "60", // the real value niri reports for a 60 Hz panel
		59951:  "60",
		143856: "143.9",
		74973:  "75",
		0:      "",
		-1:     "",
	}
	for mHz, want := range cases {
		if got := formatRefreshHz(mHz); got != want {
			t.Errorf("formatRefreshHz(%d) = %q, want %q", mHz, got, want)
		}
	}
}

func TestFormatScale(t *testing.T) {
	cases := map[float64]string{
		1.0:                "1",
		1.5:                "1.5",
		2.0:                "2",
		1.25:               "1.25",
		1.5000000000000002: "1.5", // float noise must not reach the screen
		0:                  "",
		-1:                 "",
	}
	for scale, want := range cases {
		if got := formatScale(scale); got != want {
			t.Errorf("formatScale(%v) = %q, want %q", scale, got, want)
		}
	}
}

func TestClampIdentifyDuration(t *testing.T) {
	cases := map[time.Duration]time.Duration{
		3 * time.Second:         3 * time.Second,
		1500 * time.Millisecond: 1500 * time.Millisecond,
		0:                       identifyDefaultDuration,
		-1:                      identifyDefaultDuration,
		50 * time.Millisecond:   identifyMinDuration,
		10 * time.Minute:        identifyMaxDuration,
	}
	for in, want := range cases {
		if got := clampIdentifyDuration(in); got != want {
			t.Errorf("clampIdentifyDuration(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestResolveMode(t *testing.T) {
	cases := map[string]struct {
		watch, identify bool
		want            Mode
		wantErr         bool
	}{
		"default":          {false, false, ModePicker, false},
		"watch":            {true, false, ModeWatch, false},
		"identify":         {false, true, ModeIdentify, false},
		"both is nonsense": {true, true, ModePicker, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := resolveMode(c.watch, c.identify)
			if (err != nil) != c.wantErr {
				t.Fatalf("resolveMode(%v, %v) err = %v, wantErr %v", c.watch, c.identify, err, c.wantErr)
			}
			if !c.wantErr && got != c.want {
				t.Errorf("resolveMode(%v, %v) = %v, want %v", c.watch, c.identify, got, c.want)
			}
		})
	}
}

// The real shape of a `niri msg -j outputs` entry, trimmed to the fields the
// overlay reads.
const identifyRealOutput = `{"eDP-1":{"name":"eDP-1","make":"AU Optronics","model":"0x403D",
	"modes":[{"width":1920,"height":1080,"refresh_rate":60049,"is_preferred":true}],
	"current_mode":0,
	"logical":{"x":0,"y":0,"width":1920,"height":1080,"scale":1.0,"transform":"Normal"}}}`

func TestNiriOutputInfoDetailLine(t *testing.T) {
	cases := map[string]struct {
		json      string
		connector string
		want      string
	}{
		"real single-mode output": {identifyRealOutput, "eDP-1", "1920x1080@60  1×"},
		"picks the current mode": {
			`{"DP-2":{"modes":[{"width":1920,"height":1080,"refresh_rate":60000},
				{"width":2560,"height":1440,"refresh_rate":143856}],
				"current_mode":1,"logical":{"scale":1.5}}}`,
			"DP-2", "2560x1440@143.9  1.5×",
		},
		// Connected but off: niri reports no current mode and no logical
		// region, so the overlay text falls back to what GDK knows.
		"off output has no mode": {
			`{"HDMI-A-2":{"modes":[{"width":1920,"height":1080,"refresh_rate":60000}],
				"current_mode":null,"logical":null}}`,
			"HDMI-A-2", "",
		},
		"current_mode out of range": {
			`{"DP-3":{"modes":[{"width":1920,"height":1080,"refresh_rate":60000}],"current_mode":5}}`,
			"DP-3", "",
		},
		"empty entry": {`{"DP-4":{}}`, "DP-4", ""},
		"zero refresh rate omits the segment": {
			`{"DP-5":{"modes":[{"width":1920,"height":1080,"refresh_rate":0}],
				"current_mode":0,"logical":{"scale":1.0}}}`,
			"DP-5", "1920x1080  1×",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			infos := niriOutputInfo(outputsFromJSON(t, c.json))
			info, ok := infos[c.connector]
			if !ok {
				t.Fatalf("niriOutputInfo dropped connector %q: %v", c.connector, infos)
			}
			if got := info.DetailLine(); got != c.want {
				t.Errorf("DetailLine() = %q, want %q", got, c.want)
			}
			if info.Connector != c.connector {
				t.Errorf("Connector = %q, want %q", info.Connector, c.connector)
			}
		})
	}
}

func TestNiriOutputInfoNil(t *testing.T) {
	// A failed niri query degrades to a nil map, which must stay a legal
	// lookup rather than needing a guard at every call site.
	if got := niriOutputInfo(nil); got != nil {
		t.Errorf("niriOutputInfo(nil) = %v, want nil", got)
	}
	if _, ok := niriOutputInfo(nil)["eDP-1"]; ok {
		t.Error("lookup in a nil info map reported a hit")
	}
}

func TestHeadline(t *testing.T) {
	cases := map[string]struct {
		info OutputInfo
		want string
	}{
		"connector wins":       {OutputInfo{Connector: "eDP-1", Description: "AU Optronics"}, "eDP-1"},
		"description fallback": {OutputInfo{Description: "AU Optronics - 0x403D"}, "AU Optronics - 0x403D"},
		"ordinal last resort":  {OutputInfo{Index: 1}, "Monitor 2"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := c.info.Headline(); got != c.want {
				t.Errorf("Headline() = %q, want %q", got, c.want)
			}
		})
	}
}
