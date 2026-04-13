# slp — Design Specification

Date: 2026-04-13 JST

## Overview

slp is a minimal terminal Spotify player designed for keyboard-driven environments such as tmux and zellij.

The UI philosophy is:

* always show a single-line player
* show a centered popup only when needed
* native floating window support for tmux and zellij

Goals:

* minimal UI
* cross-platform terminal compatibility
* single binary distribution
* keyboard-driven workflow
* low API usage

## Technology Stack

Language: Go

Primary TUI framework: Bubble Tea

Supporting libraries:

* lipgloss
* go-colorful (animated progress bar)
* go-runewidth
* BurntSushi/toml

Spotify communication:

* Spotify Web API over HTTP (no SDK)

## High-Level Behavior

* launch the app
* authenticate if needed
* detect current playback/device state
* show a single-line player
* accept keyboard commands
* when playlist selection is requested:
  * in tmux/zellij: open floating popup via multiplexer
  * otherwise: show centered inline popup
* after selection, choose device and start playback

## Main UI

Default UI is a single-line status/player display.

Example:

▶ [ Stitches @ Shawn Mendes ] ====--------  󰕾 :20  󰒝 :-

Paused example:

⏸ [ Stitches @ Shawn Mendes ] ============  󰕾 :20  󰒝 :-

No active playback:

⏸ no active playback

The progress bar is animated with a gradient sine wave sweep when a theme accent color is set.

When the terminal width is too small, the track/artist section is truncated first. The progress bar fills remaining space.

Preferred layout:

[ICON] [ Track @ Artist ] [progress bar]  [ICON] :vol  [ICON] :shuffle

Icons default to Nerd Fonts glyphs and are configurable via config.toml.

## Multiplexer Integration

When running inside tmux or zellij, playlist selection and key bindings are displayed via the multiplexer's native floating window, not as an inline overlay.

Detection:

* tmux: `TMUX` environment variable
* zellij: `ZELLIJ` environment variable

tmux:

```
tmux display-popup -E -w 80% -h 60% slp --select
tmux display-popup -E -w 50 -h 18 slp --keys
```

zellij:

```
zellij run --floating --close-on-exit -- slp --select
zellij run --floating --close-on-exit -- slp --keys
```

The `--select` and `--keys` flags are internal and used exclusively by the multiplexer integration.

In all other terminals, popup and key bindings are rendered inline via Bubble Tea.

## Popup Playlist Selector

The playlist selector appears as a centered popup rendered in the terminal using Bubble Tea, or as a native floating window in tmux/zellij.

Layout example:

```
╭─ playlists ──────────────────────────────────────╮
│ ❯ █                                              │
│ ─────────────────────────────────────────────── │
│ enter: your playlists  /word: search             │
╰──────────────────────────────────────────────────╯
```

After results load:

```
╭─ playlists ──────────── 1/12 ───────────────────╮
│ ❯ chill                                          │
│ ──────────────────────────────────────────────── │
│ ❯ chill chill chill                          42  │
│   coding focus                               31  │
│   jazz night                                 18  │
╰──────────────────────────────────────────────────╯
```

Behavior:

* opens in filter/search mode with cursor prompt
* Enter with empty input: fetch current user's playlists (sorted by track count)
* Enter with text: search Spotify for playlists
* `/` from list mode: re-enter filter mode
* `backspace` in filter mode with text: delete last character
* `backspace` in filter mode with empty input: close popup
* `backspace` in list mode: return to filter mode
* results show track count as right label
* selected row highlighted

Controls inside popup:

* `j` / `↓`: move down
* `k` / `↑`: move up
* `Enter`: confirm selection
* `Esc` / `q`: close popup
* `backspace`: back / close (context-dependent, see above)
* `?`: show key bindings

## Device Selection Popup

After selecting a playlist, a device selection popup appears.

Layout example:

```
╭─ select device ─────────────────────────────────╮
│ ❯ Desktop                              computer ·│
│   iPhone                               smartphone│
╰──────────────────────────────────────────────────╯
```

