package main

import (
	_ "embed"
	"fmt"
	ics "github.com/arran4/golang-ical"
	"github.com/chromedp/chromedp"
	"github.com/geeksforsocialchange/faceloader/parser"
	"github.com/spf13/viper"
	"log"
	"net/url"
)

func main() {
	// read config from config.yaml.  We can improve this and make a nice ui to edit in the future
	c := viper.New()
	c.SetConfigName(".faceloader")
	c.AddConfigPath(".")
	c.AddConfigPath("$HOME")
	c.AutomaticEnv()
	_ = c.ReadInConfig()

	c.SetDefault("ChromePath", "/opt/google/chrome/chrome")
	c.SetDefault("Debug", false)

	// build a new calendar
	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodRequest)

	_, browserContext := faceloader.BrowserContext(c.GetString("ChromePath"), c.GetBool("Debug"))
	err := faceloader.MaybeLogin(browserContext, c.GetString("Username"), c.GetString("Password"))
	if err != nil {
		log.Println(err)
	}

	log.Println(c.GetString("FacebookPage"))

	// add events to the calendar
	events := faceloader.GetFacebookEventLinks(browserContext, c.GetString("FacebookPage"), c.GetBool("Debug"))
	for _, event := range events {
		u, _ := url.Parse(event)
		u.Scheme = "https"
		u.Host = "www.facebook.com"
		calEvent, err := faceloader.Fb2ical(u.String())
		if err != nil {
			log.Printf("%s %s\n", u.String(), err)
		} else {
			cal.Components = append(cal.Components, &calEvent)
		}
	}

	// @TODO write to a file instead of stdout
	fmt.Print(cal.Serialize())

	// Manually cancel the context to gracefully close the browser
	err = chromedp.Cancel(browserContext)
	if err != nil {
		log.Fatalf("error canceling browserContext: %s\n", err)
	}
}
