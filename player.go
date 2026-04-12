package main

// PlaybackState holds current Spotify playback info.
type PlaybackState struct {
	Track         string `json:"track"`
	Artist        string `json:"artist"`
	ProgressMS    int    `json:"progress_ms"`
	DurationMS    int    `json:"duration_ms"`
	VolumePercent *int   `json:"volume_percent,omitempty"`
	Shuffle       bool   `json:"shuffle"`
	Playing       bool   `json:"playing"`
	DeviceID      string `json:"device_id"`
	DeviceName    string `json:"device_name"`
}

// Playlist holds info for a single user playlist.
type Playlist struct {
	ID         string
	Name       string
	Owner      string
	TrackCount int
	URI        string
}

// Device holds info for a Spotify Connect device.
type Device struct {
	ID       string
	Name     string
	Type     string
	IsActive bool
}
