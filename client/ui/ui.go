package ui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"cloudsave/api"
	"cloudsave/archive"
	"cloudsave/config"
	"cloudsave/discover"
)

// defaultPort is the CloudSave server port used when scanning the LAN.
const defaultPort = 45231

type App struct {
	fyneApp fyne.App
	win     fyne.Window
	cfg     *config.Config
	client  *api.Client

	gameSelect      *widget.Select
	gameIDs         []string          // parallel slice to gameSelect.Options
	serverGameNames map[string]string // game id -> name, as known to the server

	localPathLabel     *widget.Label
	localModLabel      *widget.Label
	serverSavesSelect  *widget.Select
	serverSaves        []api.Save // parallel to serverSavesSelect.Options, newest first
	serverMachineLabel *widget.Label
	serverSavedLabel   *widget.Label
	serverTimeLabel    *widget.Label
	serverSizeLabel    *widget.Label
	statusLabel        *widget.Label

	pushBtn *widget.Button
	pullBtn *widget.Button
}

func Run() {
	a := app.New()
	w := a.NewWindow("CloudSave")
	w.Resize(fyne.NewSize(560, 440))

	u := &App{fyneApp: a, win: w}

	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{Games: make(map[string]config.GameEntry)}
	}
	u.cfg = cfg
	u.serverGameNames = make(map[string]string)
	u.client = api.New(cfg.ServerURL, cfg.APIKey)

	w.SetContent(u.build())
	u.refreshList()  // show local games immediately
	u.refreshGames() // then merge in games registered on the server
	w.ShowAndRun()
}

func (u *App) build() fyne.CanvasObject {
	u.gameSelect = widget.NewSelect(nil, u.onSelect)
	u.gameSelect.PlaceHolder = "Select a game..."

	addBtn := widget.NewButtonWithIcon("Add Game", theme.ContentAddIcon(), u.showAddDialog)
	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), u.showSettings)

	topBar := container.NewBorder(
		nil, nil, nil,
		container.NewHBox(settingsBtn),
		container.NewBorder(nil, nil, nil, addBtn, u.gameSelect),
	)

	// Local section
	u.localPathLabel = widget.NewLabel("—")
	u.localPathLabel.Truncation = fyne.TextTruncateEllipsis
	u.localModLabel = widget.NewLabel("—")
	setLocationBtn := widget.NewButtonWithIcon("Set Location…", theme.FolderOpenIcon(), u.onSetLocation)

	localCard := widget.NewCard("Local", "",
		container.NewVBox(
			labelRow("Path:", u.localPathLabel),
			labelRow("Modified:", u.localModLabel),
			setLocationBtn,
		),
	)

	// Server section
	u.serverSavesSelect = widget.NewSelect(nil, u.onServerSaveSelected)
	u.serverSavesSelect.PlaceHolder = "No saves on server"
	u.serverMachineLabel = widget.NewLabel("—")
	u.serverSavedLabel = widget.NewLabel("—")
	u.serverTimeLabel = widget.NewLabel("—")
	u.serverSizeLabel = widget.NewLabel("—")

	serverCard := widget.NewCard("Server", "",
		container.NewVBox(
			u.serverSavesSelect,
			labelRow("Machine:", u.serverMachineLabel),
			labelRow("Saved:", u.serverSavedLabel),
			labelRow("Uploaded:", u.serverTimeLabel),
			labelRow("Size:", u.serverSizeLabel),
		),
	)

	u.statusLabel = widget.NewLabel("")
	u.statusLabel.Wrapping = fyne.TextWrapWord
	statusCopyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		u.win.Clipboard().SetContent(u.statusLabel.Text)
	})
	statusRow := container.NewBorder(nil, nil, nil, statusCopyBtn, u.statusLabel)

	u.pushBtn = widget.NewButtonWithIcon("Push to Server", theme.UploadIcon(), u.onPush)
	u.pullBtn = widget.NewButtonWithIcon("Pull from Server", theme.DownloadIcon(), u.onPull)
	u.pushBtn.Disable()
	u.pullBtn.Disable()

	actionRow := container.NewHBox(layout.NewSpacer(), u.pushBtn, u.pullBtn, layout.NewSpacer())

	return container.NewVBox(
		topBar,
		widget.NewSeparator(),
		localCard,
		serverCard,
		statusRow,
		widget.NewSeparator(),
		actionRow,
	)
}

