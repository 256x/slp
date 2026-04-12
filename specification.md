# spolistplay v2 — Design Specification

Date: 2026-04-12 JST

## Overview

spolistplay v2 is a minimal terminal Spotify player designed for keyboard-driven environments such as tmux, but it must not depend on tmux.

The UI philosophy is:

* always show a single-line player
* show a centered popup playlist selector only when needed

Goals:

* minimal UI
* cross-platform terminal compatibility
* single binary distribution
* keyboard-driven workflow
* low API usage
* good behavior in tmux, normal terminals, and Windows Terminal

This version is intentionally simpler than the existing Python implementation. It keeps the useful parts of the original behavior:

* Spotify OAuth authentication
* playlist search / playlist selection
* playback control
* polling-based playback state updates
* rate-limit handling
* local cache

It intentionally drops the large full-screen curses interface.

## Technology Stack

Language:

Go

Primary TUI framework:

Bubble Tea

Optional supporting libraries:

* bubbles
* lipgloss

Spotify communication:

* Spotify Web API over HTTP
* no heavy SDK required unless there is a strong reason

## High-Level Behavior

At runtime, the application behaves like this:

* launch the app
* authenticate if needed
* detect current playback/device state
* show a single-line player
* accept keyboard commands
* when playlist selection is requested, open a centered popup selector
* after selection, start playback and return to the single-line player

## Main UI

Default UI is a single-line status/player display.

Example:

▶ Stitches @ Shawn Mendes  02:01/03:26  Vol:20  Shuffle:Off

Paused example:

⏸ Stitches @ Shawn Mendes  02:01/03:26  Vol:20  Shuffle:Off

If volume is not available:

▶ Stitches @ Shawn Mendes  02:01/03:26  Shuffle:Off

If nothing is playing:

⏸ No active playback

The UI must fit in one terminal line whenever possible.

When the terminal width is too small, the track/artist section should be truncated first.

Preferred layout:

[PLAYSTATE] Track @ Artist  Progress/Duration  Vol:NN  Shuffle:On|Off

Symbols:

* ▶ playing
* ⏸ paused

## Popup Playlist Selector

The playlist selector must appear as a centered popup-like box rendered inside the terminal using Bubble Tea, not using tmux-specific features.

Example:

┌──────── Select Playlist ────────┐
│ chill chill chill               │
│ coding focus                    │
│ jazz night                      │
└─────────────────────────────────┘

Behavior:

* centered in current terminal viewport
* must work in tmux and non-tmux terminals
* must work on Linux, macOS, and Windows Terminal
* should redraw properly on terminal resize

Controls inside popup:

* Up / Down arrows: move selection
* j / k: move selection
* Enter: select playlist
* Esc: cancel popup
* / : optionally focus search input inside popup if implemented
* q: cancel popup

If popup is canceled, return to the single-line player without changing playback.

## Keyboard Controls

Global controls in main player mode:

* h: previous track
* Left Arrow: previous track
* l: next track
* Right Arrow: next track
* p: play/pause toggle
* j: volume down
* Down Arrow: volume down
* k: volume up
* Up Arrow: volume up
* s: toggle shuffle
* P: open playlist selector
* q: quit application

Notes:

* uppercase P is reserved for playlist selector
* lowercase p toggles play/pause
* if the device does not support volume control, j/k should show a short status/error message rather than failing silently

## Playlist Source Behavior

The original Python version supports:

* playlist search
* fetching current user playlists when query is "0"

For v2, playlist selection behavior should be:

Preferred behavior:

* popup opens with the current user's playlists first

Optional enhancement:

* popup has a small filter/search mode to narrow playlists client-side
* optionally support remote Spotify search later

Minimum required behavior:

* fetch current user's playlists
* show them in popup
* allow keyboard selection
* on selection, start playback

## Device Behavior

The original Python version allows device selection.

For v2:

Minimum behavior:

* use the currently active Spotify device if available

If no active device exists:

* show a short error/status message such as:
  "No active Spotify device"

Optional enhancement:

* support a second popup for device selection

But for v2, device selection is not required if it significantly complicates the implementation.

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

* SPOTIFY_CLIENT_ID
* SPOTIFY_CLIENT_SECRET
* SPOTIFY_REDIRECT_URI

Default redirect URI if not explicitly provided:

http://127.0.0.1:8888/callback

If required environment variables are missing, the app must exit with a clear message.

## Token Storage

Store token file at:

~/.config/spolistplay/token.json

Suggested fields:

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

Store current playback state at:

