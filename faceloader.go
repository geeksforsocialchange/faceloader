package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	ics "github.com/arran4/golang-ical"
	"github.com/geeksforsocialchange/faceloader/parser"
	"log"
	"net/url"
	"strings"
	"time"
)

func main() {
	a := app.NewWithID("studio.gfsc.faceloader")
	w := a.NewWindow("FaceLoader")

	txtFacebookPages := widget.NewMultiLineEntry()
	txtFacebookPages.SetText(a.Preferences().String("FacebookPages"))
	txtChromePath := widget.NewEntry()
	txtChromePath.SetText(a.Preferences().String("ChromePath"))
	txtUsername := widget.NewEntry()
	txtUsername.SetText(a.Preferences().String("Username"))
	txtPassword := widget.NewPasswordEntry()
	txtPassword.SetText(a.Preferences().String("Password"))
	boolDebug := widget.NewCheck("", func(value bool) { log.Println("Debug set to ", value) })

	txtOutput := widget.NewMultiLineEntry()

	lblStatus := widget.NewLabel("")

	form := &widget.Form{OnSubmit: func() {
		lblStatus.SetText("Loading...")
		a.Preferences().SetString("FacebookPages", txtFacebookPages.Text)
		a.Preferences().SetString("ChromePath", txtChromePath.Text)
		a.Preferences().SetString("Username", txtUsername.Text)
		a.Preferences().SetString("Password", txtPassword.Text)

		_, ctx := faceloader.BrowserContext(txtChromePath.Text, boolDebug.Checked)

		err := faceloader.MaybeLogin(ctx, txtUsername.Text, txtPassword.Text)
		if err != nil {
			log.Println(err)
		}

		cal := ics.NewCalendar()
		cal.SetMethod(ics.MethodRequest)

		pages := strings.Split(txtFacebookPages.Text, "\n")
		for _, page := range pages {
			events := faceloader.GetFacebookEventLinks(ctx, page, boolDebug.Checked)
			for _, event := range events {
				u, _ := url.Parse(event)
				u.Scheme = "https"
				u.Host = "www.facebook.com"
				calEvent, err := faceloader.Fb2ical(u.String())
				if err != nil {
					log.Printf("%s %s\n", u.String(), err)
				} else {
					cal.Components = append(cal.Components, &calEvent)
					lblStatus.SetText(fmt.Sprintf("Loading... (%v events)", len(cal.Events())))
				}
			}
		}
		txtOutput.SetText(cal.Serialize())

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
	form.Append("Chrome path:", txtChromePath)
	form.Append("Username:", txtUsername)
	form.Append("Password:", txtPassword)
	form.Append("Debug", boolDebug)

	grid := container.New(layout.NewVBoxLayout(), form, txtOutput, lblStatus)

	w.SetContent(grid)

	w.Resize(fyne.NewSize(600, 600))
	w.CenterOnScreen()
	w.ShowAndRun()
}
