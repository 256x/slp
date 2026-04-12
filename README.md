# spolistplay

A minimal terminal Spotify player. Single-line UI, keyboard-driven, works in tmux and normal terminals.

## Requirements

- Go 1.22+
- Spotify Premium account (required for playback control)
- A Spotify developer app

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
go build
```

Cross-compile for Windows:

```sh
GOOS=windows GOARCH=amd64 go build
```

## Run

```sh
./spolistplay
```

On first run, a browser window opens for Spotify OAuth. After login, the token is saved to `~/.config/spolistplay/token.json`.

## Key Bindings

| Key          | Action              |
|--------------|---------------------|
| `p`          | Play / Pause        |
| `l` / `→`   | Next track          |
| `h` / `←`   | Previous track      |
| `k` / `↑`   | Volume up (+10)     |
| `j` / `↓`   | Volume down (-10)   |
| `s`          | Toggle shuffle      |
| `P`          | Open playlist popup |
| `q`          | Quit                |

### Playlist popup

| Key          | Action              |
|--------------|---------------------|
| `j` / `↓`   | Move down           |
| `k` / `↑`   | Move up             |
| `Enter`      | Select playlist     |
| `/`          | Filter playlists    |
| `Esc` / `q`  | Close popup         |

## Flags

```
--version    Print version and exit
--logout     Remove stored token and exit
--debug      Enable debug logging to stderr
```

## Storage

- Token: `~/.config/spolistplay/token.json`
- Playback cache: `~/.cache/spolistplay/state.json`

## Limitations

- Playback control requires Spotify Premium
- Volume control is not available on all devices
- If no Spotify device is active, playback commands will fail with a message
- Spotify Web API rate limits apply; the app backs off automatically on HTTP 429
