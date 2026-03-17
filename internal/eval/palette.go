package eval

// TerminalPalette describes a terminal color scheme. Colors[0-7] are the
// standard ANSI colors, Colors[8-15] are the bright variants.
type TerminalPalette struct {
	Name      string
	DefaultFg RGB
	DefaultBg RGB
	Colors    [16]RGB
}

// AllPalettes returns all built-in terminal palettes.
func AllPalettes() []TerminalPalette {
	return []TerminalPalette{
		XtermDefault,
		SolarizedDark,
		SolarizedLight,
		Dracula,
		Nord,
		GruvboxDark,
		TokyoNight,
		CatppuccinMocha,
	}
}

// XtermDefault uses the canonical xterm default colors.
// Fg: Silver (#c0c0c0), Bg: Black (#000000).
var XtermDefault = TerminalPalette{
	Name:      "XtermDefault",
	DefaultFg: RGB{0xc0, 0xc0, 0xc0},
	DefaultBg: RGB{0x00, 0x00, 0x00},
	Colors: [16]RGB{
		{0x00, 0x00, 0x00}, // 0  Black
		{0x80, 0x00, 0x00}, // 1  Maroon
		{0x00, 0x80, 0x00}, // 2  Green
		{0x80, 0x80, 0x00}, // 3  Olive
		{0x00, 0x00, 0x80}, // 4  Navy
		{0x80, 0x00, 0x80}, // 5  Purple
		{0x00, 0x80, 0x80}, // 6  Teal
		{0xc0, 0xc0, 0xc0}, // 7  Silver
		{0x80, 0x80, 0x80}, // 8  Grey
		{0xff, 0x00, 0x00}, // 9  Red
		{0x00, 0xff, 0x00}, // 10 Lime
		{0xff, 0xff, 0x00}, // 11 Yellow
		{0x00, 0x00, 0xff}, // 12 Blue
		{0xff, 0x00, 0xff}, // 13 Fuchsia
		{0x00, 0xff, 0xff}, // 14 Aqua
		{0xff, 0xff, 0xff}, // 15 White
	},
}

// SolarizedDark uses Ethan Schoonover's Solarized dark scheme.
// Source: https://ethanschoonover.com/solarized/
var SolarizedDark = TerminalPalette{
	Name:      "SolarizedDark",
	DefaultFg: RGB{0x83, 0x94, 0x96}, // base0 #839496
	DefaultBg: RGB{0x00, 0x2b, 0x36}, // base03 #002b36
	Colors: [16]RGB{
		{0x07, 0x36, 0x42}, // 0  base02  #073642
		{0xdc, 0x32, 0x2f}, // 1  red     #dc322f
		{0x85, 0x99, 0x00}, // 2  green   #859900
		{0xb5, 0x89, 0x00}, // 3  yellow  #b58900
		{0x26, 0x8b, 0xd2}, // 4  blue    #268bd2
		{0xd3, 0x36, 0x82}, // 5  magenta #d33682
		{0x2a, 0xa1, 0x98}, // 6  cyan    #2aa198
		{0xee, 0xe8, 0xd5}, // 7  base2   #eee8d5
		{0x00, 0x2b, 0x36}, // 8  base03  #002b36
		{0xcb, 0x4b, 0x16}, // 9  orange  #cb4b16
		{0x58, 0x6e, 0x75}, // 10 base01  #586e75
		{0x65, 0x7b, 0x83}, // 11 base00  #657b83
		{0x83, 0x94, 0x96}, // 12 base0   #839496
		{0x6c, 0x71, 0xc4}, // 13 violet  #6c71c4
		{0x93, 0xa1, 0xa1}, // 14 base1   #93a1a1
		{0xfd, 0xf6, 0xe3}, // 15 base3   #fdf6e3
	},
}

// SolarizedLight uses Ethan Schoonover's Solarized light scheme.
// Source: https://ethanschoonover.com/solarized/
var SolarizedLight = TerminalPalette{
	Name:      "SolarizedLight",
	DefaultFg: RGB{0x65, 0x7b, 0x83}, // base00 #657b83
	DefaultBg: RGB{0xfd, 0xf6, 0xe3}, // base3  #fdf6e3
	Colors: [16]RGB{
		{0x07, 0x36, 0x42}, // 0  base02  #073642
		{0xdc, 0x32, 0x2f}, // 1  red     #dc322f
		{0x85, 0x99, 0x00}, // 2  green   #859900
		{0xb5, 0x89, 0x00}, // 3  yellow  #b58900
		{0x26, 0x8b, 0xd2}, // 4  blue    #268bd2
		{0xd3, 0x36, 0x82}, // 5  magenta #d33682
		{0x2a, 0xa1, 0x98}, // 6  cyan    #2aa198
		{0xee, 0xe8, 0xd5}, // 7  base2   #eee8d5
		{0x00, 0x2b, 0x36}, // 8  base03  #002b36
		{0xcb, 0x4b, 0x16}, // 9  orange  #cb4b16
		{0x58, 0x6e, 0x75}, // 10 base01  #586e75
		{0x65, 0x7b, 0x83}, // 11 base00  #657b83
		{0x83, 0x94, 0x96}, // 12 base0   #839496
		{0x6c, 0x71, 0xc4}, // 13 violet  #6c71c4
		{0x93, 0xa1, 0xa1}, // 14 base1   #93a1a1
		{0xfd, 0xf6, 0xe3}, // 15 base3   #fdf6e3
	},
}

