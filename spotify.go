package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type SpotifyClient struct {
	mu           sync.Mutex
	token        *TokenData
	clientID     string
	clientSecret string
	httpClient   *http.Client
	debugLog     func(string, ...any)
}

func NewSpotifyClient(token *TokenData, clientID, clientSecret string, debug func(string, ...any)) *SpotifyClient {
	return &SpotifyClient{
		token:        token,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		debugLog:     debug,
	}
}

func (c *SpotifyClient) Token() *TokenData {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

func (c *SpotifyClient) ensureFreshToken() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.token.Expiry.Add(-30 * time.Second)) {
		return nil
	}
	c.debugLog("refreshing token")
	newToken, err := RefreshToken(c.clientID, c.clientSecret, c.token)
	if err != nil {
		return fmt.Errorf("token refresh: %w", err)
	}
	c.token = newToken
	return SaveToken(newToken)
}

func (c *SpotifyClient) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	if err := c.ensureFreshToken(); err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, "https://api.spotify.com"+path, bodyReader)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	req.Header.Set("Authorization", "Bearer "+c.token.AccessToken)
	c.mu.Unlock()
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Handle 429 rate limit
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := 1
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if n, err := strconv.Atoi(ra); err == nil {
				retryAfter = n
			}
		}
		resp.Body.Close()
		c.debugLog("rate limited, sleeping %ds", retryAfter)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(retryAfter) * time.Second):
		}
		// Retry once
		return c.do(ctx, method, path, body)
	}

	return resp, nil
}

// --- Playback ---

type spotifyPlaybackResponse struct {
	IsPlaying  bool `json:"is_playing"`
	ProgressMS int  `json:"progress_ms"`
	Item       *struct {
		Name       string `json:"name"`
		DurationMS int    `json:"duration_ms"`
		Artists    []struct {
			Name string `json:"name"`
		} `json:"artists"`
	} `json:"item"`
	Device *struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		VolumePercent  *int   `json:"volume_percent"`
		SupportsVolume bool   `json:"supports_volume"`
	} `json:"device"`
	ShuffleState bool `json:"shuffle_state"`
}

func (c *SpotifyClient) GetCurrentPlayback(ctx context.Context) (PlaybackState, error) {
	resp, err := c.do(ctx, "GET", "/v1/me/player", nil)
	if err != nil {
		return PlaybackState{}, err
	}
	defer resp.Body.Close()

	// 204 = no active playback
	if resp.StatusCode == http.StatusNoContent {
		return PlaybackState{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return PlaybackState{}, fmt.Errorf("GET /v1/me/player: %d %s", resp.StatusCode, body)
	}

	var r spotifyPlaybackResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return PlaybackState{}, err
	}

	s := PlaybackState{
		Playing:    r.IsPlaying,
		ProgressMS: r.ProgressMS,
		Shuffle:    r.ShuffleState,
	}
	if r.Item != nil {
		s.Track = r.Item.Name
		s.DurationMS = r.Item.DurationMS
		if len(r.Item.Artists) > 0 {
			s.Artist = r.Item.Artists[0].Name
		}
	}
	if r.Device != nil {
		s.DeviceID = r.Device.ID
		s.DeviceName = r.Device.Name
		if r.Device.SupportsVolume {
			s.VolumePercent = r.Device.VolumePercent
		}
	}
	return s, nil
}

// --- Devices ---

type spotifyDevicesResponse struct {
	Devices []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		IsActive bool   `json:"is_active"`
	} `json:"devices"`
}

func (c *SpotifyClient) GetDevices(ctx context.Context) ([]Device, error) {
	resp, err := c.do(ctx, "GET", "/v1/me/player/devices", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET /v1/me/player/devices: %d %s", resp.StatusCode, body)
	}
	var r spotifyDevicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	out := make([]Device, len(r.Devices))
	for i, d := range r.Devices {
		out[i] = Device{ID: d.ID, Name: d.Name, Type: d.Type, IsActive: d.IsActive}
	}
	return out, nil
}

// --- Search ---

