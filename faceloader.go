package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	ics "github.com/arran4/golang-ical"
	faceloader "github.com/geeksforsocialchange/faceloader/parser"
	"github.com/go-co-op/gocron"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

var cal ics.Calendar

func update(a fyne.App) ics.Calendar {
	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodRequest)

	directory := a.Preferences().String("Storage")

	pages := strings.Split(a.Preferences().String("FacebookPages"), "\n")
	for _, page := range pages {
		pageCal := ics.NewCalendar()
		pageCal.SetMethod(ics.MethodRequest)

		eventLinks, err := faceloader.GetFacebookEventLinks(page)
		if err != nil {
			log.Println(err)
		}
		for _, eventLink := range eventLinks {
			i, err := faceloader.InterfaceFromMbasic(eventLink)
			if err != nil {
				log.Println(err)
			}
			event, err := faceloader.InterfaceToIcal(i)
			if err != nil {
				log.Println(err)
			}
			cal.Components = append(cal.Components, &event)
			pageCal.Components = append(pageCal.Components, &event)
			if directory != "" {
				f, err := os.OpenFile(path.Join(directory, fmt.Sprintf("%v.ics", page)), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
				if err != nil {
					log.Println(err)
				}
				err = cal.SerializeTo(f)
				if err != nil {
					log.Println(err)
				}
				f.Sync()
				f.Close()
			}
		}
	}
	return *cal
}

func main() {
	a := app.NewWithID("studio.gfsc.faceloader")
	w := a.NewWindow("FaceLoader")

	s := gocron.NewScheduler(time.UTC)
	s.Every(1).Hours().Do(func() {
		update(a)
	})

	s.StartAsync()

	txtFacebookPages := widget.NewMultiLineEntry()
	txtFacebookPages.SetText(a.Preferences().String("FacebookPages"))

	btnSetStoragePath := widget.NewButton("Set storage", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri == nil {
				return
			}
			a.Preferences().SetString("Storage", uri.Path())
			log.Println(uri.Path())
		}, w)
	})

	events := []string{}
	list := widget.NewList(
		func() int {
			return len(events)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(id widget.ListItemID, object fyne.CanvasObject) {
			object.(*widget.Label).SetText(events[id])
		})

	list.OnSelected = func(id widget.ListItemID) {
		event := cal.Events()[id]
		dialog.ShowInformation(event.GetProperty(ics.ComponentPropertySummary).Value, event.Serialize(), w)
	}

	lblStatus := widget.NewLabel("")

	form := &widget.Form{OnSubmit: func() {
		lblStatus.SetText("Loading...")
		a.Preferences().SetString("FacebookPages", txtFacebookPages.Text)

		cal = update(a)

		for _, event := range cal.Events() {
			events = append(events, event.GetProperty(ics.ComponentPropertySummary).Value)
		}

		futureEvents := 0
		for _, event := range cal.Events() {
			start, _ := event.GetStartAt()
			if start.After(time.Now()) {
				futureEvents += 1
			}
		}
		lblStatus.SetText(fmt.Sprintf("Loaded %v events, of which %v are in the future", len(cal.Events()), futureEvents))

	}}

	form.Append("Facebook Pages:", txtFacebookPages)
	form.Append("", btnSetStoragePath)

	grid := container.New(layout.NewBorderLayout(form, lblStatus, nil, nil), form, lblStatus, list)

	w.SetContent(grid)

	w.Resize(fyne.NewSize(600, 600))
	w.CenterOnScreen()
	w.ShowAndRun()
}