// Dracula uses the Dracula color scheme.
// Source: https://github.com/dracula/dracula-theme
var Dracula = TerminalPalette{
	Name:      "Dracula",
	DefaultFg: RGB{0xf8, 0xf8, 0xf2}, // foreground #f8f8f2
	DefaultBg: RGB{0x28, 0x2a, 0x36}, // background #282a36
	Colors: [16]RGB{
		{0x21, 0x22, 0x2c}, // 0  black          #21222c
		{0xff, 0x55, 0x55}, // 1  red             #ff5555
		{0x50, 0xfa, 0x7b}, // 2  green           #50fa7b
		{0xf1, 0xfa, 0x8c}, // 3  yellow          #f1fa8c
		{0xbd, 0x93, 0xf9}, // 4  blue            #bd93f9
		{0xff, 0x79, 0xc6}, // 5  magenta         #ff79c6
		{0x8b, 0xe9, 0xfd}, // 6  cyan            #8be9fd
		{0xf8, 0xf8, 0xf2}, // 7  white           #f8f8f2
		{0x6a, 0x71, 0x86}, // 8  bright black    #6a7186
		{0xff, 0x6e, 0x6e}, // 9  bright red      #ff6e6e
		{0x69, 0xff, 0x94}, // 10 bright green    #69ff94
		{0xff, 0xff, 0xa5}, // 11 bright yellow   #ffffa5
		{0xd6, 0xac, 0xff}, // 12 bright blue     #d6acff
		{0xff, 0x92, 0xdf}, // 13 bright magenta  #ff92df
		{0xa4, 0xff, 0xff}, // 14 bright cyan     #a4ffff
		{0xff, 0xff, 0xff}, // 15 bright white    #ffffff
	},
}

// Nord uses the Nord color scheme by Arctic Ice Studio.
// Source: https://www.nordtheme.com/
var Nord = TerminalPalette{
	Name:      "Nord",
	DefaultFg: RGB{0xd8, 0xde, 0xe9}, // nord4 #d8dee9
	DefaultBg: RGB{0x2e, 0x34, 0x40}, // nord0 #2e3440
	Colors: [16]RGB{
		{0x3b, 0x42, 0x52}, // 0  nord1  (black)          #3b4252
		{0xbf, 0x61, 0x6a}, // 1  nord11 (red)            #bf616a
		{0xa3, 0xbe, 0x8c}, // 2  nord14 (green)          #a3be8c
		{0xeb, 0xcb, 0x8b}, // 3  nord13 (yellow)         #ebcb8b
		{0x81, 0xa1, 0xc1}, // 4  nord9  (blue)           #81a1c1
		{0xb4, 0x8e, 0xad}, // 5  nord15 (magenta)        #b48ead
		{0x88, 0xc0, 0xd0}, // 6  nord8  (cyan)           #88c0d0
		{0xe5, 0xe9, 0xf0}, // 7  nord5  (white)          #e5e9f0
		{0x4c, 0x56, 0x6a}, // 8  nord3  (bright black)   #4c566a
		{0xbf, 0x61, 0x6a}, // 9  nord11 (bright red)     #bf616a
		{0xa3, 0xbe, 0x8c}, // 10 nord14 (bright green)   #a3be8c
		{0xeb, 0xcb, 0x8b}, // 11 nord13 (bright yellow)  #ebcb8b
		{0x81, 0xa1, 0xc1}, // 12 nord9  (bright blue)    #81a1c1
		{0xb4, 0x8e, 0xad}, // 13 nord15 (bright magenta) #b48ead
		{0x8f, 0xbc, 0xbb}, // 14 nord7  (bright cyan)    #8fbcbb
		{0xec, 0xef, 0xf4}, // 15 nord6  (bright white)   #eceff4
	},
}

