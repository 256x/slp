package main

import (
	"context"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

// --- message types ---

type tickMsg time.Time
type vizTickMsg struct{}
type playbackUpdatedMsg struct{ State PlaybackState }
type apiErrorMsg struct{ Err error }
type statusMsg struct{ Text string }
type devicesLoadedMsg struct {
	Devices []Device
	Err     error
}

// --- model ---

type model struct {
	width           int
	height          int
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
	selectMode      bool
	keysMode        bool
	client          *SpotifyClient
	debug           bool
}

var cfg Config

func newModel(client *SpotifyClient, debug bool, selectMode, keysMode bool) model {
	return model{
		client:       client,
		debug:        debug,
		selectMode:   selectMode,
		keysMode:     keysMode,
		popupOpen:    selectMode,
		filterActive: selectMode,
	}
}

func (m model) Init() tea.Cmd {
	if m.selectMode || m.keysMode {
		return nil
	}
	return tea.Batch(
		fetchPlayback(m.client),
		tick(),
		vizTick(),
	)
}

// --- commands ---

func tick() tea.Cmd {
	d := time.Duration(cfg.UI.TickInterval) * time.Second
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func vizTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return vizTickMsg{}
	})
}

func fetchPlayback(client *SpotifyClient) tea.Cmd {
	return func() tea.Msg {
		state, err := client.GetCurrentPlayback(context.Background())
		if err != nil {
			return apiErrorMsg{Err: err}
		}
		return playbackUpdatedMsg{State: state}
	}
}

func fetchDevices(client *SpotifyClient) tea.Cmd {
	return func() tea.Msg {
		devices, err := client.GetDevices(context.Background())
		return devicesLoadedMsg{Devices: devices, Err: err}
	}
}

func startPlayback(ctx context.Context, client *SpotifyClient, uri, deviceID string) error {
	if deviceID != "" {
		_ = client.TransferPlayback(ctx, deviceID)
		time.Sleep(500 * time.Millisecond)
	}
	var offsetURI string
	parts := strings.Split(uri, ":")
	if len(parts) == 3 && parts[1] == "playlist" {
		offsetURI, _ = client.GetFirstTrackURI(ctx, parts[2])
	}
	return client.PlayPlaylist(ctx, uri, deviceID, offsetURI)
}

func skipCmd(client *SpotifyClient, deviceID string, fn func(context.Context, string) error) tea.Cmd {
	return func() tea.Msg {
		if err := fn(context.Background(), deviceID); err != nil {
			return apiErrorMsg{Err: err}
		}
		time.Sleep(300 * time.Millisecond)
		s, _ := client.GetCurrentPlayback(context.Background())
		return playbackUpdatedMsg{State: s}
	}
}

func playCmd(client *SpotifyClient, uri, deviceID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := startPlayback(ctx, client, uri, deviceID); err != nil {
			return apiErrorMsg{Err: err}
		}
		time.Sleep(300 * time.Millisecond)
		state, err := client.GetCurrentPlayback(ctx)
		if err != nil {
			return statusMsg{Text: "playback started"}
		}
		return playbackUpdatedMsg{State: state}
	}
}

func openExternalHelp() tea.Cmd {
	return func() tea.Msg {
		exe, err := os.Executable()
		if err != nil {
			exe = os.Args[0]
		}
		var cmd *exec.Cmd
		switch {
		case os.Getenv("TMUX") != "":
			cmd = exec.Command("tmux", "display-popup", "-E", "-w", "50", "-h", "18", exe, "--keys")
		case os.Getenv("ZELLIJ") != "":
			cmd = exec.Command("zellij", "run", "--floating", "--close-on-exit", "--", exe, "--keys")
		}
		if cmd != nil {
			_ = cmd.Run()
		}
		return nil
	}
}

func openExternalSelect(client *SpotifyClient) tea.Cmd {
	return func() tea.Msg {
		exe, err := os.Executable()
		if err != nil {
			exe = os.Args[0]
		}
		var cmd *exec.Cmd
		switch {
		case os.Getenv("TMUX") != "":
			cmd = exec.Command("tmux", "display-popup", "-E", "-w", "66", "-h", "22", exe, "--select")
		case os.Getenv("ZELLIJ") != "":
			cmd = exec.Command("zellij", "run", "--floating", "--close-on-exit", "--width", "66", "--height", "22", "--", exe, "--select")
		}
		if cmd != nil {
			_ = cmd.Run()
		}
		state, err := client.GetCurrentPlayback(context.Background())
		if err != nil {
			return nil
		}
		return playbackUpdatedMsg{State: state}
	}
}

