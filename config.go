package main

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

//go:embed slp.toml
var sampleConfig []byte

type ThemeConfig struct {
	Name       string `toml:"name"`
	Accent     string `toml:"accent"`
	SelectedFg string `toml:"selected_fg"`
	FilterFg   string `toml:"filter_fg"`
}

type IconsConfig struct {
	Play    string `toml:"play"`
	Pause   string `toml:"pause"`
	Volume  string `toml:"volume"`
	Shuffle string `toml:"shuffle"`
}

type SpotifyConfig struct {
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
	RedirectURI  string `toml:"redirect_uri"`
	SearchLimit  int    `toml:"search_limit"`
}

type UIConfig struct {
	TickInterval int `toml:"tick_interval"`
}

type Config struct {
	Theme   ThemeConfig   `toml:"theme"`
	Icons   IconsConfig   `toml:"icons"`
	Spotify SpotifyConfig `toml:"spotify"`
	UI      UIConfig      `toml:"ui"`
}

type resolvedTheme struct {
	Accent     string
	SelectedFg string
	FilterFg   string
}

var builtinThemes = map[string]resolvedTheme{
	"dracula": {
		Accent:     "#bd93f9",
		SelectedFg: "#f8f8f2",
		FilterFg:   "#ffb86c",
	},
	"iceberg": {
		Accent:     "#84a0c6",
		SelectedFg: "#c6c8d1",
		FilterFg:   "#89b8c2",
	},
	"monokai": {
		Accent:     "#a6e22e",
		SelectedFg: "#f8f8f2",
		FilterFg:   "#e6db74",
	},
	"solarized-dark": {
		Accent:     "#268bd2",
		SelectedFg: "#fdf6e3",
		FilterFg:   "#b58900",
	},
	"solarized-light": {
		Accent:     "#268bd2",
		SelectedFg: "#002b36",
		FilterFg:   "#b58900",
	},
	"nord": {
		Accent:     "#5e81ac",
		SelectedFg: "#eceff4",
		FilterFg:   "#ebcb8b",
	},
	"gruvbox": {
		Accent:     "#689d6a",
		SelectedFg: "#ebdbb2",
		FilterFg:   "#d79921",
	},
	"tokyo-night": {
		Accent:     "#7aa2f7",
		SelectedFg: "#c0caf5",
		FilterFg:   "#e0af68",
	},
	"catppuccin": {
		Accent:     "#cba6f7",
		SelectedFg: "#cdd6f4",
		FilterFg:   "#fab387",
	},
	"rose-pine": {
		Accent:     "#c4a7e7",
		SelectedFg: "#e0def4",
		FilterFg:   "#f6c177",
	},
}

func defaultConfig() Config {
	return Config{
		Icons: IconsConfig{
			Play:    "󰐊",
			Pause:   "󰏤",
			Volume:  "󰕾",
			Shuffle: "󰒝",
		},
		Spotify: SpotifyConfig{
			RedirectURI: "http://127.0.0.1:8888/callback",
			SearchLimit: 20,
		},
		UI: UIConfig{
			TickInterval: 2,
		},
	}
}

func (t ThemeConfig) resolve() resolvedTheme {
	base := resolvedTheme{
		Accent:     "62",
		SelectedFg: "230",
		FilterFg:   "220",
	}
	if t.Name != "" {
		if b, ok := builtinThemes[t.Name]; ok {
			base = b
		}
	}
	if t.Accent != "" {
		base.Accent = t.Accent
	}
	if t.SelectedFg != "" {
		base.SelectedFg = t.SelectedFg
	}
	if t.FilterFg != "" {
		base.FilterFg = t.FilterFg
	}
	return base
}

func configPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "slp", "config.toml")
}

func LoadConfig() Config {
	cfg := defaultConfig()
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, sampleConfig, 0o644)
		return cfg
	}
	// Decode into a partial struct so missing fields keep defaults
	var file Config
	if _, err := toml.Decode(string(data), &file); err != nil {
		return cfg
	}
	// Merge: only override fields that were explicitly set
	if file.Theme.Name != "" {
		cfg.Theme.Name = file.Theme.Name
	}
	if file.Theme.Accent != "" {
		cfg.Theme.Accent = file.Theme.Accent
	}
	if file.Theme.SelectedFg != "" {
		cfg.Theme.SelectedFg = file.Theme.SelectedFg
	}
	if file.Theme.FilterFg != "" {
		cfg.Theme.FilterFg = file.Theme.FilterFg
	}
	if file.Icons.Play != "" {
		cfg.Icons.Play = file.Icons.Play
	}
	if file.Icons.Pause != "" {
		cfg.Icons.Pause = file.Icons.Pause
	}
	if file.Icons.Volume != "" {
		cfg.Icons.Volume = file.Icons.Volume
	}
	if file.Icons.Shuffle != "" {
		cfg.Icons.Shuffle = file.Icons.Shuffle
	}
	if file.Spotify.ClientID != "" {
		cfg.Spotify.ClientID = file.Spotify.ClientID
	}
	if file.Spotify.ClientSecret != "" {
		cfg.Spotify.ClientSecret = file.Spotify.ClientSecret
	}
	if file.Spotify.RedirectURI != "" {
		cfg.Spotify.RedirectURI = file.Spotify.RedirectURI
	}
	if file.Spotify.SearchLimit != 0 {
		cfg.Spotify.SearchLimit = file.Spotify.SearchLimit
	}
	if file.UI.TickInterval != 0 {
		cfg.UI.TickInterval = file.UI.TickInterval
	}
	return cfg
}