// GruvboxDark uses the Gruvbox dark color scheme by morhetz.
// Source: https://github.com/morhetz/gruvbox
var GruvboxDark = TerminalPalette{
	Name:      "GruvboxDark",
	DefaultFg: RGB{0xeb, 0xdb, 0xb2}, // fg1 #ebdbb2
	DefaultBg: RGB{0x28, 0x28, 0x28}, // bg  #282828
	Colors: [16]RGB{
		{0x28, 0x28, 0x28}, // 0  bg      #282828
		{0xcc, 0x24, 0x1d}, // 1  red1    #cc241d
		{0x98, 0x97, 0x1a}, // 2  green1  #98971a
		{0xd7, 0x99, 0x21}, // 3  yellow1 #d79921
		{0x45, 0x85, 0x88}, // 4  blue1   #458588
		{0xb1, 0x62, 0x86}, // 5  purple1 #b16286
		{0x68, 0x9d, 0x6a}, // 6  aqua1   #689d6a
		{0xa8, 0x99, 0x84}, // 7  fg4     #a89984
		{0x92, 0x83, 0x74}, // 8  bg4     #928374
		{0xfb, 0x49, 0x34}, // 9  red2    #fb4934
		{0xb8, 0xbb, 0x26}, // 10 green2  #b8bb26
		{0xfa, 0xbd, 0x2f}, // 11 yellow2 #fabd2f
		{0x83, 0xa5, 0x98}, // 12 blue2   #83a598
		{0xd3, 0x86, 0x9b}, // 13 purple2 #d3869b
		{0x8e, 0xc0, 0x7c}, // 14 aqua2   #8ec07c
		{0xeb, 0xdb, 0xb2}, // 15 fg1     #ebdbb2
	},
}

// TokyoNight uses the Tokyo Night color scheme by enkia.
// Source: https://github.com/enkia/tokyo-night-vscode-theme
var TokyoNight = TerminalPalette{
	Name:      "TokyoNight",
	DefaultFg: RGB{0xc0, 0xca, 0xf5}, // fg #c0caf5
	DefaultBg: RGB{0x1a, 0x1b, 0x26}, // bg #1a1b26
	Colors: [16]RGB{
		{0x15, 0x16, 0x22}, // 0  black          #151622
		{0xf7, 0x76, 0x8e}, // 1  red            #f7768e
		{0x9e, 0xce, 0x6a}, // 2  green          #9ece6a
		{0xe0, 0xaf, 0x68}, // 3  yellow         #e0af68
		{0x7a, 0xa2, 0xf7}, // 4  blue           #7aa2f7
		{0xbb, 0x9a, 0xf7}, // 5  magenta        #bb9af7
		{0x7d, 0xcf, 0xff}, // 6  cyan           #7dcfff
		{0xa9, 0xb1, 0xd6}, // 7  white          #a9b1d6
		{0x41, 0x44, 0x68}, // 8  bright black   #414868
		{0xf7, 0x76, 0x8e}, // 9  bright red     #f7768e
		{0x9e, 0xce, 0x6a}, // 10 bright green   #9ece6a
		{0xe0, 0xaf, 0x68}, // 11 bright yellow  #e0af68
		{0x7a, 0xa2, 0xf7}, // 12 bright blue    #7aa2f7
		{0xbb, 0x9a, 0xf7}, // 13 bright magenta #bb9af7
		{0x7d, 0xcf, 0xff}, // 14 bright cyan    #7dcfff
		{0xc0, 0xca, 0xf5}, // 15 bright white   #c0caf5
	},
}

// CatppuccinMocha uses the Catppuccin Mocha color scheme.
// Source: https://github.com/catppuccin/catppuccin
var CatppuccinMocha = TerminalPalette{
	Name:      "CatppuccinMocha",
	DefaultFg: RGB{0xcd, 0xd6, 0xf4}, // text  #cdd6f4
	DefaultBg: RGB{0x1e, 0x1e, 0x2e}, // base  #1e1e2e
	Colors: [16]RGB{
		{0x45, 0x47, 0x5a}, // 0  surface1 (black)          #45475a
		{0xf3, 0x8b, 0xa8}, // 1  red                       #f38ba8
		{0xa6, 0xe3, 0xa1}, // 2  green                     #a6e3a1
		{0xf9, 0xe2, 0xaf}, // 3  yellow                    #f9e2af
		{0x89, 0xb4, 0xfa}, // 4  blue                      #89b4fa
		{0xf5, 0xc2, 0xe7}, // 5  pink (magenta)            #f5c2e7
		{0x94, 0xe2, 0xd5}, // 6  teal (cyan)               #94e2d5
		{0xba, 0xc2, 0xde}, // 7  subtext1 (white)          #bac2de
		{0x58, 0x5b, 0x70}, // 8  surface2 (bright black)   #585b70
		{0xf3, 0x8b, 0xa8}, // 9  red (bright red)          #f38ba8
		{0xa6, 0xe3, 0xa1}, // 10 green (bright green)      #a6e3a1
		{0xf9, 0xe2, 0xaf}, // 11 yellow (bright yellow)    #f9e2af
		{0x89, 0xb4, 0xfa}, // 12 blue (bright blue)        #89b4fa
		{0xf5, 0xc2, 0xe7}, // 13 pink (bright magenta)     #f5c2e7
		{0x94, 0xe2, 0xd5}, // 14 teal (bright cyan)        #94e2d5
		{0xcd, 0xd6, 0xf4}, // 15 text (bright white)       #cdd6f4
	},
}
