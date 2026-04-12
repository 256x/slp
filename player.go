package main

// PlaybackState holds current Spotify playback info.
type PlaybackState struct {
	Track         string
	Artist        string
	ProgressMS    int
	DurationMS    int
	VolumePercent *int
	Shuffle       bool
	Playing       bool
	DeviceID      string
	DeviceName    string
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