The active device is marked with `·`. After selecting a device, playback starts on that device.

Controls:

* `j` / `↓` / `k` / `↑`: move selection
* `Enter`: start playback on selected device
* `Esc` / `backspace`: go back to playlist popup
* `q` / `ctrl+c`: quit

## Key Bindings

Global controls in main player mode:

* `enter`: play/pause toggle
* `h` / `←`: previous track
* `l` / `→`: next track
* `k` / `↑`: volume +5
* `j` / `↓`: volume -5
* `s` / `S`: toggle shuffle
* `space`: open playlist selector
* `?`: show key bindings
* `q` / `esc` / `ctrl+c`: quit (pauses playback if playing)

Notes:

* `enter` toggles play/pause
* `space` opens the playlist selector
* if the device does not support volume control, j/k show a status message

## Configuration

Config file path: `~/.config/slp/config.toml` (auto-created on first run)

Respects `XDG_CONFIG_HOME` if set.

```toml
[theme]
# built-in themes: dracula, iceberg, monokai, solarized-dark, solarized-light,
#                  nord, gruvbox, tokyo-night, catppuccin, rose-pine, mono
# name = "iceberg"
# accent      = "#84a0c6"
# selected_fg = "#c6c8d1"
# filter_fg   = "#89b8c2"

[icons]
# defaults use Nerd Fonts (nf-md-*)
# play    = "▶"
# pause   = "⏸"
# volume  = "V"
# shuffle = "S"

[spotify]
# client_id     = ""
# client_secret = ""
# redirect_uri  = "http://127.0.0.1:8888/callback"
# search_limit  = 20

[ui]
# tick_interval = 2
```

Environment variables take precedence over config file values for Spotify credentials.

## Spotify Authentication

Use Spotify OAuth Authorization Code flow.

Required behavior:

1. open browser for login
2. user logs in and grants access
3. receive callback on localhost
4. exchange authorization code for token
5. store token locally
6. refresh token automatically when expired

Environment variables:

* `SPOTIFY_CLIENT_ID`
* `SPOTIFY_CLIENT_SECRET`
* `SPOTIFY_REDIRECT_URI` (optional, default: `http://127.0.0.1:8888/callback`)

Credentials can also be set in config.toml. Environment variables take precedence.

If client ID and secret are missing from both env and config, the app exits with a clear error message.

## Token Storage

Store token file at: `~/.config/slp/token.json`

Fields:

* access_token
* refresh_token
* token_type
* expiry
* scope

Behavior:

* auto-create parent config directory if missing
* read token on startup
* refresh automatically when expired
* overwrite token file after refresh

## Playback State Cache

Store current playback state at: `~/.cache/slp/state.json`

Fields:

* track, artist
* progress_ms, duration_ms
* volume_percent
* shuffle, playing
* device_name, device_id
* updated_at

Purpose:

* restore last known state on startup
* survive temporary API failures

## Spotify API Endpoints

* `GET /v1/me/player`
* `PUT /v1/me/player/play`
* `PUT /v1/me/player/pause`
* `POST /v1/me/player/next`
* `POST /v1/me/player/previous`
* `PUT /v1/me/player/volume`
* `PUT /v1/me/player/shuffle`
* `PUT /v1/me/player` (transfer playback to device)
* `GET /v1/me/playlists`
* `GET /v1/search` (playlist search)
* `GET /v1/playlists/{id}/tracks` (first track URI for offset play)
* `GET /v1/me/player/devices`

## Polling Strategy

Polling endpoint: `GET /v1/me/player`

Polling interval: 2 seconds (configurable via `ui.tick_interval`)

Polling updates: track, artist, progress, duration, volume, shuffle state, play state, active device.

## Rate Limit Handling

Spotify may return HTTP 429.

Behavior:

* read `Retry-After` header
* sleep for Retry-After seconds
* retry request once
* keep last known cached playback state during backoff

## Error Handling

* no active device: show status message, keep app running
* token refresh failure: show status message
* network failure: show status message
* 429 rate limit: sleep and retry silently
* playback action failure: show status message
* terminal too narrow: truncate safely

