package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const version = "v1.0.2"

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	logoutFlag := flag.Bool("logout", false, "remove stored token and exit")
	debugFlag := flag.Bool("debug", false, "enable debug logging")
	selectFlag := flag.Bool("select", false, "popup selection mode (used internally with tmux display-popup)")
	keysFlag := flag.Bool("keys", false, "show key bindings (used internally with tmux display-popup)")
	flag.Parse()

	if *versionFlag {
		fmt.Println("slp", version)
		return
	}

	if *logoutFlag {
		if err := DeleteToken(); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "logout error:", err)
			os.Exit(1)
		}
		fmt.Println("Logged out.")
		return
	}

	cfg = LoadConfig()
	theme := cfg.Theme.resolve()
	if theme.UseTerminalColor {
		if hex := queryTerminalFgHex(); hex != "" {
			theme.Accent = hex
		}
	}
	initStyles(theme)
	initGradient()

	if *keysFlag {
		m := newModel(nil, false, false, true)
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, _ = p.Run()
		return
	}

	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	if clientID == "" {
		clientID = cfg.Spotify.ClientID
	}
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = cfg.Spotify.ClientSecret
	}
	redirectURI := os.Getenv("SPOTIFY_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = cfg.Spotify.RedirectURI
	}

	if clientID == "" || clientSecret == "" {
		fmt.Fprintln(os.Stderr, "error: Spotify credentials not set.")
		fmt.Fprintln(os.Stderr, "  Set environment variables:  SPOTIFY_CLIENT_ID / SPOTIFY_CLIENT_SECRET")
		fmt.Fprintln(os.Stderr, "  Or add to config file:      ~/.config/slp/config.toml")
		fmt.Fprintln(os.Stderr, "  See: https://github.com/256x/slp#setup")
		os.Exit(1)
	}

	debugLog := func(format string, args ...any) {}
	if *debugFlag {
		logger := log.New(os.Stderr, "[debug] ", log.LstdFlags)
		debugLog = logger.Printf
	}

	// Load or obtain token
	token, err := LoadToken()
	if err != nil {
		debugLog("no stored token, starting OAuth flow")
		token, err = Authenticate(clientID, clientSecret, redirectURI)
		if err != nil {
			fmt.Fprintln(os.Stderr, "authentication failed:", err)
			os.Exit(1)
		}
		if err := SaveToken(token); err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not save token:", err)
		}
	}

	client := NewSpotifyClient(token, clientID, clientSecret, debugLog)

	// Try to restore from cache if present
	m := newModel(client, *debugFlag, *selectFlag, false)
	if !*selectFlag {
		if cached, err := LoadPlaybackCache(); err == nil {
			m.playback = cached
		}
	}

	opts := []tea.ProgramOption{tea.WithAltScreen()}
	p := tea.NewProgram(m, opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
