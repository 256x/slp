package main

import (
	"context"
	"fmt"
	"math"
	"os"

	colorful "github.com/lucasb-eyer/go-colorful"
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
	vizTick          int
}

// --- styles ---

var (
	stylePopupBorder lipgloss.Style
	styleSelected    lipgloss.Style
	styleTitle       lipgloss.Style
	styleFilter      lipgloss.Style
	styleAccent      lipgloss.Style
	styleDim         lipgloss.Style

	accentColor    colorful.Color
	hasAccentColor bool
)

func initStyles(t resolvedTheme) {
	stylePopupBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Accent)).
		Padding(0, 1)
	styleSelected = lipgloss.NewStyle().
		Background(lipgloss.Color(t.Accent)).
		Foreground(lipgloss.Color(t.SelectedFg)).
		Bold(true)
	styleTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Accent)).
		Bold(true)
	styleFilter = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.FilterFg))
	styleAccent = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Accent))
	styleDim = lipgloss.NewStyle().
		Faint(true)

	hasAccentColor = false
	if c, err := colorful.Hex(t.Accent); err == nil {
		accentColor = c
		hasAccentColor = true
	}
}

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
		vizTick(),
	)
}

var cfg Config

type vizTickMsg struct{}

func vizTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return vizTickMsg{}
	})
}