// labelRow builds a two-label row used in the info cards. A Border layout
// pins the bold heading to the left and lets the value fill the remaining
// width — this gives the value label a real width constraint so long text
// (e.g. a save path) truncates with an ellipsis instead of collapsing the
// whole window into a single column of characters.
func labelRow(heading string, value *widget.Label) *fyne.Container {
	h := widget.NewLabelWithStyle(heading, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return container.NewBorder(nil, nil, h, nil, value)
}

// showError displays an error in a dialog whose message text is selectable
// and copyable, with a one-click Copy button — much faster than retyping an
// error when troubleshooting.
func (u *App) showError(err error, w fyne.Window) {
	msg := err.Error()
	body := widget.NewLabel(msg)
	body.Wrapping = fyne.TextWrapWord
	copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(msg)
	})
	d := dialog.NewCustom("Error", "Close",
		container.NewBorder(nil, copyBtn, nil, nil, body), w)
	d.Resize(fyne.NewSize(460, 240))
	d.Show()
}

// refreshGames fetches the games registered on the server and merges them into
// the dropdown, so a client can see (and set up) games it doesn't have locally.
func (u *App) refreshGames() {
	go func() {
		games, err := u.client.ListGames()
		if err != nil {
			return // server unreachable — local games are still listed
		}
		m := make(map[string]string, len(games))
		for _, g := range games {
			m[g.ID] = g.Name
		}
		u.serverGameNames = m
		u.refreshList()
	}()
}

func (u *App) refreshList() {
	type pair struct {
		id, name string
		local    bool
	}
	var pairs []pair
	seen := map[string]bool{}
	for id, g := range u.cfg.Games {
		pairs = append(pairs, pair{id, g.Name, true})
		seen[id] = true
	}
	for id, name := range u.serverGameNames {
		if !seen[id] {
			pairs = append(pairs, pair{id, name, false})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].name < pairs[j].name })

	u.gameIDs = make([]string, len(pairs))
	names := make([]string, len(pairs))
	for i, p := range pairs {
		u.gameIDs[i] = p.id
		names[i] = p.name
		if !p.local {
			names[i] = p.name + "  (on server — no local copy)"
		}
	}
	u.gameSelect.Options = names
	u.gameSelect.Refresh()
}

// gameName returns the display name for a game id, whether it's local or only
// known from the server.
func (u *App) gameName(id string) string {
	if g, ok := u.cfg.Games[id]; ok {
		return g.Name
	}
	if n, ok := u.serverGameNames[id]; ok {
		return n
	}
	return id
}

// selectGameByID selects the dropdown entry for a game id (its label may carry
// a suffix, so match by id rather than name).
func (u *App) selectGameByID(id string) {
	for i, gid := range u.gameIDs {
		if gid == id {
			u.gameSelect.SetSelected(u.gameSelect.Options[i])
			return
		}
	}
}

// onSetLocation maps (or re-points) the selected game's local save folder. This
// is how a game that only exists on the server gets a local save location.
func (u *App) onSetLocation() {
	id, ok := u.selectedID()
	if !ok {
		u.showError(fmt.Errorf("select a game first"), u.win)
		return
	}
	name := u.gameName(id)
	dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
		if err != nil || lu == nil {
			return
		}
		u.cfg.Games[id] = config.GameEntry{Name: name, LocalPath: lu.Path()}
		if err := u.cfg.Save(); err != nil {
			u.showError(fmt.Errorf("could not save config: %w", err), u.win)
			return
		}
		u.refreshList()
		// Force a fresh onSelect even if the label text is unchanged.
		u.gameSelect.ClearSelected()
		u.selectGameByID(id)
	}, u.win)
}

func (u *App) selectedID() (string, bool) {
	for i, name := range u.gameSelect.Options {
		if name == u.gameSelect.Selected {
			if i < len(u.gameIDs) {
				return u.gameIDs[i], true
			}
		}
	}
	return "", false
}