~/.cache/spolistplay/state.json

Suggested fields:

* track
* artist
* progress_ms
* duration_ms
* volume_percent
* shuffle
* playing
* device_name
* device_id
* updated_at

Purpose:

* local cache for UI state
* survive temporary API failures
* reduce unnecessary redraw logic complexity

## Spotify API Endpoints

The app will need these endpoints:

* GET /v1/me/player
* PUT /v1/me/player/play
* PUT /v1/me/player/pause
* POST /v1/me/player/next
* POST /v1/me/player/previous
* PUT /v1/me/player/volume
* PUT /v1/me/player/shuffle
* GET /v1/me/playlists
* optionally GET /v1/search if remote search is later added

## Polling Strategy

Polling endpoint:

GET /v1/me/player

Polling interval:

2 seconds

Rationale:

* low enough to feel responsive
* conservative enough to avoid excessive API requests
* similar to the current Python implementation

Polling must update:

* track name
* artist name
* progress
* duration
* volume if available
* shuffle state
* play state
* active device info

## Rate Limit Handling

Spotify may return HTTP 429.

Behavior:

* read Retry-After header
* sleep for Retry-After seconds
* retry request
* keep last known cached playback state during backoff

The UI should continue to display the most recent cached state while waiting.

## Error Handling

Errors must be non-destructive whenever possible.

Typical cases:

* no active device
* token refresh failure
* network failure
* 429 rate limit
* playback action failure
* terminal too narrow

Desired behavior:

* keep app running when reasonable
* show short status/error text in the single-line UI or temporary status area
* only exit for unrecoverable errors such as missing OAuth config

## Rendering Behavior

Main player mode:

* single-line render only
* must redraw on state changes
* must redraw on terminal resize
* should truncate long text safely

Popup mode:

* centered box
* handles resize
* returns to main UI after selection or cancel

## Internal Architecture

Suggested file layout:

spolistplay/
├── main.go
├── go.mod
├── auth.go
├── spotify.go
├── player.go
├── playlist.go
├── ui.go
├── cache.go
└── README.md

Responsibilities:

main.go
Program entry point, setup, program startup, top-level Bubble Tea launch.

auth.go
OAuth flow, token loading, token refresh, callback handling.

spotify.go
Spotify HTTP client and API request wrappers.

player.go
Playback commands: play, pause, next, previous, volume, shuffle.

playlist.go
Playlist retrieval and popup selection model/data.

ui.go
Bubble Tea model, view rendering, key handling, popup rendering.

cache.go
Read/write state cache and token helpers if desired.

## Bubble Tea Internal State Model

Main application state should include at least:

* current playback state
* cached error/status message
* popup open/closed
* playlist list
* selected playlist index
* loading flags
* terminal width/height
* last successful poll timestamp

Recommended main model fields:

* width int
* height int
* ready bool
* popupOpen bool
* popupMode string
* playlists []Playlist
* playlistCursor int
* playback PlaybackState
* statusMessage string
* lastError error
* loading bool
* quitting bool

Recommended PlaybackState fields:

* Track string
* Artist string
* ProgressMS int
* DurationMS int
* VolumePercent *int
* Shuffle bool
* Playing bool
* DeviceID string
* DeviceName string

Recommended Playlist fields:

* ID string
* Name string
* Owner string
* TrackCount int
* URI string

## Bubble Tea Message Types

Suggested messages:

* tickMsg
* playbackUpdatedMsg
* playlistsLoadedMsg
* playlistSelectedMsg
* authCompletedMsg
* apiErrorMsg
* statusMsg
* windowSizeMsg

Polling flow:

* startup triggers initial playback fetch
* periodic tick triggers playback refresh
* playlist popup open triggers playlist load if needed
* selecting playlist triggers playback start command

## CLI Behavior

Default command:

spolistplay

Optional flags:

* --version
* --logout
* --debug

Behavior:

spolistplay
Start UI, authenticate if needed, show player.

spolistplay --logout
Delete local token file and exit.

spolistplay --version
Print version and exit.

## Logging

Keep logging minimal by default.

Suggested:

* no noisy logs in normal mode
* debug logs only with --debug
* log to stderr if enabled

## Cross-Platform Requirements

Must support:

* Linux terminals
* macOS Terminal / iTerm2
* Windows Terminal
* tmux panes

Must not require:

* tmux popup
* fzf
* external binaries
* Python
* shell-specific features

## Non-Goals

Do not implement in v2:

* full-screen playback UI
* advanced device browser unless trivial
* album browser
* lyrics
* playlist editing
* library management
* tmux-only integrations
* Zebar-specific UI logic

## Acceptance Criteria

The implementation is accepted if all of the following are true:

1. `go build` succeeds on the project.
2. Running `spolistplay` opens a single-line player UI.
3. OAuth works using environment variables and stores token locally.
4. Playback state updates every 2 seconds.
5. h/l/p/j/k/s and arrow keys work.
6. Pressing P opens a centered playlist selector popup.
7. Selecting a playlist starts playback on the active device.
8. Esc closes the popup without changing playback.
9. 429 handling respects Retry-After.
10. The app works in tmux and non-tmux terminals.
11. The app does not depend on tmux-specific APIs.
12. Long track names truncate safely.
13. Volume display is omitted or gracefully degraded when unsupported.
14. `--logout` removes token state and exits cleanly.

## Build Requirements

The generated project must include:

* full Go source
* go.mod
* README with build and setup instructions

Build command:

go build

Optional cross compile example:

GOOS=windows GOARCH=amd64 go build

## README Requirements

README must include:

* what the app does
* Spotify developer app setup
* required environment variables
* how to run
* key bindings
* limitations
* Premium requirement for playback control

## Implementation Guidance

Keep the code simple.

Priorities:

* correctness
* portability
* small codebase
* clear separation of concerns

Avoid unnecessary abstraction and overengineering.

The project should feel like a compact, well-structured CLI app rather than a framework-heavy TUI application.

You are an expert Go engineer.

Implement the attached specification exactly:

* spolistplay_v2_design.md

This is a real project implementation task, not a mockup.

Your job is to generate the complete Go project.

The application is a minimal Spotify terminal player with:

* a single-line main player UI
* a centered popup playlist selector
* Spotify OAuth authentication
* playback control
* periodic polling
* local token/cache storage
* cross-platform terminal behavior

Use these constraints:

* Language: Go
* TUI framework: Bubble Tea
* Cross-platform: Linux, macOS, Windows Terminal
* Must work inside tmux, but must not depend on tmux-specific APIs
* Single binary CLI tool
* Minimal dependencies
* Idiomatic Go
* Clean, readable code
* No extra features beyond the specification unless required for correctness

Deliverables:

1. Full Go source code
2. go.mod
3. README.md
4. Build instructions
5. Any small helper structs/types required

Project structure to generate:

spolistplay/
├── main.go
├── go.mod
├── auth.go
├── spotify.go
├── player.go
├── playlist.go
├── ui.go
├── cache.go
└── README.md

Rules:

* Follow the specification strictly
* Code must compile with `go build`
* Handle errors properly
* Avoid overengineering
* Avoid global mutable state unless clearly justified
* Keep architecture small and practical
* Prefer explicit code over unnecessary abstraction

Environment variables to use:

* SPOTIFY_CLIENT_ID
* SPOTIFY_CLIENT_SECRET
* SPOTIFY_REDIRECT_URI

Default redirect URI if missing:

http://127.0.0.1:8888/callback

Token file:

~/.config/spolistplay/token.json

Playback cache file:

~/.cache/spolistplay/state.json

Minimum required runtime behavior:

* startup authentication flow
* single-line player render
* 2-second polling of current playback
* key controls: h/l/p/j/k/s/P/q and arrow keys
* centered popup playlist selector on uppercase P
* playlist selection from current user playlists
* start playback on selected playlist using current active device
* retry/sleep handling for HTTP 429 using Retry-After
* graceful behavior when there is no active device
* proper truncation for narrow terminals

Do not skip the popup implementation.

Do not replace the popup with tmux popup, fzf, or shell commands.

Use Bubble Tea to render both:

* the single-line main player
* the centered popup selector

Implementation guidance:

* define a Bubble Tea model for the app
* maintain playback state and popup state in the model
* use periodic ticks for polling
* implement Spotify API using direct HTTP requests if practical
* implement OAuth with localhost callback receiver
* auto-refresh expired tokens
* keep the UI compact and clean

Required acceptance conditions:

1. `go build` works
2. `spolistplay` launches
3. OAuth works with provided env vars
4. Player line updates every 2 seconds
5. Key bindings work
6. Popup opens and closes correctly
7. Playlist selection starts playback
8. The app works in ordinary terminals and tmux panes
9. The app does not depend on external binaries
10. README explains setup and usage

Output format:

Return the complete project as files, not a high-level explanation.

# Internal State Model for Implementation

## Core types

### PlaybackState

