package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/RushikeshIche/ResolverX/internal/app"
	"github.com/RushikeshIche/ResolverX/internal/redirects"
)

// RedirectsView provides CRUD management for custom URL shortcuts and redirects.
type RedirectsView struct {
	app           *app.App
	window        fyne.Window
	allRedirects  []*redirects.Redirect
	filtered      []*redirects.Redirect
	searchEntry   *widget.Entry
	redirectsList *widget.List
}

// NewRedirectsView creates a new view for URL shortcuts.
func NewRedirectsView(a *app.App, w fyne.Window) *RedirectsView {
	return &RedirectsView{
		app:          a,
		window:       w,
		allRedirects: make([]*redirects.Redirect, 0),
		filtered:     make([]*redirects.Redirect, 0),
	}
}

// Build creates the Fyne canvas object for the URL Redirects view.
func (rv *RedirectsView) Build() fyne.CanvasObject {
	rv.searchEntry = widget.NewEntry()
	rv.searchEntry.SetPlaceHolder("Search shortcuts or destination URLs...")
	rv.searchEntry.OnChanged = func(query string) {
		rv.filterRedirects(query)
	}

	addBtn := widget.NewButton("Add Redirect", rv.showAddDialog)
	addBtn.Importance = widget.HighImportance

	refreshBtn := widget.NewButton("Refresh", func() {
		rv.Reload()
	})

	topBar := container.NewBorder(
		nil, nil, nil, container.NewHBox(refreshBtn, addBtn),
		rv.searchEntry,
	)

	banner := widget.NewLabel(fmt.Sprintf("HTTP Redirect Server active on http://%s  — Enter shortcut as domain or path (e.g. gitrush.test or /gitrush)", rv.app.Config.HTTPRedirectAddress))

	rv.redirectsList = widget.NewList(
		func() int {
			return len(rv.filtered)
		},
		func() fyne.CanvasObject {
			shortcutLbl := widget.NewLabelWithStyle("shortcut.test", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			codeLbl := widget.NewLabel("[302]")
			targetLbl := widget.NewLabel("https://destination.url")
			descLbl := widget.NewLabel("Description")

			editBtn := widget.NewButton("Edit", nil)
			delBtn := widget.NewButton("Delete", nil)
			delBtn.Importance = widget.DangerImportance

			return container.NewBorder(
				nil, nil,
				container.NewHBox(shortcutLbl, codeLbl),
				container.NewHBox(descLbl, editBtn, delBtn),
				targetLbl,
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(rv.filtered) {
				return
			}
			redir := rv.filtered[id]
			root := obj.(*fyne.Container)

			leftBox := root.Objects[1].(*fyne.Container)
			shortcutLbl := leftBox.Objects[0].(*widget.Label)
			shortcutLbl.SetText(redir.Shortcut)

			codeLbl := leftBox.Objects[1].(*widget.Label)
			codeLbl.SetText(fmt.Sprintf("[%d]", redir.StatusCode))

			targetLbl := root.Objects[0].(*widget.Label)
			targetLbl.SetText(redir.TargetURL)

			rightBox := root.Objects[2].(*fyne.Container)
			descLbl := rightBox.Objects[0].(*widget.Label)
			descLbl.SetText(redir.Description)

			editBtn := rightBox.Objects[1].(*widget.Button)
			editBtn.OnTapped = func() {
				rv.showEditDialog(redir)
			}

			delBtn := rightBox.Objects[2].(*widget.Button)
			delBtn.OnTapped = func() {
				rv.confirmDeleteDialog(redir)
			}
		},
	)

	rv.Reload()

	return container.NewBorder(
		container.NewVBox(banner, topBar), nil, nil, nil,
		rv.redirectsList,
	)
}

// Reload reloads the list of redirects from storage.
func (rv *RedirectsView) Reload() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	list, err := rv.app.Redirects.List(ctx)
	if err != nil {
		dialog.ShowError(err, rv.window)
		return
	}

	rv.allRedirects = list
	rv.filterRedirects(rv.searchEntry.Text)
}

func (rv *RedirectsView) filterRedirects(query string) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		rv.filtered = rv.allRedirects
	} else {
		filtered := make([]*redirects.Redirect, 0)
		for _, r := range rv.allRedirects {
			if strings.Contains(strings.ToLower(r.Shortcut), q) || strings.Contains(strings.ToLower(r.TargetURL), q) || strings.Contains(strings.ToLower(r.Description), q) {
				filtered = append(filtered, r)
			}
		}
		rv.filtered = filtered
	}
	if rv.redirectsList != nil {
		rv.redirectsList.Refresh()
	}
}