func (u *App) onSelect(name string) {
	id, ok := u.selectedID()
	if !ok {
		return
	}
	entry, hasLocal := u.cfg.Games[id]

	// Reset UI state
	if hasLocal {
		u.localPathLabel.SetText(entry.LocalPath)
	} else {
		u.localPathLabel.SetText("(no local location set — click \"Set Location…\")")
	}
	u.localModLabel.SetText("checking...")
	u.serverMachineLabel.SetText("checking...")
	u.serverSavedLabel.SetText("—")
	u.serverTimeLabel.SetText("—")
	u.serverSizeLabel.SetText("—")
	u.statusLabel.SetText("")
	u.pushBtn.Disable()
	u.pullBtn.Disable()

	go func() {
		var localMod time.Time
		localExists := false
		if hasLocal {
			localMod, localExists = latestModTime(entry.LocalPath)
		}
		if localExists {
			u.localModLabel.SetText(localMod.Local().Format("2006-01-02 15:04:05"))
		} else if hasLocal {
			u.localModLabel.SetText("(not found locally)")
		} else {
			u.localModLabel.SetText("—")
		}

		saves, err := u.client.ListSaves(id)
		if err != nil {
			u.serverMachineLabel.SetText("—")
			u.statusLabel.SetText("Could not reach server: " + err.Error())
			return
		}

		if len(saves) == 0 {
			u.serverSaves = nil
			u.serverSavesSelect.Options = nil
			u.serverSavesSelect.ClearSelected()
			u.serverSavesSelect.Refresh()
			u.serverMachineLabel.SetText("—")
			u.serverSavedLabel.SetText("No saves yet")
			u.serverTimeLabel.SetText("—")
			u.serverSizeLabel.SetText("—")
			u.statusLabel.SetText("No saves on server — push to create the first one.")
			if localExists {
				u.pushBtn.Enable()
			}
			return
		}

		// Populate the history dropdown (newest first). Selecting an entry
		// drives the detail labels and is what Pull will fetch.
		u.serverSaves = saves
		opts := make([]string, len(saves))
		for i, s := range saves {
			opts[i] = serverSaveLabel(s)
		}
		u.serverSavesSelect.Options = opts
		u.serverSavesSelect.Refresh()
		u.serverSavesSelect.SetSelected(opts[0])

		latest := saves[0]
		serverSaved := latest.SavedAt
		if serverSaved.IsZero() {
			serverSaved = latest.UploadedAt
		}

		// Compare the save *content* times, not the upload time. A small
		// tolerance absorbs sub-second drift from the upload round-trip so an
		// unchanged save doesn't read as "newer" by a fraction of a second.
		const slop = 2 * time.Second
		switch diff := localMod.Sub(serverSaved); {
		case !hasLocal:
			u.statusLabel.SetText("This game is on the server but not set up here — click \"Set Location…\" to choose where saves go, then pull.")
		case !localExists:
			u.statusLabel.SetText("Local save not found — pull to restore.")
			u.pullBtn.Enable()
		case diff > slop:
			u.statusLabel.SetText("Local is newer than server — push to update it.")
			u.pushBtn.Enable()
			u.pullBtn.Enable()
		case diff < -slop:
			u.statusLabel.SetText("Server is newer than local — pull to update.")
			u.pushBtn.Enable()
			u.pullBtn.Enable()
		default:
			u.statusLabel.SetText("In sync.")
			u.pushBtn.Enable()
			u.pullBtn.Enable()
		}
	}()
}

// latestModTime returns the most recent modification time of a save. For a
// directory it returns the newest mtime among all files within (the directory's
// own mtime doesn't always change when a nested file is written), which is the
// best proxy for "when this save was last played".
func latestModTime(path string) (time.Time, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	if !info.IsDir() {
		return info.ModTime(), true
	}
	latest := info.ModTime()
	_ = filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() && fi.ModTime().After(latest) {
			latest = fi.ModTime()
		}
		return nil
	})
	return latest, true
}

// saveContentTime returns the save's content modification time, falling back to
// the upload time for old saves recorded before saved_at existed.
func saveContentTime(s api.Save) time.Time {
	if s.SavedAt.IsZero() {
		return s.UploadedAt
	}
	return s.SavedAt
}