// --- update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case tickMsg:
		return m, tea.Batch(fetchPlayback(m.client), tick())

	case vizTickMsg:
		grad.Tick(m.loading, m.playback.Playing)
		return m, vizTick()

	case playbackUpdatedMsg:
		m.playback = msg.State
		_ = SavePlaybackCache(msg.State)

	case playlistsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.setStatus("failed to load playlists: " + msg.Err.Error())
		} else {
			pl := msg.Playlists
			sort.Slice(pl, func(i, j int) bool {
				return pl[i].TrackCount > pl[j].TrackCount
			})
			m.playlists = pl
			m.filteredLists = pl
			m.filterInput = ""
			m.playlistCursor = 0
		}

	case devicesLoadedMsg:
		m.deviceLoading = false
		if msg.Err != nil {
			m.devicePopupOpen = false
			m.setStatus("failed to load devices: " + msg.Err.Error())
		} else {
			m.devices = msg.Devices
			m.deviceCursor = 0
			if len(m.devices) == 0 {
				m.devicePopupOpen = false
				m.setStatus("no devices available")
			}
		}

	case apiErrorMsg:
		m.setStatus(friendlyError(msg.Err))

	case statusMsg:
		m.setStatus(msg.Text)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *model) setStatus(s string) {
	m.statusMessage = s
	m.statusExpiry = time.Now().Add(4 * time.Second)
}

func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	return "error: " + err.Error()
}

// --- key handlers ---

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keysMode {
		return m, tea.Quit
	}
	if m.helpOpen {
		m.helpOpen = false
		return m, nil
	}
	if m.devicePopupOpen {
		return m.handleDevicePopupKey(msg)
	}
	if m.popupOpen {
		return m.handlePopupKey(msg)
	}
	return m.handlePlayerKey(msg)
}

func (m model) handlePlayerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		m.quitting = true
		client := m.client
		deviceID := m.playback.DeviceID
		playing := m.playback.Playing
		return m, func() tea.Msg {
			if playing && deviceID != "" {
				_ = client.Pause(context.Background(), deviceID)
			}
			return tea.QuitMsg{}
		}

	case "enter":
		if m.playback.DeviceID == "" {
			m.setStatus("no active Spotify device")
			return m, nil
		}
		var cmd tea.Cmd
		if m.playback.Playing {
			cmd = func() tea.Msg {
				if err := m.client.Pause(context.Background(), m.playback.DeviceID); err != nil {
					return apiErrorMsg{Err: err}
				}
				s := m.playback
				s.Playing = false
				return playbackUpdatedMsg{State: s}
			}
		} else {
			cmd = func() tea.Msg {
				if err := m.client.Resume(context.Background(), m.playback.DeviceID); err != nil {
					return apiErrorMsg{Err: err}
				}
				s := m.playback
				s.Playing = true
				return playbackUpdatedMsg{State: s}
			}
		}
		return m, cmd

	case "l", "right":
		if m.playback.DeviceID == "" {
			m.setStatus("no active Spotify device")
			return m, nil
		}
		return m, skipCmd(m.client, m.playback.DeviceID, m.client.Next)

	case "h", "left":
		if m.playback.DeviceID == "" {
			m.setStatus("no active Spotify device")
			return m, nil
		}
		return m, skipCmd(m.client, m.playback.DeviceID, m.client.Previous)

	case "k", "up":
		return m, m.adjustVolume(+5)

	case "j", "down":
		return m, m.adjustVolume(-5)

	case "?":
		if os.Getenv("TMUX") != "" || os.Getenv("ZELLIJ") != "" {
			return m, openExternalHelp()
		}
		m.helpOpen = true
		return m, nil

	case "s", "S":
		if m.playback.DeviceID == "" {
			m.setStatus("no active Spotify device")
			return m, nil
		}
		newShuffle := !m.playback.Shuffle
		m.playback.Shuffle = newShuffle
		return m, func() tea.Msg {
			if err := m.client.SetShuffle(context.Background(), m.playback.DeviceID, newShuffle); err != nil {
				return apiErrorMsg{Err: err}
			}
			return nil
		}

	case " ":
		if os.Getenv("TMUX") != "" || os.Getenv("ZELLIJ") != "" {
			return m, openExternalSelect(m.client)
		}
		m.popupOpen = true
		m.filterInput = ""
		m.filterActive = true
		m.filteredLists = nil
		m.playlists = nil
		m.playlistCursor = 0
	}

	return m, nil
}

