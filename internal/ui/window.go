package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	coreapp "github.com/RushikeshIche/ResolverX/internal/app"
)

// MainWindow represents the top-level Fyne GUI window for ResolverX.
type MainWindow struct {
	fyneApp   fyne.App
	window    fyne.Window
	coreApp   *coreapp.App
	dashboard *DashboardView
	records   *RecordsView
	redirects *RedirectsView
	settings  *SettingsView
}

// NewMainWindow creates and sets up the primary application window.
func NewMainWindow(core *coreapp.App) *MainWindow {
	a := app.NewWithID("com.resolverx.desktop")
	w := a.NewWindow("ResolverX — Local DNS Resolver & Manager")

	dash := NewDashboardView(core, w)
	recs := NewRecordsView(core, w)
	redirs := NewRedirectsView(core, w)
	settings := NewSettingsView(core, w)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Dashboard", theme.HomeIcon(), dash.Build()),
		container.NewTabItemWithIcon("DNS Records", theme.ListIcon(), recs.Build()),
		container.NewTabItemWithIcon("URL Redirects", theme.ComputerIcon(), redirs.Build()),
		container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), settings.Build()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	w.SetContent(tabs)
	w.Resize(fyne.NewSize(980, 700))

	// Ensure graceful cleanup on window close
	w.SetOnClosed(func() {
		_ = core.Close()
	})

	return &MainWindow{
		fyneApp:   a,
		window:    w,
		coreApp:   core,
		dashboard: dash,
		records:   recs,
		redirects: redirs,
		settings:  settings,
	}
}

// ShowAndRun displays the window and runs the Fyne event loop.
func (mw *MainWindow) ShowAndRun() {
	mw.window.ShowAndRun()
}