// serverSaveLabel formats a save for the history dropdown: "<saved time> — <machine>".
func serverSaveLabel(s api.Save) string {
	return fmt.Sprintf("%s — %s",
		saveContentTime(s).Local().Format("2006-01-02 15:04:05"), s.MachineName)
}

// selectedServerSave returns the save currently chosen in the history dropdown.
func (u *App) selectedServerSave() *api.Save {
	for i, opt := range u.serverSavesSelect.Options {
		if opt == u.serverSavesSelect.Selected && i < len(u.serverSaves) {
			return &u.serverSaves[i]
		}
	}
	return nil
}

func (u *App) onServerSaveSelected(string) {
	s := u.selectedServerSave()
	if s == nil {
		return
	}
	u.serverMachineLabel.SetText(s.MachineName)
	u.serverSavedLabel.SetText(saveContentTime(*s).Local().Format("2006-01-02 15:04:05"))
	u.serverTimeLabel.SetText(s.UploadedAt.Local().Format("2006-01-02 15:04:05"))
	u.serverSizeLabel.SetText(formatBytes(s.FileSize))
}

func (u *App) onPush() {
	id, ok := u.selectedID()
	if !ok {
		return
	}
	entry := u.cfg.Games[id]

	prog := dialog.NewProgressInfinite("Pushing", "Compressing and uploading...", u.win)
	prog.Show()

	go func() {
		defer prog.Hide()

		// Ensure the game is registered (idempotent).
		if err := u.client.RegisterGame(id, entry.Name); err != nil {
			u.showError(fmt.Errorf("server registration failed: %w", err), u.win)
			return
		}

		savedAt, ok := latestModTime(entry.LocalPath)
		if !ok {
			u.showError(fmt.Errorf("local save not found at %s", entry.LocalPath), u.win)
			return
		}

		var buf bytes.Buffer
		if err := archive.Pack(entry.LocalPath, &buf); err != nil {
			u.showError(fmt.Errorf("could not zip save: %w", err), u.win)
			return
		}
		if err := u.client.UploadSave(id, u.cfg.MachineName, savedAt, &buf); err != nil {
			u.showError(fmt.Errorf("upload failed: %w", err), u.win)
			return
		}
		dialog.ShowInformation("Done", "Save pushed to server.", u.win)
		u.onSelect(u.gameSelect.Selected)
	}()
}

func (u *App) onPull() {
	id, ok := u.selectedID()
	if !ok {
		return
	}
	entry := u.cfg.Games[id]

	save := u.selectedServerSave()
	if save == nil {
		u.showError(fmt.Errorf("no server save selected to pull"), u.win)
		return
	}
	saveID := save.ID

	dialog.ShowConfirm(
		"Pull Save",
		fmt.Sprintf("Overwrite your local files with the save from %s (%s)?",
			save.MachineName, saveContentTime(*save).Local().Format("2006-01-02 15:04:05")),
		func(confirmed bool) {
			if !confirmed {
				return
			}
			prog := dialog.NewProgressInfinite("Pulling", "Downloading save...", u.win)
			prog.Show()

			go func() {
				defer prog.Hide()

				rc, err := u.client.DownloadSave(id, saveID)
				if err != nil {
					u.showError(err, u.win)
					return
				}
				defer rc.Close()

				// Stream to temp file so zip.NewReader gets a ReaderAt + known size.
				tmp, err := os.CreateTemp("", "cloudsave-*.zip")
				if err != nil {
					u.showError(err, u.win)
					return
				}
				defer tmp.Close()
				defer os.Remove(tmp.Name())

				size, err := io.Copy(tmp, rc)
				if err != nil {
					u.showError(err, u.win)
					return
				}

				dest := filepath.Dir(entry.LocalPath)
				if err := archive.Unpack(tmp, size, dest); err != nil {
					u.showError(fmt.Errorf("could not extract save: %w", err), u.win)
					return
				}
				dialog.ShowInformation("Done", "Save pulled from server.", u.win)
				u.onSelect(u.gameSelect.Selected)
			}()
		},
		u.win,
	)
}

