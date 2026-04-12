# slp

A minimal terminal Spotify player. Single-line playback display, keyboard-driven, popup playlist search. Works in tmux and zellij (popup opens as a floating pane), and in normal terminals.

## Requirements

- Go 1.22+
- Spotify Premium account (required for playback control)
- A Spotify developer app
- [Nerd Fonts](https://www.nerdfonts.com/) (recommended for icons, see [Icons](#icons))

## Spotify Developer App Setup

1. Go to https://developer.spotify.com/dashboard
2. Create an app
3. Add `http://127.0.0.1:8888/callback` to the Redirect URIs
4. Note your Client ID and Client Secret

## Environment Variables

```sh
export SPOTIFY_CLIENT_ID=your_client_id
export SPOTIFY_CLIENT_SECRET=your_client_secret
export SPOTIFY_REDIRECT_URI=http://127.0.0.1:8888/callback  # optional, this is the default
```

## Build

```sh
go build -o slp .
```

Install to PATH:

```sh
go build -o slp . && cp slp ~/.local/bin/
```

## Usage

```sh
slp
```

On first run, a browser window opens for Spotify OAuth. After login, the token is saved to `~/.config/slp/token.json`.

The player line shows current playback:

```
󰐊 [ bad guy @ Billie Eilish ] ======---- 󰕾:65 󰒟:-
```

Press `P` to open the playlist search popup. In tmux or zellij, the popup appears as a floating pane over the full terminal.

### Playlist search

Type a search term and press `Enter` to search Spotify. Type `0` and press `Enter` to list your own playlists (sorted by track count). Results are then navigable with `j`/`k`.

## Key Bindings

### Player

| Key            | Action                |
|----------------|-----------------------|
| `p`            | Play / Pause          |
| `l` / `→`     | Next track            |
| `h` / `←`     | Previous track        |
| `k` / `↑`     | Volume +5             |
| `j` / `↓`     | Volume -5             |
| `s` / `S`      | Toggle shuffle        |
| `P`            | Open playlist search  |
| `?`            | Show key bindings     |
| `q` / `Esc`   | Quit (pauses playback)|

### Playlist search popup

| Key            | Action                        |
|----------------|-------------------------------|
| type + `Enter` | Search Spotify for playlists  |
| `0` + `Enter`  | Show your own playlists       |
| `j` / `↓`     | Move down                     |
| `k` / `↑`     | Move up                       |
| `Enter`        | Select → device selection     |
| `/`            | Search again                  |
| `Esc` / `q`   | Close                         |

### Device selection popup

| Key          | Action              |
|--------------|---------------------|
| `j` / `↓`  | Move down           |
| `k` / `↑`  | Move up             |
| `Enter`      | Play on device      |
| `Esc`        | Back to playlists   |

## Flags

```
--version    Print version and exit
--logout     Remove stored token and exit
--debug      Enable debug logging to stderr
```

## Icons

slp uses [Nerd Fonts](https://www.nerdfonts.com/) Material Design icons by default. If you don't use Nerd Fonts, edit `icons.go` and replace the constants with the fallback values shown in the comments:

```go
const (
    IconPlay    = "󰐊" // fallback: ▶
    IconPause   = "󰏤" // fallback: ⏸
    IconVolume  = "󰕾" // fallback: 🔊
    IconShuffle = "󰒟" // fallback: ⇄
)
```

## Storage

- Token: `~/.config/slp/token.json`
- Playback cache: `~/.cache/slp/state.json`

## Limitations

- Playback control requires Spotify Premium
- Volume control is not available on all devices
- Popup playlist search requires tmux or zellij for floating display; falls back to inline in plain terminals
