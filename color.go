package nirilayout

import (
	"fmt"
	"hash/fnv"
	"image/color"
	"math"
	"strconv"
	"strings"
)

var fillColors = []color.RGBA{
	gray600,
	red700, orange700, amber700, yellow700,
	lime700, green700, emerald700, teal700,
	cyan700, sky700, blue700, indigo700,
	violet700, purple700, fuchsia700, pink700,
	rose700,
}

var borderColors = []color.RGBA{
	gray400,
	red500, orange500, amber500, yellow500,
	lime500, green500, emerald500, teal500,
	cyan500, sky500, blue500, indigo500,
	violet500, purple500, fuchsia500, pink500,
	rose500,
}

func pickWindowColors(name string) (fill, border color.RGBA) {
	h := fnv.New32a()
	h.Write([]byte(name))
	index := int(h.Sum32()) % len(fillColors)
	return fillColors[index], borderColors[index]
}

func rgba(color color.Color) (r, g, b, a float64) {
	rw, gw, bw, aw := color.RGBA()
	r = float64(rw) / 0xffff
	g = float64(gw) / 0xffff
	b = float64(bw) / 0xffff
	a = float64(aw) / 0xffff
	return
}

func parseColor(s string) (color.RGBA, error) {
	if c, ok := colorNames[s]; ok {
		return c, nil
	}

	if s[0] == '#' {
		return parseHexColor(s)
	}

	fnName, argsStr, isFn := strings.Cut(s, "(")
	if isFn {
		argsStr, closed := strings.CutSuffix(argsStr, ")")
		if !closed {
			return color.RGBA{}, fmt.Errorf("invalid color function: missing closing parenthesis")
		}
		args := strings.Split(argsStr, ",")
		for i := range args {
			args[i] = strings.TrimSpace(args[i])
		}

		return parseColorFunction(fnName, args)
	}

	return color.RGBA{}, fmt.Errorf("unknown color format: %q", s)
}

func parseHexColor(s string) (color.RGBA, error) {
	c := color.RGBA{A: 255}
	switch len(s) {
	case 4: // #RGB
		r, g, b := hexToByte(s[1]), hexToByte(s[2]), hexToByte(s[3])
		if r < 0 || g < 0 || b < 0 {
			return color.RGBA{}, fmt.Errorf("invalid hex digits in color: %q", s)
		}
		c.R, c.G, c.B = uint8(r*17), uint8(g*17), uint8(b*17)
	case 7: // #RRGGBB
		r, g, b := hexToByte(s[1])*16+hexToByte(s[2]), hexToByte(s[3])*16+hexToByte(s[4]), hexToByte(s[5])*16+hexToByte(s[6])
		if r < 0 || g < 0 || b < 0 {
			return color.RGBA{}, fmt.Errorf("invalid hex digits in color: %q", s)
		}
		c.R, c.G, c.B = uint8(r), uint8(g), uint8(b)
	case 9: // #RRGGBBAA
		r, g, b, a := hexToByte(s[1])*16+hexToByte(s[2]), hexToByte(s[3])*16+hexToByte(s[4]), hexToByte(s[5])*16+hexToByte(s[6]), hexToByte(s[7])*16+hexToByte(s[8])
		if r < 0 || g < 0 || b < 0 || a < 0 {
			return color.RGBA{}, fmt.Errorf("invalid hex digits in color: %q", s)
		}
		c.R, c.G, c.B, c.A = uint8(r), uint8(g), uint8(b), uint8(a)
	}
	return c, nil
}

func hexToByte(b byte) int {
	switch {
	case '0' <= b && b <= '9':
		return int(b - '0')
	case 'a' <= b && b <= 'f':
		return int(b - 'a' + 10)
	case 'A' <= b && b <= 'F':
		return int(b - 'A' + 10)
	}
	return math.MinInt
}