func (u *App) showAddDialog() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("e.g. Stardew Valley")

	pathEntry := widget.NewEntry()
	pathEntry.SetPlaceHolder("Local save folder or file")

	browseBtn := widget.NewButton("Browse...", func() {
		dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil || lu == nil {
				return
			}
			pathEntry.SetText(lu.Path())
		}, u.win)
	})

	pathRow := container.NewBorder(nil, nil, nil, browseBtn, pathEntry)

	d := dialog.NewForm("Add Game", "Add", "Cancel",
		[]*widget.FormItem{
			{Text: "Game Name", Widget: nameEntry},
			{Text: "Save Path", Widget: pathRow},
		},
		func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(nameEntry.Text)
			path := strings.TrimSpace(pathEntry.Text)
			if name == "" || path == "" {
				u.showError(fmt.Errorf("both fields are required"), u.win)
				return
			}
			id := toSlug(name)

			// Persist locally first so the game is saved even if the server
			// is unreachable. Server registration is best-effort and also
			// happens (idempotently) on the first push.
			u.cfg.Games[id] = config.GameEntry{Name: name, LocalPath: path}
			if err := u.cfg.Save(); err != nil {
				u.showError(fmt.Errorf("could not save config: %w", err), u.win)
				return
			}
			u.refreshList()
			u.gameSelect.SetSelected(name)

			go func() {
				if err := u.client.RegisterGame(id, name); err != nil {
					u.showError(fmt.Errorf(
						"game saved locally, but could not register with the server "+
							"(it will retry on push):\n%w", err), u.win)
				}
			}()
		},
		u.win,
	)
	d.Resize(fyne.NewSize(500, 220))
	d.Show()
}

func (u *App) showSettings() {
	urlEntry := widget.NewEntry()
	urlEntry.SetText(u.cfg.ServerURL)
	scanBtn := widget.NewButtonWithIcon("Scan", theme.SearchIcon(), func() {
		u.scanForServers(urlEntry)
	})
	urlRow := container.NewBorder(nil, nil, nil, scanBtn, urlEntry)

	keyEntry := widget.NewPasswordEntry()
	keyEntry.SetText(u.cfg.APIKey)

	machineEntry := widget.NewEntry()
	machineEntry.SetText(u.cfg.MachineName)

	d := dialog.NewForm("Settings", "Save", "Cancel",
		[]*widget.FormItem{
			{Text: "Server URL", Widget: urlRow},
			{Text: "API Key", Widget: keyEntry},
			{Text: "Machine Name", Widget: machineEntry},
		},
		func(ok bool) {
			if !ok {
				return
			}
			u.cfg.ServerURL = config.NormalizeServerURL(urlEntry.Text)
			u.cfg.APIKey = keyEntry.Text
			u.cfg.MachineName = strings.TrimSpace(machineEntry.Text)
			u.client = api.New(u.cfg.ServerURL, u.cfg.APIKey)
			if err := u.cfg.Save(); err != nil {
				u.showError(fmt.Errorf("could not save config: %w", err), u.win)
			}
		},
		u.win,
	)
	d.Resize(fyne.NewSize(440, 250))
	d.Show()
}

// scanForServers probes the local network for CloudSave servers and fills the
// given entry with the result (or lets the user pick if several are found).
func (u *App) scanForServers(target *widget.Entry) {
	prog := dialog.NewProgressInfinite("Scanning",
		fmt.Sprintf("Looking for CloudSave servers on your network (port %d)...", defaultPort), u.win)
	prog.Show()

	go func() {
		servers := discover.Servers(defaultPort, 500*time.Millisecond)
		prog.Hide()

		switch len(servers) {
		case 0:
			dialog.ShowInformation("Scan complete",
				fmt.Sprintf("No CloudSave servers found on port %d.\nYou can still enter the address manually.", defaultPort), u.win)
		case 1:
			target.SetText(servers[0])
			dialog.ShowInformation("Server found", "Using "+servers[0], u.win)
		default:
			sel := widget.NewSelect(servers, nil)
			sel.SetSelected(servers[0])
			dialog.ShowCustomConfirm("Multiple servers found", "Use", "Cancel", sel,
				func(ok bool) {
					if ok && sel.Selected != "" {
						target.SetText(sel.Selected)
					}
				}, u.win)
		}
	}()
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func toSlug(s string) string {
	s = strings.ToLower(s)
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
