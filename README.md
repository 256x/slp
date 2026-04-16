# slp

**A Spotify player that lives in one line of your terminal.**

```
▶ [ Lo-Fi Hip Hop @ ChilledCow ] ══════════────  󰕾 :42  󰒝 :+
```

---

## Concept

Most Spotify clients want your attention. slp doesn't.

slp is built for people who work in the terminal. You have tmux or zellij open, you're deep in a task, and you want music — without switching windows, without a GUI, without breaking flow.

The entire player fits in a single line. It sits quietly at the bottom of a pane. You forget it's there, and that's the point.

**Why playlists only?**

A song ends in 3 minutes. An artist radio ends eventually. A playlist — especially one made by someone else — runs indefinitely. Spotify has millions of community-made playlists for every mood, genre, and activity. slp is designed around that: pick a playlist, go back to work, keep listening.

**Why tmux/zellij?**

The popup for playlist selection uses your multiplexer's native floating window. No alt-screen takeover, no context switch. Press `space`, pick a playlist, and the popup disappears. Your layout is untouched.

[User Guide](./docs/USER_GUIDE.md)

---

## Requirements

- Go 1.22+
- Spotify Premium
- [Nerd Fonts](https://www.nerdfonts.com/) (recommended, fallback to plain text icons)
- tmux or zellij (recommended, not required)

---

## Install

```sh
go install github.com/256x/slp@latest
```

Or download a pre-built binary from the [releases page](https://github.com/256x/slp/releases).

---

## Setup

1. Go to [developer.spotify.com/dashboard](https://developer.spotify.com/dashboard) and create an app
2. Add `http://127.0.0.1:8888/callback` as a Redirect URI
3. Set your credentials:

```sh
export SPOTIFY_CLIENT_ID=your_client_id
export SPOTIFY_CLIENT_SECRET=your_client_secret
# optional: override the default redirect URI
export SPOTIFY_REDIRECT_URI=http://127.0.0.1:8888/callback
```

Credentials can also be set in `~/.config/slp/config.toml` (created on first run).

On first launch, a browser window opens for OAuth. The token is saved locally and refreshed automatically.

---

## tmux setup (recommended)

Add a dedicated 1-line pane at the bottom of your session:

```sh
# in your tmux config or session script
split-window -v -l 1 'slp'
```

Or start it manually in any pane — the single-line UI works at any height.

## zellij setup (recommended)

```sh
zellij run -- slp
```

Or add it to your zellij layout as a fixed-size pane.

---

## Keys

| Key | Action |
|---|---|
| `enter` | play / pause |
| `space` | select playlist |
| `h` / `←` | previous track |
| `l` / `→` | next track |
| `k` / `↑` | volume +5 |
| `j` / `↓` | volume -5 |
| `s` | toggle shuffle |
| `?` | key bindings |
| `q` / `esc` | quit (pauses playback) |

In the playlist popup:

| Key | Action |
|---|---|
| `enter` (empty) | load your playlists |
| `enter` (with text) | search Spotify playlists |
| `j` / `k` | navigate list |
| `/` | re-enter filter mode |
| `backspace` | back / close |
| `esc` / `q` | close popup |

In the device picker:

| Key | Action |
|---|---|
| `j` / `k` | navigate list |
| `enter` | select device and start playback |
| `backspace` / `esc` | back to playlist popup |
| `q` / `ctrl+c` | quit |

---

## Configuration

Config file: `~/.config/slp/config.toml`

```toml
[theme]
name = "iceberg"   # dracula, nord, gruvbox, tokyo-night, catppuccin, rose-pine, mono, ...

[icons]
# plain text fallback if Nerd Fonts unavailable
# play = "▶"  pause = "⏸"  volume = "V"  shuffle = "S"

[spotify]
# client_id     = ""          # alternative to SPOTIFY_CLIENT_ID env var
# client_secret = ""          # alternative to SPOTIFY_CLIENT_SECRET env var
# redirect_uri  = "http://127.0.0.1:8888/callback"
# search_limit  = 20          # number of results returned by playlist search

[ui]
tick_interval = 2  # polling interval in seconds
```

---

## Flags

```
slp --version   print version
slp --logout    remove stored token
slp --debug     enable debug logging
```
