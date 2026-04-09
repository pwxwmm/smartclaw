package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type VersionInfo struct {
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date"`
	DownloadURL string `json:"download_url"`
	Changelog   string `json:"changelog"`
	Mandatory   bool   `json:"mandatory"`
}

type UpdateStatus int

const (
	UpdateChecking UpdateStatus = iota
	UpdateAvailable
	UpdateNotAvailable
	UpdateDownloading
	UpdateReady
	UpdateError
)

type UpdateMsg struct {
	Status  UpdateStatus
	Version string
	Error   error
}

type AutoUpdater struct {
	currentVersion string
	latestVersion  string
	status         UpdateStatus
	checkInterval  time.Duration
	lastCheck      time.Time
	downloadURL    string
	changelog      string
	mandatory      bool
	progress       float64
	error          string
}

func NewAutoUpdater(currentVersion string) *AutoUpdater {
	return &AutoUpdater{
		currentVersion: currentVersion,
		status:         UpdateNotAvailable,
		checkInterval:  24 * time.Hour,
		lastCheck:      time.Time{},
	}
}

func (u *AutoUpdater) CheckForUpdate() tea.Cmd {
	return func() tea.Msg {
		u.status = UpdateChecking

		resp, err := http.Get("https://api.github.com/repos/instructkr/smartcode/releases/latest")
		if err != nil {
			return UpdateMsg{Status: UpdateError, Error: err}
		}
		defer resp.Body.Close()

		var release struct {
			TagName string `json:"tag_name"`
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			return UpdateMsg{Status: UpdateError, Error: err}
		}

		latestVersion := release.TagName
		if latestVersion[0] == 'v' {
			latestVersion = latestVersion[1:]
		}

		if latestVersion != u.currentVersion {
			u.latestVersion = latestVersion
			u.changelog = release.Body
			u.downloadURL = release.HTMLURL
			u.status = UpdateAvailable
			return UpdateMsg{Status: UpdateAvailable, Version: latestVersion}
		}

		u.status = UpdateNotAvailable
		u.lastCheck = time.Now()
		return UpdateMsg{Status: UpdateNotAvailable}
	}
}

func (u *AutoUpdater) ShouldCheck() bool {
	if u.status == UpdateChecking || u.status == UpdateDownloading {
		return false
	}
	return time.Since(u.lastCheck) > u.checkInterval
}

func (u *AutoUpdater) GetStatus() UpdateStatus {
	return u.status
}

func (u *AutoUpdater) GetLatestVersion() string {
	return u.latestVersion
}

func (u *AutoUpdater) GetChangelog() string {
	return u.changelog
}

func (u *AutoUpdater) GetDownloadURL() string {
	return u.downloadURL
}

func (u *AutoUpdater) IsMandatory() bool {
	return u.mandatory
}

func (u *AutoUpdater) SetCheckInterval(interval time.Duration) {
	u.checkInterval = interval
}

func (u *AutoUpdater) Render() string {
	theme := GetTheme()

	switch u.status {
	case UpdateChecking:
		return theme.InfoStyle().Render("🔄 Checking for updates...")
	case UpdateAvailable:
		msg := fmt.Sprintf("🎉 Update available! v%s → v%s", u.currentVersion, u.latestVersion)
		if u.changelog != "" {
			msg += "\n\n" + u.changelog
		}
		return theme.SuccessStyle().Render(msg)
	case UpdateNotAvailable:
		return theme.SuccessStyle().Render("✅ You're on the latest version: v" + u.currentVersion)
	case UpdateDownloading:
		bar := NewProgressBar(30, 100)
		bar.SetProgress(u.progress)
		return theme.InfoStyle().Render("⬇️  Downloading update...\n" + bar.Render())
	case UpdateReady:
		return theme.SuccessStyle().Render("✅ Update ready! Restart to apply.")
	case UpdateError:
		return theme.ErrorStyle().Render("❌ Update check failed: " + u.error)
	default:
		return ""
	}
}

type UpdateDialog struct {
	updater *AutoUpdater
	visible bool
}

func NewUpdateDialog(updater *AutoUpdater) *UpdateDialog {
	return &UpdateDialog{
		updater: updater,
		visible: false,
	}
}

func (d *UpdateDialog) Show() {
	d.visible = true
}

func (d *UpdateDialog) Hide() {
	d.visible = false
}

func (d *UpdateDialog) Render() string {
	if !d.visible {
		return ""
	}

	if d.updater.status != UpdateAvailable {
		return ""
	}

	theme := GetTheme()

	var content string
	content = theme.TitleStyle().Render("🔄 Update Available")
	content += "\n\n"
	content += fmt.Sprintf("Current version: v%s\n", d.updater.currentVersion)
	content += fmt.Sprintf("Latest version: v%s\n\n", d.updater.latestVersion)

	if d.updater.changelog != "" {
		content += theme.HelpStyle().Render("What's new:")
		content += "\n" + d.updater.changelog + "\n\n"
	}

	content += theme.HelpStyle().Render("Press 'U' to update, 'Esc' to skip")

	dialog := NewDialog("Update Available", content, DialogInfo)
	dialog.width = 60

	return dialog.View()
}
