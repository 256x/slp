package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type CachedState struct {
	Track         string     `json:"track"`
	Artist        string     `json:"artist"`
	ProgressMS    int        `json:"progress_ms"`
	DurationMS    int        `json:"duration_ms"`
	VolumePercent *int       `json:"volume_percent,omitempty"`
	Shuffle       bool       `json:"shuffle"`
	Playing       bool       `json:"playing"`
	DeviceID      string     `json:"device_id"`
	DeviceName    string     `json:"device_name"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func cacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cache/slp"
	}
	return filepath.Join(home, ".cache", "slp")
}

func stateCachePath() string {
	return filepath.Join(cacheDir(), "state.json")
}

func SavePlaybackCache(s PlaybackState) error {
	dir := cacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	c := CachedState{
		Track:         s.Track,
		Artist:        s.Artist,
		ProgressMS:    s.ProgressMS,
		DurationMS:    s.DurationMS,
		VolumePercent: s.VolumePercent,
		Shuffle:       s.Shuffle,
		Playing:       s.Playing,
		DeviceID:      s.DeviceID,
		DeviceName:    s.DeviceName,
		UpdatedAt:     time.Now(),
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateCachePath(), data, 0o600)
}

func LoadPlaybackCache() (PlaybackState, error) {
	data, err := os.ReadFile(stateCachePath())
	if err != nil {
		return PlaybackState{}, err
	}
	var c CachedState
	if err := json.Unmarshal(data, &c); err != nil {
		return PlaybackState{}, err
	}
	return PlaybackState{
		Track:         c.Track,
		Artist:        c.Artist,
		ProgressMS:    c.ProgressMS,
		DurationMS:    c.DurationMS,
		VolumePercent: c.VolumePercent,
		Shuffle:       c.Shuffle,
		Playing:       c.Playing,
		DeviceID:      c.DeviceID,
		DeviceName:    c.DeviceName,
	}, nil
}