func parseColorFunction(fnName string, args []string) (color.RGBA, error) {
	switch fnName {
	case "rgb":
		if len(args) != 3 {
			return color.RGBA{}, fmt.Errorf("rgb() expects 3 arguments, got %d", len(args))
		}
		r, okr := parseUint8(args[0])
		g, okg := parseUint8(args[1])
		b, okb := parseUint8(args[2])
		if !okr || !okg || !okb {
			return color.RGBA{}, fmt.Errorf("invalid color components in rgb(): %v", args)
		}
		return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, nil
	case "rgba":
		if len(args) != 4 {
			return color.RGBA{}, fmt.Errorf("rgba() expects 4 arguments, got %d", len(args))
		}
		r, okr := parseUint8(args[0])
		g, okg := parseUint8(args[1])
		b, okb := parseUint8(args[2])
		a, oka := parseUint8(args[3])
		if !okr || !okg || !okb || !oka {
			return color.RGBA{}, fmt.Errorf("invalid color components in rgba(): %v", args)
		}
		return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, nil
	}

	return color.RGBA{}, fmt.Errorf("unknown color function: %q", fnName)
}

func parseUint8(s string) (uint8, bool) {
	i, err := strconv.ParseInt(s, 0, 64)
	return uint8(i), err == nil && i >= 0 && i <= 255
}

func clampUint8[T int | float64](v T) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}

	return uint8(v)
}

func applyBrightness(c color.RGBA, factor float64) color.RGBA {
	return color.RGBA{
		R: clampUint8(float64(c.R) * factor),
		G: clampUint8(float64(c.G) * factor),
		B: clampUint8(float64(c.B) * factor),
		A: c.A,
	}
}

var lightTextColor = gray100
var darkTextColor = gray900
var lightTextLuminance = luminance(lightTextColor)
var darkTextLuminance = luminance(darkTextColor)

// pickTextColor returns either lightTextColor or darkTextColor depending on
// which has better contrast with the given background color (ignoring alpha).
func pickTextColor(background color.RGBA) color.RGBA {
	bg := luminance(background)
	lightContrast := wcagContrast(bg, lightTextLuminance)
	darkContrast := wcagContrast(bg, darkTextLuminance)
	if lightContrast >= darkContrast {
		return lightTextColor
	}
	return darkTextColor
}

// luminance returns the Y component of the color (ignoring alpha) in the XYZ
// color space using the sRGB color space and D65 white point.
func luminance(c color.RGBA) (y float64) {
	// https://www.w3.org/TR/WCAG21/#dfn-relative-luminance
	nr, nb, ng := float64(c.R)/255, float64(c.B)/255, float64(c.G)/255
	lr, lg, lb := linearize(nr), linearize(ng), linearize(nb)
	y = 0.21263900587151027*lr + 0.715168678767756*lg + 0.07219231536073371*lb
	return
}

// wcagContrast returns the WCAG 2.1 contrast ratio n:1 between two relative
// luminance values.
func wcagContrast(y1, y2 float64) float64 {
	if y1 > y2 {
		return (y1 + 0.05) / (y2 + 0.05)
	}
	return (y2 + 0.05) / (y1 + 0.05)
}

