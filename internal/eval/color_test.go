package eval

import (
	"math"
	"testing"
)

func TestRelativeLuminance(t *testing.T) {
	tests := []struct {
		name string
		c    RGB
		want float64
	}{
		{"black", RGB{0, 0, 0}, 0.0},
		{"white", RGB{255, 255, 255}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RelativeLuminance(tt.c)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("RelativeLuminance(%v) = %f, want %f", tt.c, got, tt.want)
			}
		})
	}
}

func TestContrastRatio(t *testing.T) {
	black := RGB{0, 0, 0}
	white := RGB{255, 255, 255}
	// Mid-range pair: #767676 on white is the WCAG AA boundary for normal text
	// at exactly 4.54:1 per the WCAG reference calculator.
	gray := RGB{0x76, 0x76, 0x76}

	tests := []struct {
		name      string
		fg, bg    RGB
		wantRatio float64
		tolerance float64
	}{
		{"black on white", black, white, 21.0, 0.01},
		{"white on black", white, black, 21.0, 0.01},
		{"identical (black)", black, black, 1.0, 1e-9},
		{"identical (white)", white, white, 1.0, 1e-9},
		{"gray #767676 on white", gray, white, 4.54, 0.02},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContrastRatio(tt.fg, tt.bg)
			if math.Abs(got-tt.wantRatio) > tt.tolerance {
				t.Errorf("ContrastRatio(%v, %v) = %.4f, want %.4f (±%.4f)",
					tt.fg, tt.bg, got, tt.wantRatio, tt.tolerance)
			}
		})
	}
}

func TestXterm256ToRGB(t *testing.T) {
	tests := []struct {
		index int
		want  RGB
	}{
		// Color cube: index 196 = r=5,g=0,b=0 → #ff0000
		{196, RGB{0xff, 0x00, 0x00}},
		// Color cube: index 46 = r=0,g=5,b=0 → #00ff00
		{46, RGB{0x00, 0xff, 0x00}},
		// Grayscale ramp: index 232 = 8+0*10=8 → #080808
		{232, RGB{0x08, 0x08, 0x08}},
		// Grayscale ramp: index 255 = 8+23*10=238 → #eeeeee
		{255, RGB{0xee, 0xee, 0xee}},
		// Named color: index 0 = black
		{0, RGB{0x00, 0x00, 0x00}},
		// Named color: index 15 = white
		{15, RGB{0xff, 0xff, 0xff}},
	}
	for _, tt := range tests {
		got := Xterm256ToRGB(tt.index)
		if got != tt.want {
			t.Errorf("Xterm256ToRGB(%d) = %v, want %v", tt.index, got, tt.want)
		}
	}
}

func TestRGBToHSL(t *testing.T) {
	tests := []struct {
		name     string
		c        RGB
		wantH    float64
		wantHue  bool // only check hue when true (achromatic colors have undefined hue)
		wantS    float64
		wantL    float64
		tolH     float64
		tolSL    float64
	}{
		{
			name: "pure red",
			c:    RGB{255, 0, 0},
			wantH: 0, wantHue: true,
			wantS: 1.0, wantL: 0.5,
			tolH: 0.01, tolSL: 1e-9,
		},
		{
			name: "pure green",
			c:    RGB{0, 255, 0},
			wantH: 120, wantHue: true,
			wantS: 1.0, wantL: 0.5,
			tolH: 0.01, tolSL: 1e-9,
		},
		{
			name: "pure blue",
			c:    RGB{0, 0, 255},
			wantH: 240, wantHue: true,
			wantS: 1.0, wantL: 0.5,
			tolH: 0.01, tolSL: 1e-9,
		},
		{
			name: "black",
			c:    RGB{0, 0, 0},
			wantHue: false,
			wantS: 0.0, wantL: 0.0,
			tolSL: 1e-9,
		},
		{
			name: "white",
			c:    RGB{255, 255, 255},
			wantHue: false,
			wantS: 0.0, wantL: 1.0,
			tolSL: 1e-9,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s, l := RGBToHSL(tt.c)
			if tt.wantHue && math.Abs(h-tt.wantH) > tt.tolH {
				t.Errorf("RGBToHSL(%v) hue = %.2f, want %.2f (±%.2f)", tt.c, h, tt.wantH, tt.tolH)
			}
			if math.Abs(s-tt.wantS) > tt.tolSL {
				t.Errorf("RGBToHSL(%v) saturation = %.6f, want %.6f", tt.c, s, tt.wantS)
			}
			if math.Abs(l-tt.wantL) > tt.tolSL {
				t.Errorf("RGBToHSL(%v) lightness = %.6f, want %.6f", tt.c, l, tt.wantL)
			}
		})
	}
}

func TestHueDelta(t *testing.T) {
	tests := []struct {
		h1, h2 float64
		want   float64
	}{
		{0, 180, 180},
		{0, 90, 90},
		{350, 10, 20},  // wraps around the 0/360 boundary
		{10, 350, 20},
		{0, 0, 0},
		{180, 180, 0},
	}
	for _, tt := range tests {
		got := HueDelta(tt.h1, tt.h2)
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("HueDelta(%.1f, %.1f) = %.6f, want %.6f", tt.h1, tt.h2, got, tt.want)
		}
	}
}
