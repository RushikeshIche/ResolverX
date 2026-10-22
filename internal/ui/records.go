package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/RushikeshIche/ResolverX/internal/app"
	"github.com/RushikeshIche/ResolverX/internal/records"
)

// RecordsView provides CRUD operations for custom DNS records.
type RecordsView struct {
	app         *app.App
	window      fyne.Window
	allRecords  []*records.Record
	filtered    []*records.Record
	searchEntry *widget.Entry
	recordsList *widget.List
}

// NewRecordsView creates the records management view.
func NewRecordsView(a *app.App, w fyne.Window) *RecordsView {
	return &RecordsView{
		app:        a,
		window:     w,
		allRecords: make([]*records.Record, 0),
		filtered:   make([]*records.Record, 0),
	}
}

// Build creates the Fyne canvas object for Records management.
func (rv *RecordsView) Build() fyne.CanvasObject {
	rv.searchEntry = widget.NewEntry()
	rv.searchEntry.SetPlaceHolder("Search records by domain or IP...")
	rv.searchEntry.OnChanged = func(query string) {
		rv.filterRecords(query)
	}

	addBtn := widget.NewButton("Add Record", rv.showAddDialog)
	addBtn.Importance = widget.HighImportance

	refreshBtn := widget.NewButton("Refresh", func() {
		rv.Reload()
	})

	topBar := container.NewBorder(
		nil, nil, nil, container.NewHBox(refreshBtn, addBtn),
		rv.searchEntry,
	)

	rv.recordsList = widget.NewList(
		func() int {
			return len(rv.filtered)
		},
		func() fyne.CanvasObject {
			domainLbl := widget.NewLabelWithStyle("domain.test", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			typeLbl := widget.NewLabel("[A]")
			valLbl := widget.NewLabel("127.0.0.1")
			ttlLbl := widget.NewLabel("TTL: 300s")
			editBtn := widget.NewButton("Edit", nil)
			delBtn := widget.NewButton("Delete", nil)
			delBtn.Importance = widget.DangerImportance

			return container.NewBorder(
				nil, nil,
				container.NewHBox(domainLbl, typeLbl),
				container.NewHBox(ttlLbl, editBtn, delBtn),
				valLbl,
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(rv.filtered) {
				return
			}
			rec := rv.filtered[id]
			root := obj.(*fyne.Container)

			leftBox := root.Objects[1].(*fyne.Container)
			domainLbl := leftBox.Objects[0].(*widget.Label)
			domainLbl.SetText(rec.Domain)

			typeLbl := leftBox.Objects[1].(*widget.Label)
			typeLbl.SetText(fmt.Sprintf("[%s]", rec.Type))

			valLbl := root.Objects[0].(*widget.Label)
			valLbl.SetText(rec.Value)

			rightBox := root.Objects[2].(*fyne.Container)
			ttlLbl := rightBox.Objects[0].(*widget.Label)
			ttlLbl.SetText(fmt.Sprintf("TTL: %ds", rec.TTL))

			editBtn := rightBox.Objects[1].(*widget.Button)
			editBtn.OnTapped = func() {
				rv.showEditDialog(rec)
			}

			delBtn := rightBox.Objects[2].(*widget.Button)
			delBtn.OnTapped = func() {
				rv.confirmDeleteDialog(rec)
			}
		},
	)

	rv.Reload()

	return container.NewBorder(
		topBar, nil, nil, nil,
		rv.recordsList,
	)
}

// Reload fetches all records from the database and updates the view.
func (rv *RecordsView) Reload() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	list, err := rv.app.Records.List(ctx)
	if err != nil {
		dialog.ShowError(err, rv.window)
		return
	}

	rv.allRecords = list
	rv.filterRecords(rv.searchEntry.Text)
}

func (rv *RecordsView) filterRecords(query string) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		rv.filtered = rv.allRecords
	} else {
		filtered := make([]*records.Record, 0)
		for _, r := range rv.allRecords {
			if strings.Contains(strings.ToLower(r.Domain), q) || strings.Contains(r.Value, q) {
				filtered = append(filtered, r)
			}
		}
		rv.filtered = filtered
	}
	if rv.recordsList != nil {
		rv.recordsList.Refresh()
	}
}

