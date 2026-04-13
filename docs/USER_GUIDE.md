# slp User Guide

## First Run

On first launch, slp opens a browser window for Spotify OAuth authentication.
After logging in, the token is saved to `~/.config/slp/token.json` and reused on subsequent runs.

A default config file is created at `~/.config/slp/config.toml` if it doesn't exist.

---

## Typical Workflow

**1. Start slp in a dedicated pane**

In tmux, open a small pane at the bottom:

```
Ctrl-b : split-window -v -l 1 'slp'
```

In zellij, open a new pane:

```
Ctrl-p n  (then run slp)
```

**2. Open the playlist selector**

Press `space`. A floating popup appears using your multiplexer's native window.

**3. Find a playlist**

- Press `enter` with no input to load your own playlists (sorted by track count)
- Type a word and press `enter` to search Spotify for matching playlists
- Navigate the list with `j` / `k`

**4. Select a device**

After choosing a playlist, a device picker appears. Select the device you want to play on.

**5. Go back to work**

The popup closes. The player line updates. Done.

---

## Playlist Selection in Detail

The popup opens in filter mode with a text cursor:

```
╭─ playlists ──────────────────────────────────────╮
│ ❯ █                                              │
│ ────────────────────────────────────────────────  │
│ enter: your playlists  /word: search             │
╰──────────────────────────────────────────────────╯
```

**Loading your playlists:**
Press `enter` with empty input. Your playlists load, sorted by track count (most tracks first).

**Searching Spotify:**
Type a word (e.g. `lofi`) and press `enter`. Returns matching public playlists.

**Filtering loaded results:**
After loading your playlists, press `/` to re-enter filter mode. Typing narrows the list client-side without a new API call.

**Navigating:**
Use `j` / `k` or arrow keys. Press `enter` to select.

**Going back:**
`backspace` from list mode returns to filter mode.
`backspace` from filter mode with empty input closes the popup.
`esc` / `q` closes the popup from anywhere.

---

## Progress Bar

The progress bar fills the available space between the track info and the right-side indicators.

With a theme accent color set, the bar animates with a sine wave gradient. Without a hex color, it uses plain `=` / `-` characters.

---

## Themes

Set a built-in theme in config:

```toml
[theme]
name = "tokyo-night"
```

Available themes: `dracula`, `iceberg`, `monokai`, `solarized-dark`, `solarized-light`, `nord`, `gruvbox`, `tokyo-night`, `catppuccin`, `rose-pine`, `mono`

Override individual colors:

```toml
[theme]
name = "nord"
accent = "#88c0d0"      # progress bar, borders, icons
selected_fg = "#eceff4" # selected item text
filter_fg = "#ebcb8b"   # filter prompt
```

Colors accept hex (`#rrggbb`) or terminal 256-color numbers.

---

## Icons

Defaults use Nerd Fonts glyphs. If Nerd Fonts is not available, set plain text fallbacks:

```toml
[icons]
play    = "▶"
pause   = "⏸"
volume  = "V"
shuffle = "S"
```

---

## Volume

Volume control requires a Spotify Premium device that supports it.
If the active device doesn't support volume, `j` / `k` will show a status message instead.
The volume indicator is hidden automatically on unsupported devices.

---

## Without a Multiplexer

slp works in any terminal. In non-tmux/zellij environments:

- `space` opens an inline popup centered on the screen
- `?` shows key bindings as an inline overlay

The single-line player still works in a full-height terminal window.

---

## Token Management

```sh
slp --logout   # remove stored token, re-authenticate on next launch
```

Token is stored at `~/.config/slp/token.json` and refreshed automatically before expiry.

---

## Troubleshooting

**"no active Spotify device"**
Open Spotify on any device (desktop, phone, web player) and start playing something.
The active device will be detected on the next poll (within 2 seconds).

**Popup doesn't appear in tmux**
Make sure your tmux version supports `display-popup` (tmux 3.2+).

**Icons show as squares or question marks**
Install a [Nerd Font](https://www.nerdfonts.com/) and configure your terminal to use it,
or set plain text icons in config.toml.

**OAuth callback fails**
Make sure `http://127.0.0.1:8888/callback` is listed as a Redirect URI in your Spotify developer app settings.
