package faceloader

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/araddon/dateparse"
	ics "github.com/arran4/golang-ical"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type MBasic struct {
	httpClient      *http.Client
	approvedCookies bool
}

// EventScheme The key parts that a Facebook json+ld event includes
type EventScheme struct {
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

func NewMBasicClient() *MBasic {
	jar, _ := cookiejar.New(nil)
	return &MBasic{
		httpClient: &http.Client{
			Jar: jar,
		},
		approvedCookies: false,
	}
}

func (m *MBasic) Get(link string) ([]byte, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}

	if !m.approvedCookies {
		u, _ := url.Parse("https://mbasic.facebook.com/cookie/consent/?next_uri=https%3A%2F%2Fmbasic.facebook.com%2F")
		resp, err := m.httpClient.Post(u.String(), "application/x-www-form-urlencoded", bytes.NewBuffer([]byte("accept_only_essential=1")))
		if err != nil {
			return nil, err
		}
		m.httpClient.Jar.SetCookies(u, resp.Cookies())
		m.approvedCookies = true
	}
	resp, err := m.httpClient.Get(link)
	if err != nil {
		return nil, err
	}
	m.httpClient.Jar.SetCookies(u, resp.Cookies())

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("status code error: %d %s", resp.StatusCode, resp.Status))
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	printTitle(respBody)
	return respBody, err
}

func (m *MBasic) InterfaceFromMbasic(eventUrl string) (map[string]interface{}, error) {
	var result []map[string]interface{}
	res, err := m.Get(eventUrl)
	if err != nil {
		return nil, nil
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(res))
	if err != nil {
		return nil, err
	}
	selector := `script[type="application/ld+json"]`
	scripts := doc.Find(selector)
	scripts.Each(func(i int, s *goquery.Selection) {
		var decoded map[string]interface{}
		text := s.Text()
		text = strings.Replace(text, "//<![CDATA[", "", -1)
		text = strings.Replace(text, "//]]", "", -1)
		text = strings.Replace(text, ">", "", -1)
		err = json.Unmarshal([]byte(text), &decoded)
		if err != nil {
			return
		}
		if err == nil {
			result = append(result, decoded)
		}
	})
	if len(result) > 0 {
		return result[0], nil
	} else {
		return nil, errors.New(fmt.Sprintf("no ld+json events found on %v", eventUrl))
	}
}

func InterfaceToIcal(i map[string]interface{}) (ics.VEvent, error) {
	encoded, err := json.Marshal(i)
	if err != nil {
		return ics.VEvent{}, err
	}
	var event EventScheme
	err = json.Unmarshal(encoded, &event)
	if err != nil {
		return ics.VEvent{}, err
	}

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

// RemoveDuplicateStr de-duplicate a slice of strings
func RemoveDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	var list []string
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

// GetFacebookEventLinks find links to Facebook events from a Facebook page name
func (m *MBasic) GetFacebookEventLinks(pageName string) ([]string, error) {
	var links []string
	res, err := m.Get(fmt.Sprintf("https://mbasic.facebook.com/%v?v=events", pageName))
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(res))
	if err != nil {
		return nil, err
	}
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		link, _ := url.Parse(href)
		match, _ := regexp.MatchString("/events/\\d+", link.Path)
		if match {
			link.Scheme = "https"
			link.Host = "mbasic.facebook.com"
			link.RawQuery = ""
			links = append(links, link.String())
		}
	})

	return RemoveDuplicateStr(links), nil
}

func printTitle(body []byte) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(body))
	if err != nil {
		log.Println(err)
	}
	doc.Find("title").Each(func(i int, s *goquery.Selection) {
		log.Println("Page title:", s.Text())
	})
}