func (c *SpotifyClient) SearchPlaylists(ctx context.Context, query string) ([]Playlist, error) {
	path := "/v1/search?q=" + url.QueryEscape(query) + "&type=playlist&limit=20"
	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search: %d %s", resp.StatusCode, body)
	}
	var r struct {
		Playlists struct {
			Items []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				URI   string `json:"uri"`
				Owner struct {
					DisplayName string `json:"display_name"`
				} `json:"owner"`
				Tracks *struct {
					Total int `json:"total"`
				} `json:"tracks"`
			} `json:"items"`
		} `json:"playlists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	var out []Playlist
	for _, item := range r.Playlists.Items {
		if item.ID == "" {
			continue
		}
		total := 0
		if item.Tracks != nil {
			total = item.Tracks.Total
		}
		out = append(out, Playlist{
			ID:         item.ID,
			Name:       item.Name,
			URI:        item.URI,
			Owner:      item.Owner.DisplayName,
			TrackCount: total,
		})
	}
	return out, nil
}

// --- Playlists ---

type spotifyPlaylistsResponse struct {
	Items []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		URI   string `json:"uri"`
		Owner struct {
			DisplayName string `json:"display_name"`
		} `json:"owner"`
		Tracks struct {
			Total int `json:"total"`
		} `json:"tracks"`
	} `json:"items"`
	Next string `json:"next"`
}

func (c *SpotifyClient) GetUserPlaylists(ctx context.Context) ([]Playlist, error) {
	var all []Playlist
	path := "/v1/me/playlists?limit=50"

	for path != "" {
		resp, err := c.do(ctx, "GET", path, nil)
		if err != nil {
			return nil, err
		}
		var r spotifyPlaylistsResponse
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, item := range r.Items {
			all = append(all, Playlist{
				ID:         item.ID,
				Name:       item.Name,
				Owner:      item.Owner.DisplayName,
				TrackCount: item.Tracks.Total,
				URI:        item.URI,
			})
		}

		// Next is a full URL; extract path+query
		if r.Next != "" {
			u, err := url.Parse(r.Next)
			if err != nil || u.Path == "" {
				break
			}
			path = u.Path + "?" + u.RawQuery
		} else {
			path = ""
		}
	}
	return all, nil
}

// --- Playback controls ---

func (c *SpotifyClient) control(ctx context.Context, method, path string) error {
	resp, err := c.do(ctx, method, path, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return checkStatus(resp)
}

func (c *SpotifyClient) TransferPlayback(ctx context.Context, deviceID string) error {
	body := map[string]any{"device_ids": []string{deviceID}}
	resp, err := c.do(ctx, "PUT", "/v1/me/player", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return checkStatus(resp)
}

func (c *SpotifyClient) GetFirstTrackURI(ctx context.Context, playlistID string) (string, error) {
	path := "/v1/playlists/" + url.PathEscape(playlistID) + "/tracks?limit=1&fields=items(track(uri))"
	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	var r struct {
		Items []struct {
			Track struct {
				URI string `json:"uri"`
			} `json:"track"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", nil
	}
	if len(r.Items) > 0 {
		return r.Items[0].Track.URI, nil
	}
	return "", nil
}

func (c *SpotifyClient) PlayPlaylist(ctx context.Context, uri, deviceID, offsetURI string) error {
	body := map[string]any{"context_uri": uri}
	if offsetURI != "" {
		body["offset"] = map[string]string{"uri": offsetURI}
	}
	q := ""
	if deviceID != "" {
		q = "?device_id=" + url.QueryEscape(deviceID)
	}
	resp, err := c.do(ctx, "PUT", "/v1/me/player/play"+q, body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return checkStatus(resp)
}

func (c *SpotifyClient) Pause(ctx context.Context, deviceID string) error {
	return c.control(ctx, "PUT", "/v1/me/player/pause"+deviceQuery(deviceID))
}

func (c *SpotifyClient) Resume(ctx context.Context, deviceID string) error {
	return c.control(ctx, "PUT", "/v1/me/player/play"+deviceQuery(deviceID))
}

func (c *SpotifyClient) Next(ctx context.Context, deviceID string) error {
	return c.control(ctx, "POST", "/v1/me/player/next"+deviceQuery(deviceID))
}

func (c *SpotifyClient) Previous(ctx context.Context, deviceID string) error {
	return c.control(ctx, "POST", "/v1/me/player/previous"+deviceQuery(deviceID))
}

func (c *SpotifyClient) SetVolume(ctx context.Context, deviceID string, volume int) error {
	q := "?volume_percent=" + strconv.Itoa(volume)
	if deviceID != "" {
		q += "&device_id=" + url.QueryEscape(deviceID)
	}
	resp, err := c.do(ctx, "PUT", "/v1/me/player/volume"+q, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return checkStatus(resp)
}

func (c *SpotifyClient) SetShuffle(ctx context.Context, deviceID string, state bool) error {
	q := "?state=" + strconv.FormatBool(state)
	if deviceID != "" {
		q += "&device_id=" + url.QueryEscape(deviceID)
	}
	resp, err := c.do(ctx, "PUT", "/v1/me/player/shuffle"+q, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return checkStatus(resp)
}

func deviceQuery(deviceID string) string {
	if deviceID == "" {
		return ""
	}
	return "?device_id=" + url.QueryEscape(deviceID)
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("unexpected status %d", resp.StatusCode)
}