// linearize converts a normalized sRGB component [0..1] to a linear sRGB
// component [0..1].
func linearize(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// colors from Tailwind CSS v4.1.5 (MIT)
// generated at https://colors.calebc.co

var (
	slate50  = color.RGBA{248, 250, 252, 255}
	slate100 = color.RGBA{241, 245, 249, 255}
	slate200 = color.RGBA{226, 232, 240, 255}
	slate300 = color.RGBA{202, 213, 226, 255}
	slate400 = color.RGBA{144, 161, 185, 255}
	slate500 = color.RGBA{98, 116, 142, 255}
	slate600 = color.RGBA{69, 85, 108, 255}
	slate700 = color.RGBA{49, 65, 88, 255}
	slate800 = color.RGBA{29, 41, 61, 255}
	slate900 = color.RGBA{15, 23, 43, 255}
	slate950 = color.RGBA{2, 6, 24, 255}
)

var (
	gray50  = color.RGBA{249, 250, 251, 255}
	gray100 = color.RGBA{243, 244, 246, 255}
	gray200 = color.RGBA{229, 231, 235, 255}
	gray300 = color.RGBA{209, 213, 220, 255}
	gray400 = color.RGBA{153, 161, 175, 255}
	gray500 = color.RGBA{106, 114, 130, 255}
	gray600 = color.RGBA{74, 85, 101, 255}
	gray700 = color.RGBA{54, 65, 83, 255}
	gray800 = color.RGBA{30, 41, 57, 255}
	gray900 = color.RGBA{16, 24, 40, 255}
	gray950 = color.RGBA{3, 7, 18, 255}
)

var (
	zinc50  = color.RGBA{250, 250, 250, 255}
	zinc100 = color.RGBA{244, 244, 245, 255}
	zinc200 = color.RGBA{228, 228, 231, 255}
	zinc300 = color.RGBA{212, 212, 216, 255}
	zinc400 = color.RGBA{159, 159, 169, 255}
	zinc500 = color.RGBA{113, 113, 123, 255}
	zinc600 = color.RGBA{82, 82, 92, 255}
	zinc700 = color.RGBA{63, 63, 71, 255}
	zinc800 = color.RGBA{39, 39, 42, 255}
	zinc900 = color.RGBA{24, 24, 27, 255}
	zinc950 = color.RGBA{9, 9, 11, 255}
)

var (
	neutral50  = color.RGBA{250, 250, 250, 255}
	neutral100 = color.RGBA{245, 245, 245, 255}
	neutral200 = color.RGBA{229, 229, 229, 255}
	neutral300 = color.RGBA{212, 212, 212, 255}
	neutral400 = color.RGBA{161, 161, 161, 255}
	neutral500 = color.RGBA{115, 115, 115, 255}
	neutral600 = color.RGBA{82, 82, 82, 255}
	neutral700 = color.RGBA{64, 64, 64, 255}
	neutral800 = color.RGBA{38, 38, 38, 255}
	neutral900 = color.RGBA{23, 23, 23, 255}
	neutral950 = color.RGBA{10, 10, 10, 255}
)

var (
	stone50  = color.RGBA{250, 250, 249, 255}
	stone100 = color.RGBA{245, 245, 244, 255}
	stone200 = color.RGBA{231, 229, 228, 255}
	stone300 = color.RGBA{215, 211, 209, 255}
	stone400 = color.RGBA{166, 160, 155, 255}
	stone500 = color.RGBA{121, 113, 107, 255}
	stone600 = color.RGBA{87, 83, 77, 255}
	stone700 = color.RGBA{68, 64, 59, 255}
	stone800 = color.RGBA{41, 37, 36, 255}
	stone900 = color.RGBA{28, 25, 23, 255}
	stone950 = color.RGBA{12, 10, 9, 255}
)

var (
	red50  = color.RGBA{254, 242, 242, 255}
	red100 = color.RGBA{255, 226, 226, 255}
	red200 = color.RGBA{255, 201, 201, 255}
	red300 = color.RGBA{255, 162, 162, 255}
	red400 = color.RGBA{255, 100, 103, 255}
	red500 = color.RGBA{251, 44, 54, 255}
	red600 = color.RGBA{231, 0, 11, 255}
	red700 = color.RGBA{193, 0, 7, 255}
	red800 = color.RGBA{159, 7, 18, 255}
	red900 = color.RGBA{130, 24, 26, 255}
	red950 = color.RGBA{70, 8, 9, 255}
)

var (
	orange50  = color.RGBA{255, 247, 237, 255}
	orange100 = color.RGBA{255, 237, 212, 255}
	orange200 = color.RGBA{255, 214, 167, 255}
	orange300 = color.RGBA{255, 184, 106, 255}
	orange400 = color.RGBA{255, 137, 4, 255}
	orange500 = color.RGBA{255, 105, 0, 255}
	orange600 = color.RGBA{245, 73, 0, 255}
	orange700 = color.RGBA{202, 53, 0, 255}
	orange800 = color.RGBA{159, 45, 0, 255}
	orange900 = color.RGBA{126, 42, 12, 255}
	orange950 = color.RGBA{68, 19, 6, 255}
)

var (
	amber50  = color.RGBA{255, 251, 235, 255}
	amber100 = color.RGBA{254, 243, 198, 255}
	amber200 = color.RGBA{254, 230, 133, 255}
	amber300 = color.RGBA{255, 210, 48, 255}
	amber400 = color.RGBA{255, 185, 0, 255}
	amber500 = color.RGBA{254, 154, 0, 255}
	amber600 = color.RGBA{225, 113, 0, 255}
	amber700 = color.RGBA{187, 77, 0, 255}
	amber800 = color.RGBA{151, 60, 0, 255}
	amber900 = color.RGBA{123, 51, 6, 255}
	amber950 = color.RGBA{70, 25, 1, 255}
)

var (
	yellow50  = color.RGBA{254, 252, 232, 255}
	yellow100 = color.RGBA{254, 249, 194, 255}
	yellow200 = color.RGBA{255, 240, 133, 255}
	yellow300 = color.RGBA{255, 223, 32, 255}
	yellow400 = color.RGBA{253, 199, 0, 255}
	yellow500 = color.RGBA{240, 177, 0, 255}
	yellow600 = color.RGBA{208, 135, 0, 255}
	yellow700 = color.RGBA{166, 95, 0, 255}
	yellow800 = color.RGBA{137, 75, 0, 255}
	yellow900 = color.RGBA{115, 62, 10, 255}
	yellow950 = color.RGBA{67, 32, 4, 255}
)

var (
	lime50  = color.RGBA{247, 254, 231, 255}
	lime100 = color.RGBA{236, 252, 202, 255}
	lime200 = color.RGBA{216, 249, 153, 255}
	lime300 = color.RGBA{187, 244, 81, 255}
	lime400 = color.RGBA{154, 230, 0, 255}
	lime500 = color.RGBA{124, 207, 0, 255}
	lime600 = color.RGBA{94, 165, 0, 255}
	lime700 = color.RGBA{73, 125, 0, 255}
	lime800 = color.RGBA{60, 99, 0, 255}
	lime900 = color.RGBA{53, 83, 14, 255}
	lime950 = color.RGBA{25, 46, 3, 255}
)

var (
	green50  = color.RGBA{240, 253, 244, 255}
	green100 = color.RGBA{220, 252, 231, 255}
	green200 = color.RGBA{185, 248, 207, 255}
	green300 = color.RGBA{123, 241, 168, 255}
	green400 = color.RGBA{5, 223, 114, 255}
	green500 = color.RGBA{0, 201, 80, 255}
	green600 = color.RGBA{0, 166, 62, 255}
	green700 = color.RGBA{0, 130, 53, 255}
	green800 = color.RGBA{1, 102, 48, 255}
	green900 = color.RGBA{13, 84, 43, 255}
	green950 = color.RGBA{3, 46, 21, 255}
)

var (
	emerald50  = color.RGBA{236, 253, 245, 255}
	emerald100 = color.RGBA{208, 250, 229, 255}
	emerald200 = color.RGBA{164, 244, 207, 255}
	emerald300 = color.RGBA{94, 233, 181, 255}
	emerald400 = color.RGBA{0, 212, 146, 255}
	emerald500 = color.RGBA{0, 188, 125, 255}
	emerald600 = color.RGBA{0, 153, 102, 255}
	emerald700 = color.RGBA{0, 122, 85, 255}
	emerald800 = color.RGBA{0, 96, 69, 255}
	emerald900 = color.RGBA{0, 79, 59, 255}
	emerald950 = color.RGBA{0, 44, 34, 255}
)

var (
	teal50  = color.RGBA{240, 253, 250, 255}
	teal100 = color.RGBA{203, 251, 241, 255}
	teal200 = color.RGBA{150, 247, 228, 255}
	teal300 = color.RGBA{70, 236, 213, 255}
	teal400 = color.RGBA{0, 213, 190, 255}
	teal500 = color.RGBA{0, 187, 167, 255}
	teal600 = color.RGBA{0, 150, 137, 255}
	teal700 = color.RGBA{0, 120, 111, 255}
	teal800 = color.RGBA{0, 95, 90, 255}
	teal900 = color.RGBA{11, 79, 74, 255}
	teal950 = color.RGBA{2, 47, 46, 255}
)

var (
	cyan50  = color.RGBA{236, 254, 255, 255}
	cyan100 = color.RGBA{206, 250, 254, 255}
	cyan200 = color.RGBA{162, 244, 253, 255}
	cyan300 = color.RGBA{83, 234, 253, 255}
	cyan400 = color.RGBA{0, 211, 243, 255}
	cyan500 = color.RGBA{0, 184, 219, 255}
	cyan600 = color.RGBA{0, 146, 184, 255}
	cyan700 = color.RGBA{0, 117, 149, 255}
	cyan800 = color.RGBA{0, 95, 120, 255}
	cyan900 = color.RGBA{16, 78, 100, 255}
	cyan950 = color.RGBA{5, 51, 69, 255}
)

var (
	sky50  = color.RGBA{240, 249, 255, 255}
	sky100 = color.RGBA{223, 242, 254, 255}
	sky200 = color.RGBA{184, 230, 254, 255}
	sky300 = color.RGBA{116, 212, 255, 255}
	sky400 = color.RGBA{0, 188, 255, 255}
	sky500 = color.RGBA{0, 166, 244, 255}
	sky600 = color.RGBA{0, 132, 209, 255}
	sky700 = color.RGBA{0, 105, 168, 255}
	sky800 = color.RGBA{0, 89, 138, 255}
	sky900 = color.RGBA{2, 74, 112, 255}
	sky950 = color.RGBA{5, 47, 74, 255}
)

var (
	blue50  = color.RGBA{239, 246, 255, 255}
	blue100 = color.RGBA{219, 234, 254, 255}
	blue200 = color.RGBA{190, 219, 255, 255}
	blue300 = color.RGBA{142, 197, 255, 255}
	blue400 = color.RGBA{81, 162, 255, 255}
	blue500 = color.RGBA{43, 127, 255, 255}
	blue600 = color.RGBA{21, 93, 252, 255}
	blue700 = color.RGBA{20, 71, 230, 255}
	blue800 = color.RGBA{25, 60, 184, 255}
	blue900 = color.RGBA{28, 57, 142, 255}
	blue950 = color.RGBA{22, 36, 86, 255}
)

var (
	indigo50  = color.RGBA{238, 242, 255, 255}
	indigo100 = color.RGBA{224, 231, 255, 255}
	indigo200 = color.RGBA{198, 210, 255, 255}
	indigo300 = color.RGBA{163, 179, 255, 255}
	indigo400 = color.RGBA{124, 134, 255, 255}
	indigo500 = color.RGBA{97, 95, 255, 255}
	indigo600 = color.RGBA{79, 57, 246, 255}
	indigo700 = color.RGBA{67, 45, 215, 255}
	indigo800 = color.RGBA{55, 42, 172, 255}
	indigo900 = color.RGBA{49, 44, 133, 255}
	indigo950 = color.RGBA{30, 26, 77, 255}
)

var (
	violet50  = color.RGBA{245, 243, 255, 255}
	violet100 = color.RGBA{237, 233, 254, 255}
	violet200 = color.RGBA{221, 214, 255, 255}
	violet300 = color.RGBA{196, 179, 255, 255}
	violet400 = color.RGBA{166, 132, 255, 255}
	violet500 = color.RGBA{142, 81, 255, 255}
	violet600 = color.RGBA{127, 34, 254, 255}
	violet700 = color.RGBA{112, 8, 231, 255}
	violet800 = color.RGBA{93, 14, 192, 255}
	violet900 = color.RGBA{77, 23, 154, 255}
	violet950 = color.RGBA{47, 13, 104, 255}
)

var (
	purple50  = color.RGBA{250, 245, 255, 255}
	purple100 = color.RGBA{243, 232, 255, 255}
	purple200 = color.RGBA{233, 212, 255, 255}
	purple300 = color.RGBA{218, 178, 255, 255}
	purple400 = color.RGBA{194, 122, 255, 255}
	purple500 = color.RGBA{173, 70, 255, 255}
	purple600 = color.RGBA{152, 16, 250, 255}
	purple700 = color.RGBA{130, 0, 219, 255}
	purple800 = color.RGBA{110, 17, 176, 255}
	purple900 = color.RGBA{89, 22, 139, 255}
	purple950 = color.RGBA{60, 3, 102, 255}
)

var (
	fuchsia50  = color.RGBA{253, 244, 255, 255}
	fuchsia100 = color.RGBA{250, 232, 255, 255}
	fuchsia200 = color.RGBA{246, 207, 255, 255}
	fuchsia300 = color.RGBA{244, 168, 255, 255}
	fuchsia400 = color.RGBA{237, 106, 255, 255}
	fuchsia500 = color.RGBA{225, 42, 251, 255}
	fuchsia600 = color.RGBA{200, 0, 222, 255}
	fuchsia700 = color.RGBA{168, 0, 183, 255}
	fuchsia800 = color.RGBA{138, 1, 148, 255}
	fuchsia900 = color.RGBA{114, 19, 120, 255}
	fuchsia950 = color.RGBA{75, 0, 79, 255}
)

var (
	pink50  = color.RGBA{253, 242, 248, 255}
	pink100 = color.RGBA{252, 231, 243, 255}
	pink200 = color.RGBA{252, 206, 232, 255}
	pink300 = color.RGBA{253, 165, 213, 255}
	pink400 = color.RGBA{251, 100, 182, 255}
	pink500 = color.RGBA{246, 51, 154, 255}
	pink600 = color.RGBA{230, 0, 118, 255}
	pink700 = color.RGBA{198, 0, 92, 255}
	pink800 = color.RGBA{163, 0, 76, 255}
	pink900 = color.RGBA{134, 16, 67, 255}
	pink950 = color.RGBA{81, 4, 36, 255}
)

var (
	rose50  = color.RGBA{255, 241, 242, 255}
	rose100 = color.RGBA{255, 228, 230, 255}
	rose200 = color.RGBA{255, 204, 211, 255}
	rose300 = color.RGBA{255, 161, 173, 255}
	rose400 = color.RGBA{255, 99, 126, 255}
	rose500 = color.RGBA{255, 32, 86, 255}
	rose600 = color.RGBA{236, 0, 63, 255}
	rose700 = color.RGBA{199, 0, 54, 255}
	rose800 = color.RGBA{165, 0, 54, 255}
	rose900 = color.RGBA{139, 8, 54, 255}
	rose950 = color.RGBA{77, 2, 24, 255}
)

var colorNames = map[string]color.RGBA{
	"slate50":  slate50,
	"slate100": slate100,
	"slate200": slate200,
	"slate300": slate300,
	"slate400": slate400,
	"slate500": slate500,
	"slate600": slate600,
	"slate700": slate700,
	"slate800": slate800,
	"slate900": slate900,
	"slate950": slate950,

	"gray50":  gray50,
	"gray100": gray100,
	"gray200": gray200,
	"gray300": gray300,
	"gray400": gray400,
	"gray500": gray500,
	"gray600": gray600,
	"gray700": gray700,
	"gray800": gray800,
	"gray900": gray900,
	"gray950": gray950,

	"zinc50":  zinc50,
	"zinc100": zinc100,
	"zinc200": zinc200,
	"zinc300": zinc300,
	"zinc400": zinc400,
	"zinc500": zinc500,
	"zinc600": zinc600,
	"zinc700": zinc700,
	"zinc800": zinc800,
	"zinc900": zinc900,
	"zinc950": zinc950,

	"neutral50":  neutral50,
	"neutral100": neutral100,
	"neutral200": neutral200,
	"neutral300": neutral300,
	"neutral400": neutral400,
	"neutral500": neutral500,
	"neutral600": neutral600,
	"neutral700": neutral700,
	"neutral800": neutral800,
	"neutral900": neutral900,
	"neutral950": neutral950,

	"stone50":  stone50,
	"stone100": stone100,
	"stone200": stone200,
	"stone300": stone300,
	"stone400": stone400,
	"stone500": stone500,
	"stone600": stone600,
	"stone700": stone700,
	"stone800": stone800,
	"stone900": stone900,
	"stone950": stone950,

	"red50":  red50,
	"red100": red100,
	"red200": red200,
	"red300": red300,
	"red400": red400,
	"red500": red500,
	"red600": red600,
	"red700": red700,
	"red800": red800,
	"red900": red900,
	"red950": red950,

	"orange50":  orange50,
	"orange100": orange100,
	"orange200": orange200,
	"orange300": orange300,
	"orange400": orange400,
	"orange500": orange500,
	"orange600": orange600,
	"orange700": orange700,
	"orange800": orange800,
	"orange900": orange900,
	"orange950": orange950,

	"amber50":  amber50,
	"amber100": amber100,
	"amber200": amber200,
	"amber300": amber300,
	"amber400": amber400,
	"amber500": amber500,
	"amber600": amber600,
	"amber700": amber700,
	"amber800": amber800,
	"amber900": amber900,
	"amber950": amber950,

	"yellow50":  yellow50,
	"yellow100": yellow100,
	"yellow200": yellow200,
	"yellow300": yellow300,
	"yellow400": yellow400,
	"yellow500": yellow500,
	"yellow600": yellow600,
	"yellow700": yellow700,
	"yellow800": yellow800,
	"yellow900": yellow900,
	"yellow950": yellow950,

	"lime50":  lime50,
	"lime100": lime100,
	"lime200": lime200,
	"lime300": lime300,
	"lime400": lime400,
	"lime500": lime500,
	"lime600": lime600,
	"lime700": lime700,
	"lime800": lime800,
	"lime900": lime900,
	"lime950": lime950,

	"green50":  green50,
	"green100": green100,
	"green200": green200,
	"green300": green300,
	"green400": green400,
	"green500": green500,
	"green600": green600,
	"green700": green700,
	"green800": green800,
	"green900": green900,
	"green950": green950,

	"emerald50":  emerald50,
	"emerald100": emerald100,
	"emerald200": emerald200,
	"emerald300": emerald300,
	"emerald400": emerald400,
	"emerald500": emerald500,
	"emerald600": emerald600,
	"emerald700": emerald700,
	"emerald800": emerald800,
	"emerald900": emerald900,
	"emerald950": emerald950,

	"teal50":  teal50,
	"teal100": teal100,
	"teal200": teal200,
	"teal300": teal300,
	"teal400": teal400,
	"teal500": teal500,
	"teal600": teal600,
	"teal700": teal700,
	"teal800": teal800,
	"teal900": teal900,
	"teal950": teal950,

	"cyan50":  cyan50,
	"cyan100": cyan100,
	"cyan200": cyan200,
	"cyan300": cyan300,
	"cyan400": cyan400,
	"cyan500": cyan500,
	"cyan600": cyan600,
	"cyan700": cyan700,
	"cyan800": cyan800,
	"cyan900": cyan900,
	"cyan950": cyan950,

	"sky50":  sky50,
	"sky100": sky100,
	"sky200": sky200,
	"sky300": sky300,
	"sky400": sky400,
	"sky500": sky500,
	"sky600": sky600,
	"sky700": sky700,
	"sky800": sky800,
	"sky900": sky900,
	"sky950": sky950,

	"blue50":  blue50,
	"blue100": blue100,
	"blue200": blue200,
	"blue300": blue300,
	"blue400": blue400,
	"blue500": blue500,
	"blue600": blue600,
	"blue700": blue700,
	"blue800": blue800,
	"blue900": blue900,
	"blue950": blue950,

	"indigo50":  indigo50,
	"indigo100": indigo100,
	"indigo200": indigo200,
	"indigo300": indigo300,
	"indigo400": indigo400,
	"indigo500": indigo500,
	"indigo600": indigo600,
	"indigo700": indigo700,
	"indigo800": indigo800,
	"indigo900": indigo900,
	"indigo950": indigo950,

	"violet50":  violet50,
	"violet100": violet100,
	"violet200": violet200,
	"violet300": violet300,
	"violet400": violet400,
	"violet500": violet500,
	"violet600": violet600,
	"violet700": violet700,
	"violet800": violet800,
	"violet900": violet900,
	"violet950": violet950,

	"purple50":  purple50,
	"purple100": purple100,
	"purple200": purple200,
	"purple300": purple300,
	"purple400": purple400,
	"purple500": purple500,
	"purple600": purple600,
	"purple700": purple700,
	"purple800": purple800,
	"purple900": purple900,
	"purple950": purple950,

	"fuchsia50":  fuchsia50,
	"fuchsia100": fuchsia100,
	"fuchsia200": fuchsia200,
	"fuchsia300": fuchsia300,
	"fuchsia400": fuchsia400,
	"fuchsia500": fuchsia500,
	"fuchsia600": fuchsia600,
	"fuchsia700": fuchsia700,
	"fuchsia800": fuchsia800,
	"fuchsia900": fuchsia900,
	"fuchsia950": fuchsia950,

	"pink50":  pink50,
	"pink100": pink100,
	"pink200": pink200,
	"pink300": pink300,
	"pink400": pink400,
	"pink500": pink500,
	"pink600": pink600,
	"pink700": pink700,
	"pink800": pink800,
	"pink900": pink900,
	"pink950": pink950,

	"rose50":  rose50,
	"rose100": rose100,
	"rose200": rose200,
	"rose300": rose300,
	"rose400": rose400,
	"rose500": rose500,
	"rose600": rose600,
	"rose700": rose700,
	"rose800": rose800,
	"rose900": rose900,
	"rose950": rose950,
}
