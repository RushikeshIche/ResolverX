package ui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/RushikeshIche/ResolverX/internal/app"
	"github.com/RushikeshIche/ResolverX/internal/metrics"
)

// DashboardView provides the resolver status controls, live metric counters, and query logs.
type DashboardView struct {
	app         *app.App
	window      fyne.Window
	statusLabel *widget.Label
	toggleBtn   *widget.Button
	totalLabel  *widget.Label
	localLabel  *widget.Label
	cacheLabel  *widget.Label
	upstLabel   *widget.Label
	ratioLabel  *widget.Label
	logList     *widget.List
	logs        []metrics.QueryLogEntry
}

// NewDashboardView creates the dashboard tab content.
func NewDashboardView(a *app.App, w fyne.Window) *DashboardView {
	dv := &DashboardView{
		app:    a,
		window: w,
		logs:   make([]metrics.QueryLogEntry, 0),
	}
	return dv
}

// Build creates the Fyne canvas object for the Dashboard.
func (dv *DashboardView) Build() fyne.CanvasObject {
	// 1. Status & Control Header
	dv.statusLabel = widget.NewLabelWithStyle("Status: Stopped", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	dv.toggleBtn = widget.NewButton("Start Resolver", dv.onToggleServer)

	serverControlBox := container.NewHBox(
		dv.statusLabel,
		dv.toggleBtn,
	)

	controlCard := widget.NewCard("DNS Resolver Control", "", container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Configured Listener: %s", dv.app.Config.ListenAddress)),
		serverControlBox,
	))

	// 2. Metric Counters
	dv.totalLabel = widget.NewLabelWithStyle("0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	dv.localLabel = widget.NewLabelWithStyle("0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	dv.cacheLabel = widget.NewLabelWithStyle("0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	dv.upstLabel = widget.NewLabelWithStyle("0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	dv.ratioLabel = widget.NewLabelWithStyle("0.0%", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	makeMetricCard := func(title string, valLabel *widget.Label) *widget.Card {
		return widget.NewCard(title, "", container.NewCenter(valLabel))
	}

	metricsGrid := container.NewGridWithColumns(5,
		makeMetricCard("Total Queries", dv.totalLabel),
		makeMetricCard("Local Hits", dv.localLabel),
		makeMetricCard("Cache Hits", dv.cacheLabel),
		makeMetricCard("Upstream", dv.upstLabel),
		makeMetricCard("Hit Ratio", dv.ratioLabel),
	)

	// 3. Live Query Log
	dv.logList = widget.NewList(
		func() int {
			return len(dv.logs)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				canvas.NewText("00:00:00", color.Gray{Y: 128}),
				widget.NewLabelWithStyle("DOMAIN.TEST", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabel("[A]"),
				widget.NewLabel("source: local"),
				widget.NewLabel("status: NOERROR"),
				widget.NewLabel("(1.2ms)"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(dv.logs) {
				return
			}
			entry := dv.logs[len(dv.logs)-1-id] // reverse chronological
			box := obj.(*fyne.Container)

			timeText := box.Objects[0].(*canvas.Text)
			timeText.Text = entry.Timestamp.Format("15:04:05")
			timeText.Refresh()

			domainLabel := box.Objects[1].(*widget.Label)
			domainLabel.SetText(entry.Domain)

			typeLabel := box.Objects[2].(*widget.Label)
			typeLabel.SetText(fmt.Sprintf("[%s]", entry.QType))

			sourceLabel := box.Objects[3].(*widget.Label)
			sourceLabel.SetText(fmt.Sprintf("via %s", entry.Source))

			codeLabel := box.Objects[4].(*widget.Label)
			codeLabel.SetText(entry.RCode)

			durLabel := box.Objects[5].(*widget.Label)
			durLabel.SetText(fmt.Sprintf("(%s)", entry.Duration.Round(100*time.Microsecond)))
		},
	)

	queryLogCard := widget.NewCard("Live DNS Query Log", "Real-time stream of incoming queries", container.NewBorder(
		nil, nil, nil, nil,
		dv.logList,
	))

	// Listen for live query logs from the collector
	dv.app.Metrics.OnLogEntry(func(entry metrics.QueryLogEntry) {
		dv.Refresh()
	})

	dv.updateStatusDisplay()

	// Periodic refresh ticker for stats
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			dv.Refresh()
		}
	}()

	return container.NewBorder(
		container.NewVBox(controlCard, metricsGrid),
		nil, nil, nil,
		queryLogCard,
	)
}

func (dv *DashboardView) onToggleServer() {
	if dv.app.IsServerRunning() {
		if err := dv.app.StopServer(); err != nil {
			dialog.ShowError(err, dv.window)
		}
	} else {
		if err := dv.app.StartServer(); err != nil {
			dialog.ShowError(err, dv.window)
		}
	}
	dv.updateStatusDisplay()
}

func (dv *DashboardView) updateStatusDisplay() {
	if dv.app.IsServerRunning() {
		dv.statusLabel.SetText(fmt.Sprintf("Status: Running on %s", dv.app.Server.Addr()))
		dv.toggleBtn.SetText("Stop Resolver")
		dv.toggleBtn.Importance = widget.DangerImportance
	} else {
		dv.statusLabel.SetText("Status: Stopped")
		dv.toggleBtn.SetText("Start Resolver")
		dv.toggleBtn.Importance = widget.HighImportance
	}
	dv.toggleBtn.Refresh()
}

// Refresh reloads metrics counters and recent query logs safely on the Fyne UI thread.
func (dv *DashboardView) Refresh() {
	stats := dv.app.Metrics.GetStats()
	logs := dv.app.Metrics.GetRecentLogs()

	fyne.Do(func() {
		dv.totalLabel.SetText(fmt.Sprintf("%d", stats.TotalQueries))
		dv.localLabel.SetText(fmt.Sprintf("%d", stats.LocalHits))
		dv.cacheLabel.SetText(fmt.Sprintf("%d", stats.CacheHits))
		dv.upstLabel.SetText(fmt.Sprintf("%d", stats.UpstreamQueries))
		dv.ratioLabel.SetText(fmt.Sprintf("%.1f%%", stats.HitRatio))

		dv.logs = logs
		if dv.logList != nil {
			dv.logList.Refresh()
		}
		dv.updateStatusDisplay()
	})
}

