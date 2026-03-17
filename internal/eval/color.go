// Package eval provides color math and terminal palette utilities for
// evaluating widget contrast and theme coherence. This is non-production
// test tooling — it is not in the hot path.
package eval

import "math"

// RGB holds a 24-bit color as three 8-bit channels.
type RGB struct {
	R, G, B uint8
}

// xterm256Table maps xterm-256 indices to canonical RGB values.
//
// Indices 0-15 are the "standard" xterm defaults (named colors). These are
// palette-dependent in real terminals; this table uses xterm's own defaults.
// Indices 16-231 are the 6x6x6 color cube. Indices 232-255 are the 24-step
// grayscale ramp (not including pure black/white, which are in 0-15).
var xterm256Table = buildXterm256Table()

func buildXterm256Table() [256]RGB {
	var t [256]RGB

	// 0-15: xterm named colors (terminal-default values from xterm source).
	t[0] = RGB{0x00, 0x00, 0x00} // Black
	t[1] = RGB{0x80, 0x00, 0x00} // Maroon
	t[2] = RGB{0x00, 0x80, 0x00} // Green
	t[3] = RGB{0x80, 0x80, 0x00} // Olive
	t[4] = RGB{0x00, 0x00, 0x80} // Navy
	t[5] = RGB{0x80, 0x00, 0x80} // Purple
	t[6] = RGB{0x00, 0x80, 0x80} // Teal
	t[7] = RGB{0xc0, 0xc0, 0xc0} // Silver
	t[8] = RGB{0x80, 0x80, 0x80} // Grey
	t[9] = RGB{0xff, 0x00, 0x00} // Red
	t[10] = RGB{0x00, 0xff, 0x00} // Lime
	t[11] = RGB{0xff, 0xff, 0x00} // Yellow
	t[12] = RGB{0x00, 0x00, 0xff} // Blue
	t[13] = RGB{0xff, 0x00, 0xff} // Fuchsia
	t[14] = RGB{0x00, 0xff, 0xff} // Aqua
	t[15] = RGB{0xff, 0xff, 0xff} // White

	// 16-231: 6x6x6 color cube.
	// Each channel value maps 0→0, 1→95, 2→135, 3→175, 4→215, 5→255.
	cubeSteps := [6]uint8{0, 95, 135, 175, 215, 255}
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				t[16+r*36+g*6+b] = RGB{cubeSteps[r], cubeSteps[g], cubeSteps[b]}
			}
		}
	}

	// 232-255: 24-step grayscale ramp.
	// Starts at #080808, increments by 10 each step, ends at #eeeeee.
	for i := 0; i < 24; i++ {
		v := uint8(8 + i*10)
		t[232+i] = RGB{v, v, v}
	}

	return t
}

// Xterm256ToRGB returns the canonical RGB value for an xterm-256 color index.
// Indices outside 0-255 are clamped to the valid range.
func Xterm256ToRGB(index int) RGB {
	if index < 0 {
		index = 0
	}
	if index > 255 {
		index = 255
	}
	return xterm256Table[index]
}

// linearizeChannel converts an sRGB channel value (0-255) to a linear light
// value using the IEC 61966-2-1 transfer function, as specified by WCAG 2.1.
func linearizeChannel(v uint8) float64 {
	f := float64(v) / 255.0
	if f <= 0.04045 {
		return f / 12.92
	}
	return math.Pow((f+0.055)/1.055, 2.4)
}

// RelativeLuminance returns the WCAG 2.1 relative luminance of an RGB color.
// The result is in the range [0, 1], where 0 is black and 1 is white.
func RelativeLuminance(c RGB) float64 {
	r := linearizeChannel(c.R)
	g := linearizeChannel(c.G)
	b := linearizeChannel(c.B)
	return 0.2126*r + 0.7152*g + 0.0722*b
}

// ContrastRatio returns the WCAG 2.1 contrast ratio between two colors.
// The result is in the range [1, 21]. A ratio of 21 means maximum contrast
// (black on white); 1 means no contrast (identical colors).
func ContrastRatio(fg, bg RGB) float64 {
	l1 := RelativeLuminance(fg)
	l2 := RelativeLuminance(bg)
	// Put the lighter color in l1.
	if l2 > l1 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// RGBToHSL converts an RGB color to HSL (hue, saturation, lightness).
// Hue is in degrees [0, 360), saturation and lightness are in [0, 1].
func RGBToHSL(c RGB) (h, s, l float64) {
	r := float64(c.R) / 255.0
	g := float64(c.G) / 255.0
	b := float64(c.B) / 255.0

	maxC := math.Max(r, math.Max(g, b))
	minC := math.Min(r, math.Min(g, b))
	delta := maxC - minC

	l = (maxC + minC) / 2.0

	if delta == 0 {
		// Achromatic: hue and saturation are undefined/zero.
		return 0, 0, l
	}

	if l < 0.5 {
		s = delta / (maxC + minC)
	} else {
		s = delta / (2.0 - maxC - minC)
	}

	switch maxC {
	case r:
		h = (g - b) / delta
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/delta + 2
	default: // b
		h = (r-g)/delta + 4
	}
	h *= 60

	return h, s, l
}

// HueDelta returns the shortest arc distance between two hue values in degrees.
// The result is always in [0, 180].
func HueDelta(h1, h2 float64) float64 {
	diff := math.Abs(h1-h2) - math.Floor(math.Abs(h1-h2)/360)*360
	if diff > 180 {
		diff = 360 - diff
	}
	return diff
}
