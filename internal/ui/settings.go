package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/rushi/ResolverX/internal/app"
	"github.com/rushi/ResolverX/internal/config"
)

// SettingsView provides configuration management for listener and upstreams.
type SettingsView struct {
	app    *app.App
	window fyne.Window
}

// NewSettingsView creates a new settings view.
func NewSettingsView(a *app.App, w fyne.Window) *SettingsView {
	return &SettingsView{
		app:    a,
		window: w,
	}
}

// Build creates the Fyne canvas object for application settings.
func (sv *SettingsView) Build() fyne.CanvasObject {
	cfg := sv.app.Config

	listenEntry := widget.NewEntry()
	listenEntry.SetText(cfg.ListenAddress)

	upstreamsEntry := widget.NewMultiLineEntry()
	upstreamsEntry.SetText(strings.Join(cfg.UpstreamServers, "\n"))
	upstreamsEntry.SetMinRowsVisible(3)

	cacheCheck := widget.NewCheck("Enable Response Caching", nil)
	cacheCheck.SetChecked(cfg.CacheEnabled)

	cacheMaxEntry := widget.NewEntry()
	cacheMaxEntry.SetText(strconv.Itoa(cfg.CacheMaxEntries))

	logCheck := widget.NewCheck("Log Incoming DNS Queries", nil)
	logCheck.SetChecked(cfg.LogQueries)

	saveBtn := widget.NewButton("Save Configuration", func() {
		rawUpstreams := strings.Split(upstreamsEntry.Text, "\n")
		var cleanUpstreams []string
		for _, s := range rawUpstreams {
			trimmed := strings.TrimSpace(s)
			if trimmed != "" {
				cleanUpstreams = append(cleanUpstreams, trimmed)
			}
		}

		maxEntries, err := strconv.Atoi(cacheMaxEntry.Text)
		if err != nil || maxEntries <= 0 {
			dialog.ShowError(fmt.Errorf("invalid cache max entries number: %s", cacheMaxEntry.Text), sv.window)
			return
		}

		newCfg := &config.Config{
			ListenAddress:   strings.TrimSpace(listenEntry.Text),
			UpstreamServers: cleanUpstreams,
			CacheEnabled:    cacheCheck.Checked,
			CacheMaxEntries: maxEntries,
			LogQueries:      logCheck.Checked,
			DatabasePath:    cfg.DatabasePath,
		}

		if err := newCfg.Validate(); err != nil {
			dialog.ShowError(err, sv.window)
			return
		}

		if err := sv.app.UpdateConfig(newCfg); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), sv.window)
			return
		}

		dialog.ShowInformation("Configuration Saved", "Settings updated successfully. If listen address was changed, please restart the DNS resolver.", sv.window)
	})
	saveBtn.Importance = widget.HighImportance

	form := widget.NewForm(
		widget.NewFormItem("DNS Listener Address", listenEntry),
		widget.NewFormItem("Upstream DNS Servers (one per line)", upstreamsEntry),
		widget.NewFormItem("Caching", cacheCheck),
		widget.NewFormItem("Max Cache Entries", cacheMaxEntry),
		widget.NewFormItem("Logging", logCheck),
	)

	card := widget.NewCard("Resolver Settings", "Configure listener sockets, upstreams, and caching", container.NewVBox(
		form,
		container.NewCenter(saveBtn),
	))

	return container.NewVBox(card)
}