func tick() tea.Cmd {
	d := time.Duration(cfg.UI.TickInterval) * time.Second
	return tea.Tick(d, func(t time.Time) tea.Msg {
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
		m.vizTick++
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
		m.setStatus("starting on: " + selected.Name)
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
			m.setStatus("no active spotify device")
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
			m.setStatus("no active spotify device")
			return m, nil
		}
		return m, skipCmd(m.client, m.playback.DeviceID, m.client.Next)

	case "h", "left":
		if m.playback.DeviceID == "" {
			m.setStatus("no active spotify device")
			return m, nil
		}
		return m, skipCmd(m.client, m.playback.DeviceID, m.client.Previous)

	case "k", "up":
		return m, m.adjustVolume(+5)

	case "j", "down":
		return m, m.adjustVolume(-5)

	case "?":
		m.helpOpen = true
		return m, nil

	case "s", "S":
		if m.playback.DeviceID == "" {
			m.setStatus("no active spotify device")
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
		return func() tea.Msg { return statusMsg{Text: "no active spotify device"} }
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
	case "q", "esc", "backspace":
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
		"p    play / pause",
		"h/←  previous track",
		"l/→  next track",
		"k/↑  volume +5",
		"j/↓  volume -5",
		"s    toggle shuffle",
		"P    select playlist",
		"bs   go back / close",
		"q    quit",
		"",
		"any key to close",
	}
	popup := renderPopupBox("keys", items, nil, -1, -1, "", false, m.width, m.height)
	return overlay(m.renderPlayerLine(), popup, m.width, m.height)
}

func (m model) renderPlayerLine() string {
	status := m.currentStatus()
	return buildPlayerLine(m.playback, m.width, status, m.vizTick)
}

func (m model) currentStatus() string {
	if m.statusMessage != "" && time.Now().Before(m.statusExpiry) {
		return m.statusMessage
	}
	m.statusMessage = ""
	return ""
}

func buildPlayerLine(s PlaybackState, width int, status string, vizTick int) string {
	if status != "" {
		return truncate(status, width)
	}
	if s.Track == "" && s.DeviceID == "" {
		return cfg.Icons.Pause + " waiting for playlist..."
	}

	playSymbol := styleAccent.Render(cfg.Icons.Play)
	if !s.Playing {
		playSymbol = styleAccent.Render(cfg.Icons.Pause)
	}

	shuffleChar := "-"
	if s.Shuffle {
		shuffleChar = "+"
	}

	right := styleDim.Render(" "+cfg.Icons.Shuffle+" :"+shuffleChar)
	if s.VolumePercent != nil {
		right = styleDim.Render(fmt.Sprintf(" %s :%d", cfg.Icons.Volume, *s.VolumePercent)) + right
	}

	prefix := playSymbol + " "
	prefixW := lipgloss.Width(prefix)
	rightW := lipgloss.Width(right)

	available := width - prefixW - rightW
	if available < 4 {
		return prefix + truncate(s.Track, available) + right
	}

	trackStr := "[ " + s.Track + " @ " + s.Artist + " ] "
	trackW := runewidth.StringWidth(trackStr)

	const minBar = 4
	if trackW > available-minBar {
		innerMax := available - minBar - 4
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
	bar := progressBar(s.ProgressMS, s.DurationMS, barWidth, vizTick)

	return prefix + trackStr + bar + right
}

func progressBar(progressMS, durationMS, width, tick int) string {
	if width <= 0 {
		return ""
	}
	if durationMS <= 0 {
		return styleDim.Render(strings.Repeat("-", width))
	}
	ratio := float64(progressMS) / float64(durationMS)
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}
	filled := int(float64(width) * ratio)

	var sb strings.Builder
	if hasAccentColor {
		h, c, l := accentColor.Hcl()
		dark := colorful.Hcl(h, c*0.4, l*0.25).Clamped()
		offset := float64(tick) * 0.06
		for i := range width {
			// sine wave sweeping left to right
			t := math.Sin(float64(i)/float64(width)*math.Pi*2-offset)*0.5 + 0.5
			col := dark.BlendHcl(accentColor, t).Clamped()
			st := lipgloss.NewStyle().Foreground(lipgloss.Color(col.Hex()))
			if i < filled {
				sb.WriteString(st.Render("="))
			} else {
				sb.WriteString(st.Faint(true).Render("-"))
			}
		}
	} else {
		sb.WriteString(styleAccent.Render(strings.Repeat("=", filled)))
		sb.WriteString(styleDim.Render(strings.Repeat("-", width-filled)))
	}
	return sb.String()
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
	if m.loading {
		popup := renderPopupBox("playlist search", []string{"loading..."}, nil, -1, -1, m.filterInput, m.filterActive, m.width, m.height)
		return overlay(m.base(), popup, m.width, m.height)
	}

	items := make([]string, len(m.filteredLists))
	rightLabels := make([]string, len(m.filteredLists))
	for i, p := range m.filteredLists {
		items[i] = p.Name
		if p.TrackCount > 0 {
			rightLabels[i] = fmt.Sprintf("%d", p.TrackCount)
		}
	}
	if len(items) == 0 {
		if m.filterActive {
			items = []string{"[enter]:own playlists  [words]:search"}
		} else {
			items = []string{"(no results)"}
		}
		rightLabels = nil
	}

	popup := renderPopupBox("slp", items, rightLabels, m.playlistCursor, len(m.filteredLists), m.filterInput, m.filterActive, m.width, m.height)
	return overlay(m.base(), popup, m.width, m.height)
}

func (m model) renderWithDevicePopup() string {
	if m.deviceLoading {
		popup := renderPopupBox("select device", []string{"loading..."}, nil, -1, -1, "", false, m.width, m.height)
		return overlay(m.base(), popup, m.width, m.height)
	}
	items := make([]string, len(m.devices))
	rightLabels := make([]string, len(m.devices))
	for i, d := range m.devices {
		items[i] = d.Name
		rl := strings.ToLower(d.Type)
		if d.IsActive {
			rl += " ·"
		}
		rightLabels[i] = rl
	}
	if len(items) == 0 {
		items = []string{"(no devices)"}
		rightLabels = nil
	}
	popup := renderPopupBox("select device", items, rightLabels, m.deviceCursor, len(m.devices), "", false, m.width, m.height)
	return overlay(m.base(), popup, m.width, m.height)
}

func (m model) base() string {
	if m.selectMode {
		return ""
	}
	return m.renderPlayerLine()
}

// renderPopupBox renders a popup. cursor=-1 means no selection. total=-1 suppresses scroll indicator.
func renderPopupBox(title string, items, rightLabels []string, cursor, total int, filter string, filterActive bool, termW, termH int) string {
	maxW := termW - 4
	if maxW > 60 {
		maxW = 60
	}
	if maxW < 20 {
		maxW = 20
	}

	maxRows := termH - 6
	if maxRows < 3 {
		maxRows = 3
	}
	if maxRows > 15 {
		maxRows = 15
	}

	start := 0
	if cursor >= maxRows {
		start = cursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(items) {
		end = len(items)
	}

	innerW := maxW - 4 // border + padding
	faint := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder

	// Title + scroll indicator
	scrollStr := ""
	if total > 1 && cursor >= 0 {
		scrollStr = fmt.Sprintf("%d/%d", cursor+1, total)
	}
	titleMax := innerW
	if scrollStr != "" {
		titleMax = innerW - runewidth.StringWidth(scrollStr) - 1
	}
	titleRendered := styleTitle.Render(runewidth.Truncate(title, titleMax, ""))
	if scrollStr != "" {
		titleW := lipgloss.Width(titleRendered)
		pad := innerW - titleW - runewidth.StringWidth(scrollStr)
		if pad < 1 {
			pad = 1
		}
		sb.WriteString(titleRendered + strings.Repeat(" ", pad) + faint.Render(scrollStr))
	} else {
		sb.WriteString(titleRendered)
	}
	sb.WriteString("\n")

	// Filter line
	if filterActive || filter != "" {
		indicator := "❯ "
		if filterActive {
			indicator = styleFilter.Render("❯ ")
		}
		f := runewidth.Truncate(filter, innerW-2, "…")
		sb.WriteString(indicator + f)
		if filterActive {
			sb.WriteString("█")
		}
		sb.WriteString("\n")
	}

	// Separator
	sb.WriteString(faint.Render(strings.Repeat("─", innerW)))
	sb.WriteString("\n")

	// Compute max right-label width
	rightW := 0
	if rightLabels != nil {
		for _, r := range rightLabels {
			if w := runewidth.StringWidth(r); w > rightW {
				rightW = w
			}
		}
		if rightW > 0 {
			rightW++ // +1 for gap
		}
	}
	const prefixW = 2
	textW := innerW - prefixW - rightW

	// Item rows
	for i := start; i < end; i++ {
		rl := ""
		if rightLabels != nil && i < len(rightLabels) {
			rl = rightLabels[i]
		}
		text := items[i]
		if textW > 0 {
			text = runewidth.Truncate(text, textW, "…")
			text = runewidth.FillRight(text, textW)
		}
		if i == cursor {
			full := "❯ " + text
			if rightW > 0 {
				full += " " + runewidth.FillRight(rl, rightW-1)
			}
			sb.WriteString(styleSelected.Render(full))
		} else {
			sb.WriteString("  " + text)
			if rightW > 0 {
				sb.WriteString(" " + faint.Render(runewidth.FillRight(rl, rightW-1)))
			}
		}
		sb.WriteString("\n")
	}

	content := sb.String()
	// trim trailing newline so lipgloss border fits snugly
	content = strings.TrimRight(content, "\n")
	return stylePopupBorder.Width(maxW).Render(content)
}

// overlay centers the popup over the base line.
func overlay(base, popup string, termW, termH int) string {
	popupLines := strings.Split(popup, "\n")
	popupH := len(popupLines)

	// Find max popup width (strip ANSI codes for accurate measurement)
	popupW := 0
	for _, l := range popupLines {
		w := lipgloss.Width(l)
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
			lineW := lipgloss.Width(line)
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
