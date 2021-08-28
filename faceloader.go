package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/araddon/dateparse"
	ics "github.com/arran4/golang-ical"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/daetal-us/getld/extract"
	"github.com/spf13/viper"
	"log"
	"net/url"
	"regexp"
	"time"
)

// The key parts that a Facebook json+ld event includes
type eventScheme struct {
	Context             string `json:"@context"`
	Type                string `json:"@type"`
	Description         string `json:"description"`
	EndDate             string `json:"endDate"`
	EventAttendanceMode string `json:"eventAttendanceMode"`
	Image               string `json:"image"`
	Location            struct {
		Type    string `json:"@type"`
		Address struct {
			Type            string `json:"@type"`
			AddressCountry  string `json:"addressCountry"`
			AddressLocality string `json:"addressLocality"`
			PostalCode      string `json:"postalCode"`
			StreetAddress   string `json:"streetAddress"`
		} `json:"address"`
		Name string `json:"name"`
	} `json:"location"`
	Name       string        `json:"name"`
	Performers []interface{} `json:"performers"`
	StartDate  string        `json:"startDate"`
	Url        string        `json:"url"`
}

// take a Facebook event url and return an ICS event
// `extract.FromURL()` does the fetching, but we may want to use Chrome in the future
func fb2ical(url string) (ics.VEvent, error) {
	results, _ := extract.FromURL(url)
	encoded, _ := json.Marshal(results)

	var events []eventScheme
	json.Unmarshal(encoded, &events)
	if len(events) == 0 {
		return ics.VEvent{}, errors.New("no events found")
	}
	var event = events[0]

	var icsEvent ics.VEvent

	icsEvent.SetDescription(event.Description)
	icsEvent.SetSummary(event.Name)
	//@todo join the non-null location parts
	icsEvent.SetLocation(fmt.Sprintf("%s, %s, %s, %s, %s",
		event.Location.Name,
		event.Location.Address.StreetAddress,
		event.Location.Address.AddressLocality,
		event.Location.Address.PostalCode,
		event.Location.Address.AddressCountry,
	))
	icsEvent.SetURL(event.Url)

	// build the event UID from the numeric ID of the event from the URL
	r, _ := regexp.Compile("\\d+")
	icsEvent.SetProperty(ics.ComponentPropertyUniqueId, r.FindString(event.Url))

	startTime, _ := dateparse.ParseAny(event.StartDate)
	icsEvent.SetStartAt(startTime)

	endTime, _ := dateparse.ParseAny(event.EndDate)
	icsEvent.SetEndAt(endTime)
	icsEvent.SetDtStampTime(time.Now())

	return icsEvent, nil
}

// de-duplicate a slice of strings
func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

// find links to Facebook events from a url, using Chrome so that we do it as a logged-in Facebook user
func getFacebookEventLinks(pageUrl string, chrome string, profileDirectory string) []string {
	var links []string

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		// if user-data-dir is set, chrome won't load the default profile,
		// even if it's set to the directory where the default profile is stored.
		// set it to empty to prevent chromedp from setting it to a temp directory.
		chromedp.UserDataDir(""),
		// in headless mode, chrome won't load the default profile.
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-extensions", false),
		chromedp.Flag("profile-directory", profileDirectory),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("restore-on-startup", false),
		chromedp.ExecPath(chrome),
	)

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var nodes []*cdp.Node
	waitSelector := "#facebook a"
	linksSelector := "#facebook a"
	var res interface{}
	err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(pageUrl),
		chromedp.WaitReady(waitSelector),
		// We can't use chromedp.Click() because the 'See more' link might not actually exist
		chromedp.EvaluateAsDevTools("let more = document.querySelector('div[aria-label=\"See more\"]'); if (more){more.click()};''", &res),
		chromedp.Sleep(2 * time.Second),
		chromedp.EvaluateAsDevTools("more = document.querySelector('div[aria-label=\"See more\"]'); if (more){more.click()};''", &res),
		chromedp.Sleep(2 * time.Second),
		chromedp.EvaluateAsDevTools("more = document.querySelector('div[aria-label=\"See more\"]'); if (more){more.click()};''", &res),
		chromedp.Sleep(2 * time.Second),
		chromedp.Nodes(linksSelector, &nodes),
	})
	if err != nil {
		log.Fatalln(err)
	}

	for _, node := range nodes {
		href := node.AttributeValue("href")
		url, err := url.Parse(href)
		if err != nil {
			log.Fatal(err)
		}
		match, _ := regexp.MatchString("/events/\\d+/", url.Path)
		if match {
			links = append(links, url.Path)
		}
	}
	return removeDuplicateStr(links)
}

func main() {
	// read config from config.yaml.  We can improve this and make a nice ui to edit in the future
	c := viper.New()
	c.SetConfigFile("./config.yaml")
	err := c.ReadInConfig()
	if err != nil {
		fmt.Errorf("Error %v\n", err)
	}
	c.SetDefault("ChromePath", "/opt/google/chrome/chrome")
	c.SetDefault("ProfileDirectory", "Default")

	// build a new calendar
	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodRequest)

	// add events to the calendar
	events := getFacebookEventLinks(c.GetString("FacebookPage"),
		c.GetString("ChromePath"),
		c.GetString("ProfileDirectory"))
	for _, event := range events {
		u, _ := url.Parse(event)
		u.Scheme = "https"
		u.Host = "www.facebook.com"
		calEvent, err := fb2ical(u.String())
		if err != nil {
			log.Printf("%s %s\n", u.String(), err)
		} else {
			cal.Components = append(cal.Components, &calEvent)
		}
	}

	// @TODO write to a file instead of stdout
	fmt.Print(cal.Serialize())
}
