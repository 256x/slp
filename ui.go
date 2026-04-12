package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// --- message types ---

type tickMsg time.Time
type playbackUpdatedMsg struct{ State PlaybackState }
type apiErrorMsg struct{ Err error }
type statusMsg struct{ Text string }
type devicesLoadedMsg struct {
	Devices []Device
	Err     error
}

// --- model ---

type model struct {
	width            int
	height           int
	ready            bool
	popupOpen        bool
	playlists        []Playlist
	filteredLists    []Playlist
	playlistCursor   int
	filterInput      string
	filterActive     bool
	devicePopupOpen  bool
	devices          []Device
	deviceCursor     int
	deviceLoading    bool
	pendingURI       string
	playback         PlaybackState
	statusMessage    string
	statusExpiry     time.Time
	loading          bool
	helpOpen         bool
	quitting         bool
	selectMode       bool
	client           *SpotifyClient
	debug            bool
}

// --- styles ---

var (
	stylePopupBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(0, 1)

	styleSelected = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Bold(true)

	styleTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true)

	styleFilter = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))
)

// --- init ---

func newModel(client *SpotifyClient, debug bool, selectMode bool) model {
	return model{
		client:       client,
		debug:        debug,
		selectMode:   selectMode,
		popupOpen:    selectMode,
		filterActive: selectMode,
	}
}

func (m model) Init() tea.Cmd {
	if m.selectMode {
		return nil
	}
	return tea.Batch(
		fetchPlayback(m.client),
		tick(),
	)
}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
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