func (rv *RedirectsView) showAddDialog() {
	shortcutEntry := widget.NewEntry()
	shortcutEntry.SetPlaceHolder("e.g. gitrush.test or myprofile")

	targetEntry := widget.NewEntry()
	targetEntry.SetPlaceHolder("https://github.com/RushikeshIche")

	codeSelect := widget.NewSelect([]string{"302 (Temporary)", "301 (Permanent)"}, nil)
	codeSelect.SetSelected("302 (Temporary)")

	descEntry := widget.NewEntry()
	descEntry.SetPlaceHolder("e.g. My GitHub Profile")

	items := []*widget.FormItem{
		{Text: "Shortcut / Domain", Widget: shortcutEntry},
		{Text: "Destination URL", Widget: targetEntry},
		{Text: "Redirect Code", Widget: codeSelect},
		{Text: "Description", Widget: descEntry},
	}

	formDialog := dialog.NewForm("Create URL Redirect Shortcut", "Save", "Cancel", items, func(confirmed bool) {
		if !confirmed {
			return
		}

		code := 302
		if strings.HasPrefix(codeSelect.Selected, "301") {
			code = 301
		}

		newRedir := &redirects.Redirect{
			Shortcut:    shortcutEntry.Text,
			TargetURL:   targetEntry.Text,
			StatusCode:  code,
			Description: descEntry.Text,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := rv.app.Redirects.Create(ctx, newRedir); err != nil {
			dialog.ShowError(err, rv.window)
			return
		}

		rv.Reload()
	}, rv.window)

	formDialog.Resize(fyne.NewSize(500, 320))
	formDialog.Show()
}

func (rv *RedirectsView) showEditDialog(redir *redirects.Redirect) {
	shortcutEntry := widget.NewEntry()
	shortcutEntry.SetText(redir.Shortcut)

	targetEntry := widget.NewEntry()
	targetEntry.SetText(redir.TargetURL)

	codeSelect := widget.NewSelect([]string{"302 (Temporary)", "301 (Permanent)"}, nil)
	if redir.StatusCode == 301 {
		codeSelect.SetSelected("301 (Permanent)")
	} else {
		codeSelect.SetSelected("302 (Temporary)")
	}

	descEntry := widget.NewEntry()
	descEntry.SetText(redir.Description)

	items := []*widget.FormItem{
		{Text: "Shortcut / Domain (Modifiable)", Widget: shortcutEntry},
		{Text: "Destination URL", Widget: targetEntry},
		{Text: "Redirect Code", Widget: codeSelect},
		{Text: "Description", Widget: descEntry},
	}

	formDialog := dialog.NewForm("Edit URL Shortcut & Redirect", "Update", "Cancel", items, func(confirmed bool) {
		if !confirmed {
			return
		}

		code := 302
		if strings.HasPrefix(codeSelect.Selected, "301") {
			code = 301
		}

		redir.Shortcut = shortcutEntry.Text
		redir.TargetURL = targetEntry.Text
		redir.StatusCode = code
		redir.Description = descEntry.Text

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := rv.app.Redirects.Update(ctx, redir); err != nil {
			dialog.ShowError(err, rv.window)
			return
		}

		rv.Reload()
	}, rv.window)

	formDialog.Resize(fyne.NewSize(500, 320))
	formDialog.Show()
}

func (rv *RedirectsView) confirmDeleteDialog(redir *redirects.Redirect) {
	msg := fmt.Sprintf("Are you sure you want to delete redirect shortcut '%s' (redirects to %s)?", redir.Shortcut, redir.TargetURL)
	dialog.ShowConfirm("Confirm Deletion", msg, func(confirmed bool) {
		if !confirmed {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := rv.app.Redirects.Delete(ctx, redir.ID); err != nil {
			dialog.ShowError(err, rv.window)
			return
		}

		rv.Reload()
	}, rv.window)
}