func (m model) adjustVolume(delta int) tea.Cmd {
	if m.playback.DeviceID == "" {
		return func() tea.Msg { return statusMsg{Text: "no active Spotify device"} }
	}
	if m.playback.VolumePercent == nil {
		return func() tea.Msg { return statusMsg{Text: "volume not supported on this device"} }
	}
	newVol := *m.playback.VolumePercent + delta
	if newVol < 0 {
		newVol = 0
	}
	if newVol > 100 {
		newVol = 100
	}
	m.playback.VolumePercent = &newVol
	deviceID := m.playback.DeviceID
	client := m.client
	return func() tea.Msg {
		if err := client.SetVolume(context.Background(), deviceID, newVol); err != nil {
			return apiErrorMsg{Err: err}
		}
		return nil
	}
}

func (m model) handlePopupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filterActive {
		switch msg.String() {
		case "esc":
			if m.selectMode {
				return m, tea.Quit
			}
			m.filterActive = false
			m.filterInput = ""
			m.filteredLists = m.playlists
			m.playlistCursor = 0
		case "enter":
			m.filterActive = false
			m.loading = true
			m.filteredLists = nil
			m.playlistCursor = 0
			if m.filterInput == "" {
				return m, fetchPlaylists(m.client)
			}
			return m, searchPlaylists(m.client, m.filterInput)
		case "backspace":
			if len(m.filterInput) > 0 {
				_, size := utf8.DecodeLastRuneInString(m.filterInput)
				m.filterInput = m.filterInput[:len(m.filterInput)-size]
				m.filteredLists = filterPlaylists(m.playlists, m.filterInput)
				m.playlistCursor = 0
			} else {
				// empty input: close popup
				if m.selectMode {
					return m, tea.Quit
				}
				m.popupOpen = false
				m.filterActive = false
			}
		case "?":
			if os.Getenv("TMUX") != "" || os.Getenv("ZELLIJ") != "" {
				return m, openExternalHelp()
			}
			m.helpOpen = true
		default:
			if len(msg.Runes) > 0 {
				m.filterInput += string(msg.Runes)
				m.filteredLists = filterPlaylists(m.playlists, m.filterInput)
				m.playlistCursor = 0
			}
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "esc":
		if m.selectMode {
			return m, tea.Quit
		}
		m.popupOpen = false

	case "backspace":
		// go back to filter/search screen
		m.filterActive = true

	case "j", "down":
		if m.playlistCursor < len(m.filteredLists)-1 {
			m.playlistCursor++
		}

	case "k", "up":
		if m.playlistCursor > 0 {
			m.playlistCursor--
		}

	case "/":
		m.filterActive = true

	case "enter":
		if len(m.filteredLists) == 0 {
			break
		}
		selected := m.filteredLists[m.playlistCursor]
		m.popupOpen = false
		m.pendingURI = selected.URI
		m.devicePopupOpen = true
		m.deviceLoading = true
		m.devices = nil
		m.deviceCursor = 0
		return m, fetchDevices(m.client)
	}

	return m, nil
}

func (m model) handleDevicePopupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.devicePopupOpen = false
		m.popupOpen = true
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		if m.deviceCursor < len(m.devices)-1 {
			m.deviceCursor++
		}
	case "k", "up":
		if m.deviceCursor > 0 {
			m.deviceCursor--
		}
	case "enter":
		if len(m.devices) == 0 {
			break
		}
		selected := m.devices[m.deviceCursor]
		m.devicePopupOpen = false
		if m.selectMode {
			client := m.client
			uri := m.pendingURI
			deviceID := selected.ID
			return m, func() tea.Msg {
				_ = startPlayback(context.Background(), client, uri, deviceID)
				return tea.QuitMsg{}
			}
		}
		m.setStatus("playing on: " + selected.Name)
		return m, playCmd(m.client, m.pendingURI, selected.ID)
	}
	return m, nil
}
