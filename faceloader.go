package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/araddon/dateparse"
	ics "github.com/arran4/golang-ical"
	"github.com/daetal-us/getld/extract"
	"github.com/spf13/viper"
	"log"
	"net/url"
	"os/exec"
	"regexp"
	"time"
)

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

func fb2ical(url string) (ics.VEvent, error) {
	results, _ := extract.FromURL(url)
	encoded, _ := json.Marshal(results)

	var events []eventScheme
	json.Unmarshal(encoded, &events)
	if len(events) == 0 {
		return ics.VEvent{}, errors.New("no events found")
	}
	var event = events[0]

	//cal := ics.NewCalendar()
	//cal.SetMethod(ics.MethodRequest)

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

	r, _ := regexp.Compile("\\d+")
	icsEvent.SetProperty(ics.ComponentPropertyUniqueId, r.FindString(event.Url))

	startTime, _ := dateparse.ParseAny(event.StartDate)
	icsEvent.SetStartAt(startTime)

	endTime, _ := dateparse.ParseAny(event.EndDate)
	icsEvent.SetEndAt(endTime)
	icsEvent.SetDtStampTime(time.Now())

	return icsEvent, nil
}

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

func getFacebookEventLinks(pageUrl string, chrome string) []string {
	var links []string
	out, err := exec.Command(chrome, "--headless", "--disable-gpu", "--dump-dom", pageUrl).Output()
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Printf("DOM: %s\n", out)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(out))
	if err != nil {
		log.Fatal(err)
	}
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		link, ok := s.Attr("href")
		if ok {
			u, err := url.Parse(link)
			if err != nil {
				log.Fatal(err)
			}
			match, _ := regexp.MatchString("/events/\\d+/", u.Path)
			if match {
				links = append(links, u.Path)
			}
		}
	})
	return removeDuplicateStr(links)
}

func main() {
	c := viper.New()
	c.SetConfigFile("./config.yaml")
	err := c.ReadInConfig()
	if err != nil {
		fmt.Errorf("Error %v\n", err)
	}
	c.SetDefault("ChromePath", "/opt/google/chrome/chrome")


	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodRequest)

	events := getFacebookEventLinks(c.GetString("FacebookPage"), c.GetString("ChromePath"))
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
	fmt.Print(cal.Serialize())
}
