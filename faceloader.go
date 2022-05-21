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
	"log"
	"os"
	"path"
	"strings"
	"time"
)

func main() {
	a := app.NewWithID("studio.gfsc.faceloader")
	w := a.NewWindow("FaceLoader")

	txtFacebookPages := widget.NewMultiLineEntry()
	txtFacebookPages.SetText(a.Preferences().String("FacebookPages"))

	btnSetStoragePath := widget.NewButton("Set storage", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			a.Preferences().SetString("Storage", uri.Path())
			log.Println(uri.Path())
		}, w)
	})

	//ShowFolderOpen

	txtOutput := widget.NewMultiLineEntry()

	lblStatus := widget.NewLabel("")

	form := &widget.Form{OnSubmit: func() {
		lblStatus.SetText("Loading...")
		a.Preferences().SetString("FacebookPages", txtFacebookPages.Text)

		cal := ics.NewCalendar()
		cal.SetMethod(ics.MethodRequest)

		pages := strings.Split(txtFacebookPages.Text, "\n")
		for _, page := range pages {

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
				lblStatus.SetText(fmt.Sprintf("Loading... (%v events)", len(cal.Events())))
			}

		}
		txtOutput.SetText(cal.Serialize())
		directory := a.Preferences().String("Storage")
		if directory != "" {
			f, err := os.OpenFile(path.Join(directory, "events.ics"), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
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

	grid := container.New(layout.NewVBoxLayout(), form, txtOutput, lblStatus)

	w.SetContent(grid)

	w.Resize(fyne.NewSize(600, 600))
	w.CenterOnScreen()
	w.ShowAndRun()
}
