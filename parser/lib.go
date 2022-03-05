package faceloader

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/araddon/dateparse"
	ics "github.com/arran4/golang-ical"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/daetal-us/getld/extract"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"regexp"
	"time"
)

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

// take a Facebook event url and return an ICS event
// `extract.FromURL()` does the fetching, but we may want to use Chrome in the future
func Fb2ical(url string) (ics.VEvent, error) {
	results, _ := extract.FromURL(url)
	encoded, _ := json.Marshal(results)

	var events []EventScheme
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

// RemoveDuplicateStr de-duplicate a slice of strings
func RemoveDuplicateStr(strSlice []string) []string {
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

func BrowserContext(chrome string, debug bool) (context.Context, context.Context) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Couldn't get home directory: %s", err)
	}

	var contextOpts []chromedp.ContextOption
	if debug {
		contextOpts = []chromedp.ContextOption{
			chromedp.WithLogf(log.Printf),
			chromedp.WithDebugf(log.Printf),
			chromedp.WithErrorf(log.Printf),
		}
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.ExecPath(chrome),
		chromedp.UserDataDir(path.Join(home, ".faceloader", "userdata")),
	)
	allocatorCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	browserCtx, _ := chromedp.NewContext(allocatorCtx, contextOpts...)

	if debug {
		chromedp.ListenTarget(browserCtx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *runtime.EventConsoleAPICalled:
				log.Printf("* console.%s call:\n", ev.Type)
				for _, arg := range ev.Args {
					log.Printf("%s - %s\n", arg.Type, arg.Value)
				}
			case *runtime.EventExceptionThrown:
				s := ev.ExceptionDetails.Error()
				log.Printf("* %s\n", s)
			}
		})
	}

	chromedp.Run(browserCtx)

	return allocatorCtx, browserCtx
}

func MaybeLogin(ctx context.Context, username string, password string) error {
	timeoutContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	selName := `//input[@id="email"]`
	selPass := `//input[@id="pass"]`
	var acceptRes interface{}
	err := chromedp.Run(timeoutContext, chromedp.Tasks{
		network.Enable(),
		chromedp.Navigate(`https://www.facebook.com`),
		chromedp.WaitVisible(selPass),
		chromedp.EvaluateAsDevTools(acceptCookiesJS, &acceptRes, awaitPromise),
		chromedp.SendKeys(selName, username),
		chromedp.SendKeys(selPass, password),
		chromedp.Submit(selPass),
		//chromedp.WaitVisible(`//a[@title="Profile"]`),
	})
	if err == nil {
		log.Println("Performed login")
	}
	return err
}

//go:embed js/more.js
var moreJS string

//go:embed js/acceptCookies.js
var acceptCookiesJS string

func awaitPromise(params *runtime.EvaluateParams) *runtime.EvaluateParams {
	return params.WithAwaitPromise(true)
}

// GetFacebookEventLinks find links to Facebook events from a url, using Chrome so that we do it as a logged-in Facebook user
func GetFacebookEventLinks(ctx context.Context, pageUrl string, debug bool) []string {
	var links []string

	var nodes []*cdp.Node
	waitSelector := "#facebook a"
	linksSelector := "#facebook a"
	var res interface{}
	var buf []byte
	err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(pageUrl),
		chromedp.WaitReady(waitSelector),
		chromedp.EvaluateAsDevTools(moreJS, &res, awaitPromise),
		chromedp.FullScreenshot(&buf, 90),
		chromedp.Nodes(linksSelector, &nodes),
	})
	if err != nil {
		log.Fatalln(err)
	}
	if debug {
		ioutil.WriteFile("fullScreenshot.png", buf, 0o600)
		log.Println("Saved screenshot of final event listing page to fullScreenshot.png")
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
	return RemoveDuplicateStr(links)
}
