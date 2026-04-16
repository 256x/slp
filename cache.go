package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func cacheDir() string {
	if base := os.Getenv("XDG_CACHE_HOME"); base != "" {
		return filepath.Join(base, "slp")
	}
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
	data, err := json.MarshalIndent(s, "", "  ")
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
	var s PlaybackState
	if err := json.Unmarshal(data, &s); err != nil {
		return PlaybackState{}, err
	}
	return s, nil
}