```go
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
```

### Playlist

```go
type Playlist struct {
    ID         string
    Name       string
    Owner      string
    TrackCount int
    URI        string
}
```

### TokenData

```go
type TokenData struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    TokenType    string    `json:"token_type"`
    Expiry       time.Time `json:"expiry"`
    Scope        string    `json:"scope"`
}
```

### App Model

```go
type model struct {
    width          int
    height         int
    ready          bool
    popupOpen      bool
    popupMode      string
    playlists      []Playlist
    playlistCursor int
    playback       PlaybackState
    statusMessage  string
    loading        bool
    quitting       bool
    client         *SpotifyClient
}
```

## Suggested message types

```go
type tickMsg time.Time
type playbackUpdatedMsg struct {
    State PlaybackState
}
type playlistsLoadedMsg struct {
    Playlists []Playlist
}
type apiErrorMsg struct {
    Err error
}
type statusMsg struct {
    Text string
}
```

## Suggested update flow

* startup:

  * initialize auth
  * initialize client
  * fetch first playback state
  * start tick command

* every tick:

  * fetch playback state
  * update model
  * schedule next tick

* on key press:

  * h/left => previous track
  * l/right => next track
  * p => toggle play/pause
  * j/down => volume down
  * k/up => volume up
  * s => toggle shuffle
  * P => open popup and load playlists if needed
  * q => quit

* popup mode:

  * up/down/j/k => move
  * enter => play selected playlist
  * esc/q => close popup

## Rendering rules

### Main line

Prefer rendering like:

▶ Track @ Artist  02:01/03:26  Vol:20  Shuffle:Off

If width is narrow:

* truncate track first
* then artist
* keep progress visible if possible
* drop volume before dropping play state

### Popup

* centered in viewport
* border box
* title: Select Playlist
* visible rows depend on terminal height
* selected row highlighted

## Status message behavior

`statusMessage` should be short-lived.

Examples:

* No active Spotify device
* Volume not supported
* Rate limited, retrying...
* Playback started
* Authentication failed

Can be displayed in main line temporarily or as a second short overlay line if Bubble Tea makes that practical, but main design target remains one-line idle state.

## Spotify client responsibilities

The client should expose methods like:

```go
GetCurrentPlayback(ctx context.Context) (PlaybackState, error)
GetUserPlaylists(ctx context.Context) ([]Playlist, error)
PlayPlaylist(ctx context.Context, playlistURI string, deviceID string) error
Pause(ctx context.Context, deviceID string) error
Resume(ctx context.Context, deviceID string) error
Next(ctx context.Context, deviceID string) error
Previous(ctx context.Context, deviceID string) error
SetVolume(ctx context.Context, deviceID string, volume int) error
SetShuffle(ctx context.Context, deviceID string, state bool) error
```

## Rate-limit handling contract

If Spotify returns 429:

* parse Retry-After
* sleep
* retry once or retry through caller policy
* preserve last known state in UI

## Cache contract

`state.json` should be written after successful playback fetch.

Suggested JSON fields:

```json
{
  "track": "Stitches",
  "artist": "Shawn Mendes",
  "progress_ms": 121000,
  "duration_ms": 206000,
  "volume_percent": 20,
  "shuffle": false,
  "playing": true,
  "device_id": "abc",
  "device_name": "Desktop",
  "updated_at": "2026-04-12T12:34:56+09:00"
}
```

# Implementation Notes and Edge Cases

## OAuth callback

Use a small localhost HTTP server bound to the redirect URI host/port.

Expected default:

http://127.0.0.1:8888/callback

The app should:

* start temporary callback server
* open browser
* wait for code
* exchange token
* shut down callback server

## Active device requirement

Spotify playback control typically requires an active device and Premium for full control.

If no device is active:

* show "No active Spotify device"
* do not crash
* keep app running

## Volume support

Some devices do not support volume control.

If unsupported:

* hide volume or show N/A
* j/k should not crash
* show short message if attempted

## Terminal resizing

Bubble Tea window size messages must update:

* single-line truncation logic
* popup centering
* popup height/width

## Playlist loading

Current user playlists should be paginated if necessary.

Do not assume one page only.

## Narrow terminals

If terminal width is very small:

* keep play symbol
* try to keep progress
* aggressively truncate track/artist
* popup may reduce width/height accordingly

## Logging

Use debug logging only behind --debug.

Normal mode should remain quiet.

## Simplicity preference

Do not build a large architecture.

This is a compact terminal app.

Keep code practical and understandable.


