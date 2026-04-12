package main

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// playlistsLoadedMsg is sent when playlist fetch completes.
type playlistsLoadedMsg struct {
	Playlists []Playlist
	Err       error
}

func fetchPlaylists(client *SpotifyClient) tea.Cmd {
	return func() tea.Msg {
		playlists, err := client.GetUserPlaylists(context.Background())
		return playlistsLoadedMsg{Playlists: playlists, Err: err}
	}
}

func searchPlaylists(client *SpotifyClient, query string) tea.Cmd {
	return func() tea.Msg {
		playlists, err := client.SearchPlaylists(context.Background(), query)
		return playlistsLoadedMsg{Playlists: playlists, Err: err}
	}
}

// filterPlaylists returns playlists whose name contains query (case-insensitive).
func filterPlaylists(all []Playlist, query string) []Playlist {
	if query == "" {
		return all
	}
	q := strings.ToLower(query)
	var out []Playlist
	for _, p := range all {
		if strings.Contains(strings.ToLower(p.Name), q) {
			out = append(out, p)
		}
	}
	return out
}
