package nirilayout

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/calico32/kdl-go"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/google/go-cmp/cmp"
)

var configTestCases = []struct {
	name   string
	config string
	want   *Layout // nil == want err
}{
	{
		name:   "empty",
		config: "",
		want:   nil,
	},
	{
		name: "all fields",
		config: `
			//! name "test"
			//! shortcut "abc" "d"
			//! style fill="#aaa" border="#bbb" text="#ccc" border-width=11 font-family="Test Font" font-size=22 font-weight="bold" font-style="italic" hide-details=true line-spacing=33

			output "output1" {
				//! name "output name override"
				//! style fill="#ddd" border="#eee" text="#fff" border-width=44 font-family="Test Font 2" font-size=55 font-weight="bold" font-style="oblique" hide-details=true line-spacing=66
				scale 1.5
				transform "flipped-90"
				position x=11 y=22
				mode "1920x1080@59.960"
				modeline 173.00 1920 2048 2248 2576 1080 1083 1088 1120 "-hsync" "+vsync"
			}
		`,
		want: &Layout{
			Name:      "test",
			Shortcuts: []string{"abc", "d"},
			DefaultStyle: outputStyleConfig{
				Fill:        kdl.NewString("#aaa"),
				Border:      kdl.NewString("#bbb"),
				Text:        kdl.NewString("#ccc"),
				BorderWidth: ptr(11),
				FontFamily:  "Test Font",
				FontSize:    22,
				FontWeight:  kdl.NewString("bold"),
				FontStyle:   kdl.NewString("italic"),
				HideDetails: ptr(true),
				LineSpacing: ptr(33),
			},
			Outputs: []*Output{
				{
					Name:         "output1",
					NameOverride: "output name override",
					Style: outputStyleConfig{
						Fill:        kdl.NewString("#ddd"),
						Border:      kdl.NewString("#eee"),
						Text:        kdl.NewString("#fff"),
						BorderWidth: ptr(44),
						FontFamily:  "Test Font 2",
						FontSize:    55,
						FontWeight:  kdl.NewString("bold"),
						FontStyle:   kdl.NewString("oblique"),
						HideDetails: ptr(true),
						LineSpacing: ptr(66),
					},
					Scale:     1.5,
					Transform: "flipped-90",
					Position: &Position{
						X: 11,
						Y: 22,
					},
					Mode: "1920x1080@59.960",
					Modeline: Modeline{
						DotClock:   173.00,
						HDisplay:   1920,
						HSyncStart: 2048,
						HSyncEnd:   2248,
						HTotal:     2576,
						VDisplay:   1080,
						VSyncStart: 1083,
						VSyncEnd:   1088,
						VTotal:     1120,
						Flags:      []string{"-hsync", "+vsync"},
					},
					resolvedStyle: outputStyle{
						fillColor:   mustParseHexColor("#ddd"),
						borderColor: mustParseHexColor("#eee"),
						textColor:   mustParseHexColor("#fff"),
						borderWidth: 44,
						fontFamily:  "Test Font 2",
						fontSize:    55,
						fontWeight:  cairo.FontWeightBold,
						fontStyle:   cairo.FontSlantOblique,
						hideDetails: true,
						lineSpacing: 66,
					},
				},
			},
		},
	},
	{
		name: "no mode or modeline",
		config: `
			output "output1" {
				scale 1.5
				position x=11 y=22
			}
		`,
		want: nil,
	},
	{
		name: "legacy color index",
		config: `
			//! name "test"
			output "output1" {
				//! color 4
				mode "1920x1080@59.960"
			}
		`,
		want: &Layout{
			Name: "test",
			Outputs: []*Output{
				{
					Name:        "output1",
					LegacyColor: ptr(4),
					Mode:        "1920x1080@59.960",
					resolvedStyle: outputStyle{
						fillColor:   fillColors[4],
						borderColor: borderColors[4],
						textColor:   lightTextColor,
						borderWidth: defaultBorderWidth,
						fontFamily:  defaultFontFamily,
						fontSize:    defaultFontSize,
						fontWeight:  defaultFontWeight,
						fontStyle:   defaultFontStyle,
						hideDetails: false,
						lineSpacing: defaultLineSpacing,
					},
				},
			},
		},
	},
	{
		name: "style inheritance",
		config: `
			//! name "test"
			//! style fill="#111" border="#222" text="#333" border-width=11 font-family="Test Font" font-size=22 font-weight="bold" font-style="italic" hide-details=true line-spacing=33

			output "output1" {
				mode "1920x1080@59.960"
			}
		`,
		want: &Layout{
			Name: "test",
			DefaultStyle: outputStyleConfig{
				Fill:        kdl.NewString("#111"),
				Border:      kdl.NewString("#222"),
				Text:        kdl.NewString("#333"),
				BorderWidth: ptr(11),
				FontFamily:  "Test Font",
				FontSize:    22,
				FontWeight:  kdl.NewString("bold"),
				FontStyle:   kdl.NewString("italic"),
				HideDetails: ptr(true),
				LineSpacing: ptr(33),
			},
			Outputs: []*Output{
				{
					Name: "output1",
					Mode: "1920x1080@59.960",
					resolvedStyle: outputStyle{
						fillColor:   mustParseHexColor("#111"),
						borderColor: mustParseHexColor("#222"),
						textColor:   mustParseHexColor("#333"),
						borderWidth: 11,
						fontFamily:  "Test Font",
						fontSize:    22,
						fontWeight:  cairo.FontWeightBold,
						fontStyle:   cairo.FontSlantItalic,
						hideDetails: true,
						lineSpacing: 33,
					},
				},
			},
		},
	},
	{
		name: "auto border and text color",
		config: `
			//! name "test"
			//! style fill="#fafafa"

			output "output1" {
				//! style fill="#0a0a0a"
				mode "1920x1080@59.960"
			}
			output "output2" {
				mode "1920x1080@59.960"
			}
		`,
		want: &Layout{
			Name: "test",
			DefaultStyle: outputStyleConfig{
				Fill: kdl.NewString("#fafafa"),
			},
			Outputs: []*Output{
				{
					Name: "output1",
					Style: outputStyleConfig{
						Fill: kdl.NewString("#0a0a0a"),
					},
					Mode: "1920x1080@59.960",
					resolvedStyle: outputStyle{
						fillColor:   mustParseHexColor("#0a0a0a"),
						borderColor: pickBorderColor(mustParseHexColor("#0a0a0a")),
						textColor:   lightTextColor,
						borderWidth: defaultBorderWidth,
						fontFamily:  defaultFontFamily,
						fontSize:    defaultFontSize,
						fontWeight:  defaultFontWeight,
						fontStyle:   defaultFontStyle,
						hideDetails: false,
						lineSpacing: defaultLineSpacing,
					},
				},
				{
					Name: "output2",
					Mode: "1920x1080@59.960",
					resolvedStyle: outputStyle{
						fillColor:   mustParseHexColor("#fafafa"),
						borderColor: pickBorderColor(mustParseHexColor("#fafafa")),
						textColor:   darkTextColor,
						borderWidth: defaultBorderWidth,
						fontFamily:  defaultFontFamily,
						fontSize:    defaultFontSize,
						fontWeight:  defaultFontWeight,
						fontStyle:   defaultFontStyle,
						hideDetails: false,
						lineSpacing: defaultLineSpacing,
					},
				},
			},
		},
	},
}

func TestParseConfig(t *testing.T) {

	for _, tt := range configTestCases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLayoutFromConfig("input", []byte(tt.config))
			if tt.want == nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(*tt.want, got,
				cmp.AllowUnexported(Output{}, outputStyle{}),
			); diff != "" {
				t.Errorf("layout does not match (-want +got):\n%s", diff)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}

func mustParseHexColor(hex string) color.RGBA {
	c, err := parseHexColor(hex)
	if err != nil {
		panic(fmt.Sprintf("mustParseHexColor: invalid hex color %q: %v", hex, err))
	}
	return c
}

func FuzzParseConfig(f *testing.F) {
	for _, tt := range configTestCases {
		f.Add(tt.config)
	}
	f.Fuzz(func(t *testing.T, config string) {
		// don't care about the output, just want to make sure it doesn't panic
		_, _ = parseLayoutFromConfig("input", []byte(config))
	})
}
