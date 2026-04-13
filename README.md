# slp

spotify list player

```sh
go install github.com/256x/slp@latest
```

## setup

Spotify developer app が必要です。

1. https://developer.spotify.com/dashboard でアプリを作成
2. Redirect URI に `http://127.0.0.1:8888/callback` を追加
3. Client ID と Client Secret を環境変数に設定

```sh
export SPOTIFY_CLIENT_ID=your_client_id
export SPOTIFY_CLIENT_SECRET=your_client_secret
```

初回起動時にブラウザで OAuth 認証が開きます。以降はトークンが保存されます。

設定ファイルは初回起動時に `~/.config/slp/config.toml` に自動生成されます。

## keys

```
p      play / pause
l / →  next track
h / ←  previous track
k / ↑  volume +5
j / ↓  volume -5
s      toggle shuffle
P      playlist search
?      help
q      quit
```

## requirements

- Go 1.22+
- Spotify Premium
- [Nerd Fonts](https://www.nerdfonts.com/) (recommended)