func playCmd(client *SpotifyClient, uri, deviceID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := startPlayback(ctx, client, uri, deviceID); err != nil {
			return apiErrorMsg{Err: err}
		}
		time.Sleep(300 * time.Millisecond)
		state, err := client.GetCurrentPlayback(ctx)
		if err != nil {
			return statusMsg{Text: "Playback started"}
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

	case playbackUpdatedMsg:
		m.playback = msg.State
		_ = SavePlaybackCache(msg.State)

	case playlistsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.setStatus("Failed to load playlists: " + msg.Err.Error())
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
			m.setStatus("Failed to load devices: " + msg.Err.Error())
		} else {
			m.devices = msg.Devices
			m.deviceCursor = 0
			if len(m.devices) == 0 {
				m.devicePopupOpen = false
				m.setStatus("No devices available")
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
	return "Error: " + err.Error()
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func openExternalSelect(client *SpotifyClient) tea.Cmd {
	return func() tea.Msg {
		exe, err := os.Executable()
		if err != nil {
			exe = os.Args[0]
		}
		var cmd *exec.Cmd
		switch {
		case os.Getenv("TMUX") != "":
			cmd = exec.Command("tmux", "display-popup", "-E", "-w", "80%", "-h", "60%", exe, "--select")
		case os.Getenv("ZELLIJ") != "":
			cmd = exec.Command("zellij", "run", "--floating", "--close-on-exit", "--", exe, "--select")
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

func fetchDevices(client *SpotifyClient) tea.Cmd {
	return func() tea.Msg {
		devices, err := client.GetDevices(context.Background())
		return devicesLoadedMsg{Devices: devices, Err: err}
	}
}

func (m model) handleDevicePopupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
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
		m.setStatus("Starting on: " + selected.Name)
		return m, playCmd(m.client, m.pendingURI, selected.ID)
	}
	return m, nil
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

	case "p":
		if m.playback.DeviceID == "" {
			m.setStatus("No active Spotify device")
			return m, nil
		}
		var cmd tea.Cmd
		if m.playback.Playing {
			cmd = func() tea.Msg {
				if err := m.client.Pause(context.Background(), m.playback.DeviceID); err != nil {
					return apiErrorMsg{Err: err}
				}
				return playbackUpdatedMsg{State: func() PlaybackState {
					s := m.playback
					s.Playing = false
					return s
				}()}
			}
		} else {
			cmd = func() tea.Msg {
				if err := m.client.Resume(context.Background(), m.playback.DeviceID); err != nil {
					return apiErrorMsg{Err: err}
				}
				return playbackUpdatedMsg{State: func() PlaybackState {
					s := m.playback
					s.Playing = true
					return s
				}()}
			}
		}
		return m, cmd

	case "l", "right":
		if m.playback.DeviceID == "" {
			m.setStatus("No active Spotify device")
			return m, nil
		}
		return m, func() tea.Msg {
			if err := m.client.Next(context.Background(), m.playback.DeviceID); err != nil {
				return apiErrorMsg{Err: err}
			}
			time.Sleep(300 * time.Millisecond)
			s, _ := m.client.GetCurrentPlayback(context.Background())
			return playbackUpdatedMsg{State: s}
		}

	case "h", "left":
		if m.playback.DeviceID == "" {
			m.setStatus("No active Spotify device")
			return m, nil
		}
		return m, func() tea.Msg {
			if err := m.client.Previous(context.Background(), m.playback.DeviceID); err != nil {
				return apiErrorMsg{Err: err}
			}
			time.Sleep(300 * time.Millisecond)
			s, _ := m.client.GetCurrentPlayback(context.Background())
			return playbackUpdatedMsg{State: s}
		}

	case "k", "up":
		return m, m.adjustVolume(+5)

	case "j", "down":
		return m, m.adjustVolume(-5)

	case "?":
		m.helpOpen = true
		return m, nil

	case "s", "S":
		if m.playback.DeviceID == "" {
			m.setStatus("No active Spotify device")
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

	case "P":
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
		return func() tea.Msg { return statusMsg{Text: "No active Spotify device"} }
	}
	if m.playback.VolumePercent == nil {
		return func() tea.Msg { return statusMsg{Text: "Volume not supported on this device"} }
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
			if m.filterInput == "" {
				break
			}
			m.filterActive = false
			m.loading = true
			m.filteredLists = nil
			m.playlistCursor = 0
			if m.filterInput == "0" {
				return m, fetchPlaylists(m.client)
			}
			return m, searchPlaylists(m.client, m.filterInput)
		case "backspace":
			if len(m.filterInput) > 0 {
				_, size := utf8.DecodeLastRuneInString(m.filterInput)
				m.filterInput = m.filterInput[:len(m.filterInput)-size]
				m.filteredLists = filterPlaylists(m.playlists, m.filterInput)
				m.playlistCursor = 0
			}
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

// --- view ---

func (m model) View() string {
	if !m.ready {
		return ""
	}
	if m.helpOpen {
		return m.renderWithHelp()
	}
	if m.devicePopupOpen {
		return m.renderWithDevicePopup()
	}
	if m.popupOpen {
		return m.renderWithPopup()
	}
	return m.renderPlayerLine()
}

func (m model) renderWithHelp() string {
	items := []string{
		"p        Play / Pause",
		"h / ←   Previous track",
		"l / →   Next track",
		"k / ↑   Volume +5",
		"j / ↓   Volume -5",
		"s        Toggle shuffle",
		"P        Select playlist",
		"q        Quit",
	}
	popup := renderPopupBox("Keys", items, -1, "", false, "any key to close", m.width, m.height)
	return overlay(m.renderPlayerLine(), popup, m.width, m.height)
}

func (m model) renderPlayerLine() string {
	status := m.currentStatus()
	line := buildPlayerLine(m.playback, m.width, status)
	return line
}

func (m model) currentStatus() string {
	if m.statusMessage != "" && time.Now().Before(m.statusExpiry) {
		return m.statusMessage
	}
	m.statusMessage = ""
	return ""
}

func buildPlayerLine(s PlaybackState, width int, status string) string {
	if status != "" {
		return truncate(status, width)
	}
	if s.Track == "" && s.DeviceID == "" {
		return IconPause + " No active playback"
	}

	playSymbol := IconPlay
	if !s.Playing {
		playSymbol = IconPause
	}

	shuffleChar := "-"
	if s.Shuffle {
		shuffleChar = "+"
	}

	right := " " + IconShuffle + ":" + shuffleChar
	if s.VolumePercent != nil {
		right = fmt.Sprintf(" %s:%d", IconVolume, *s.VolumePercent) + right
	}

	prefix := playSymbol + " "
	prefixW := runewidth.StringWidth(prefix)
	rightW := runewidth.StringWidth(right)

	// Space for [ track @ artist ] + bar
	available := width - prefixW - rightW
	if available < 4 {
		return prefix + truncate(s.Track, available) + right
	}

	trackStr := "[ " + s.Track + " @ " + s.Artist + " ] "
	trackW := runewidth.StringWidth(trackStr)

	const minBar = 4
	if trackW > available-minBar {
		// Truncate inner text to fit, leaving room for bar
		innerMax := available - minBar - 4 // 4 = len("[ " + " ] ")
		if innerMax < 1 {
			innerMax = 1
		}
		inner := truncate(s.Track+" @ "+s.Artist, innerMax)
		trackStr = "[ " + inner + " ] "
		trackW = runewidth.StringWidth(trackStr)
	}

	barWidth := available - trackW
	if barWidth < 0 {
		barWidth = 0
	}
	bar := progressBar(s.ProgressMS, s.DurationMS, barWidth)

	return prefix + trackStr + bar + right
}

func progressBar(progressMS, durationMS, width int) string {
	if width <= 0 {
		return ""
	}
	if durationMS <= 0 {
		return strings.Repeat("-", width)
	}
	ratio := float64(progressMS) / float64(durationMS)
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}
	filled := int(float64(width) * ratio)
	return strings.Repeat("=", filled) + strings.Repeat("-", width-filled)
}

func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}

// --- popup rendering ---

func (m model) renderWithPopup() string {
	footer := "↑↓/jk select  Enter play  / search  Esc close"
	if m.loading {
		popup := renderPopupBox("Playlist Search", []string{"Loading..."}, -1, m.filterInput, m.filterActive, footer, m.width, m.height)
		return overlay(m.base(), popup, m.width, m.height)
	}

	items := make([]string, len(m.filteredLists))
	for i, p := range m.filteredLists {
		items[i] = p.Name
	}
	if len(items) == 0 {
		if m.filterActive {
			items = []string{"0: own playlists  /  other: Spotify search"}
		} else {
			items = []string{"(no results)"}
		}
	}

	popup := renderPopupBox("Playlist Search", items, m.playlistCursor, m.filterInput, m.filterActive, footer, m.width, m.height)
	return overlay(m.base(), popup, m.width, m.height)
}

func (m model) renderWithDevicePopup() string {
	footer := "↑↓/jk select  Enter play  Esc back"
	if m.deviceLoading {
		popup := renderPopupBox("Select Device", []string{"Loading..."}, -1, "", false, footer, m.width, m.height)
		return overlay(m.base(), popup, m.width, m.height)
	}
	items := make([]string, len(m.devices))
	for i, d := range m.devices {
		label := d.Name + " [" + d.Type + "]"
		if d.IsActive {
			label += " *"
		}
		items[i] = label
	}
	if len(items) == 0 {
		items = []string{"(no devices)"}
	}
	popup := renderPopupBox("Select Device", items, m.deviceCursor, "", false, footer, m.width, m.height)
	return overlay(m.base(), popup, m.width, m.height)
}

func (m model) base() string {
	if m.selectMode {
		return ""
	}
	return m.renderPlayerLine()
}

func renderPopupBox(title string, items []string, cursor int, filter string, filterActive bool, footer string, termW, termH int) string {
	maxW := termW - 4
	if maxW > 60 {
		maxW = 60
	}
	if maxW < 20 {
		maxW = 20
	}

	// How many rows can we show?
	maxRows := termH - 6
	if maxRows < 3 {
		maxRows = 3
	}
	if maxRows > 15 {
		maxRows = 15
	}

	// Scroll window
	start := 0
	if cursor >= maxRows {
		start = cursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(items) {
		end = len(items)
	}

	innerW := maxW - 4 // account for border+padding

	var sb strings.Builder

	// Title line
	titleLine := styleTitle.Render(title)
	sb.WriteString(titleLine)
	sb.WriteString("\n")

	// Filter line
	if filterActive || filter != "" {
		indicator := "/"
		if filterActive {
			indicator = styleFilter.Render("/")
		}
		sb.WriteString(indicator + filter)
		if filterActive {
			sb.WriteString("█")
		}
		sb.WriteString("\n")
	}

	// Item rows
	for i := start; i < end; i++ {
		row := truncate(items[i], innerW)
		// Pad to innerW
		rowRunes := utf8.RuneCountInString(row)
		if rowRunes < innerW {
			row += strings.Repeat(" ", innerW-rowRunes)
		}
		if i == cursor {
			sb.WriteString(styleSelected.Render(row))
		} else {
			sb.WriteString(row)
		}
		sb.WriteString("\n")
	}

	// Footer hint
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render(footer))

	content := sb.String()
	boxed := stylePopupBorder.Width(maxW).Render(content)
	return boxed
}

// overlay centers the popup over the base line.
func overlay(base, popup string, termW, termH int) string {
	popupLines := strings.Split(popup, "\n")
	popupH := len(popupLines)

	// Find max popup width
	popupW := 0
	for _, l := range popupLines {
		w := utf8.RuneCountInString(l)
		if w > popupW {
			popupW = w
		}
	}

	// Center position
	startRow := (termH - popupH) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (termW - popupW) / 2
	if startCol < 0 {
		startCol = 0
	}

	// Build output: blank lines above, then popup, then player line at bottom
	var sb strings.Builder

	// Move cursor to top, clear screen approach via newlines
	// Bubble Tea manages the screen; just render the full frame.
	// We emit enough lines to fill the terminal height.
	blank := strings.Repeat(" ", termW)

	for row := 0; row < termH-1; row++ {
		if row >= startRow && row < startRow+popupH {
			lineIdx := row - startRow
			line := popupLines[lineIdx]
			lineW := utf8.RuneCountInString(line)
			leftPad := startCol
			rightPad := termW - startCol - lineW
			if rightPad < 0 {
				rightPad = 0
			}
			sb.WriteString(strings.Repeat(" ", leftPad))
			sb.WriteString(line)
			sb.WriteString(strings.Repeat(" ", rightPad))
		} else {
			sb.WriteString(blank)
		}
		if row < termH-2 {
			sb.WriteString("\n")
		}
	}

	// Last line: player
	sb.WriteString("\n")
	sb.WriteString(base)

	return sb.String()
}