func (rv *RecordsView) showAddDialog() {
	domainEntry := widget.NewEntry()
	domainEntry.SetPlaceHolder("myapp.test")

	typeSelect := widget.NewSelect([]string{"A", "AAAA"}, nil)
	typeSelect.SetSelected("A")

	valueEntry := widget.NewEntry()
	valueEntry.SetPlaceHolder("127.0.0.1")

	ttlEntry := widget.NewEntry()
	ttlEntry.SetText("300")

	items := []*widget.FormItem{
		{Text: "Domain Name", Widget: domainEntry},
		{Text: "Record Type", Widget: typeSelect},
		{Text: "IP Address", Widget: valueEntry},
		{Text: "TTL (seconds)", Widget: ttlEntry},
	}

	formDialog := dialog.NewForm("Add Custom DNS Record", "Create", "Cancel", items, func(confirmed bool) {
		if !confirmed {
			return
		}

		ttl, err := strconv.ParseUint(ttlEntry.Text, 10, 32)
		if err != nil {
			dialog.ShowError(fmt.Errorf("invalid TTL number: %w", err), rv.window)
			return
		}

		newRec := &records.Record{
			Domain: domainEntry.Text,
			Type:   records.RecordType(typeSelect.Selected),
			Value:  valueEntry.Text,
			TTL:    uint32(ttl),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := rv.app.Records.Create(ctx, newRec); err != nil {
			dialog.ShowError(err, rv.window)
			return
		}

		rv.Reload()
	}, rv.window)

	formDialog.Resize(fyne.NewSize(450, 300))
	formDialog.Show()
}

func (rv *RecordsView) showEditDialog(rec *records.Record) {
	domainEntry := widget.NewEntry()
	domainEntry.SetText(rec.Domain)

	typeSelect := widget.NewSelect([]string{"A", "AAAA"}, nil)
	typeSelect.SetSelected(string(rec.Type))

	valueEntry := widget.NewEntry()
	valueEntry.SetText(rec.Value)

	ttlEntry := widget.NewEntry()
	ttlEntry.SetText(strconv.FormatUint(uint64(rec.TTL), 10))

	items := []*widget.FormItem{
		{Text: "Domain Name", Widget: domainEntry},
		{Text: "Record Type", Widget: typeSelect},
		{Text: "IP Address", Widget: valueEntry},
		{Text: "TTL (seconds)", Widget: ttlEntry},
	}

	formDialog := dialog.NewForm("Edit Custom DNS Record", "Save", "Cancel", items, func(confirmed bool) {
		if !confirmed {
			return
		}

		ttl, err := strconv.ParseUint(ttlEntry.Text, 10, 32)
		if err != nil {
			dialog.ShowError(fmt.Errorf("invalid TTL: %w", err), rv.window)
			return
		}

		rec.Domain = domainEntry.Text
		rec.Type = records.RecordType(typeSelect.Selected)
		rec.Value = valueEntry.Text
		rec.TTL = uint32(ttl)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := rv.app.Records.Update(ctx, rec); err != nil {
			dialog.ShowError(err, rv.window)
			return
		}

		rv.Reload()
	}, rv.window)

	formDialog.Resize(fyne.NewSize(450, 300))
	formDialog.Show()
}

func (rv *RecordsView) confirmDeleteDialog(rec *records.Record) {
	msg := fmt.Sprintf("Are you sure you want to delete custom record for %s (%s -> %s)?", rec.Domain, rec.Type, rec.Value)
	dialog.ShowConfirm("Confirm Deletion", msg, func(confirmed bool) {
		if !confirmed {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := rv.app.Records.Delete(ctx, rec.ID); err != nil {
			dialog.ShowError(err, rv.window)
			return
		}

		rv.Reload()
	}, rv.window)
}