Status messages are shown in the single-line UI and expire after 4 seconds.

## CLI Flags

```
slp                  start player
slp --version        print version and exit
slp --logout         delete stored token and exit
slp --debug          enable debug logging to stderr
slp --select         internal: playlist selection mode for multiplexer popup
slp --keys           internal: show key bindings for multiplexer popup
```

## Internal Architecture

```
slp/
├── main.go       entry point, flag handling, auth setup, program launch
├── auth.go       OAuth flow, token load/save/refresh, callback server
├── spotify.go    Spotify HTTP client, API request wrappers
├── player.go     PlaybackState and Device types
├── playlist.go   Playlist type, fetch/search/filter commands
├── ui.go         Bubble Tea model, Init/Update, key handlers, commands
├── view.go       View, all render functions, styles
├── cache.go      playback state cache read/write
├── config.go     TOML config load, theme resolution, built-in themes
├── browser.go    open browser for OAuth
├── go.mod
├── slp.toml      embedded default config template
└── README.md
```

## Bubble Tea Model

```go
type model struct {
    width, height   int
    ready           bool
    popupOpen       bool
    playlists       []Playlist
    filteredLists   []Playlist
    playlistCursor  int
    filterInput     string
    filterActive    bool
    devicePopupOpen bool
    devices         []Device
    deviceCursor    int
    deviceLoading   bool
    pendingURI      string
    playback        PlaybackState
    statusMessage   string
    statusExpiry    time.Time
    loading         bool
    helpOpen        bool
    quitting        bool
    selectMode      bool   // internal: running as --select popup
    keysMode        bool   // internal: running as --keys popup
    client          *SpotifyClient
    debug           bool
    vizTick         int    // drives progress bar animation
}
```

## Message Types

```go
type tickMsg time.Time
type vizTickMsg struct{}
type playbackUpdatedMsg struct{ State PlaybackState }
type playlistsLoadedMsg struct{ Playlists []Playlist; Err error }
type devicesLoadedMsg   struct{ Devices []Device; Err error }
type apiErrorMsg        struct{ Err error }
type statusMsg          struct{ Text string }
```

## Rendering

### Main player line

```
[icon] [ Track @ Artist ] [progress bar]  [vol icon] :N  [shuffle icon] :+/-
```

Progress bar:

* animated sine wave gradient when accent color is a valid hex color
* plain `=`/`-` fallback otherwise
* redraws at 100ms tick for animation

### Popup overlay

* centered in terminal viewport
* player line pinned to last row
* handles terminal resize
* popup width: min 20, max 60, terminal width − 4
* popup height: min 3, max 15, terminal height − 6

## Cross-Platform Requirements

Must support:

* Linux terminals
* macOS Terminal / iTerm2
* Windows Terminal
* tmux panes
* zellij panes

Must not require:

* fzf or external binaries
* Python
* shell-specific features

## Non-Goals

* full-screen playback UI
* album / library browser
* lyrics
* playlist editing
* additional multiplexer integrations beyond tmux and zellij

## Acceptance Criteria

1. `go build` succeeds.
2. `slp` launches and shows single-line player.
3. OAuth works via environment variables or config file.
4. Playback state updates every 2 seconds.
5. `enter` / `h` / `l` / `j` / `k` / `s` and arrow keys work.
6. `space` opens playlist selector (external popup in tmux/zellij, inline otherwise).
7. `?` shows key bindings (external popup in tmux/zellij, inline otherwise).
8. Playlist search and user playlist fetch both work.
9. Device selection popup appears after playlist selection.
10. Selecting a device starts playback.
11. `backspace` navigation works correctly in all popup states.
12. `--logout` removes token and exits cleanly.
13. `--keys` and `--select` work as standalone modes for multiplexer integration.
14. 429 handling respects Retry-After.
15. Long track names truncate safely.
16. Volume display degrades gracefully when unsupported.
17. App works in tmux, zellij, and ordinary terminals.
