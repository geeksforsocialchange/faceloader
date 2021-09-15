package main

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
	"github.com/spf13/viper"
	"log"
	"net/url"
	"os"
	"path"
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

func browserContext(chrome string, debug bool) (context.Context, context.Context) {
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

func maybeLogin(ctx context.Context, username string, password string) error {
	// @TODO this seems to need to login every run, despite saving cookies

	selName := `//input[@id="email"]`
	selPass := `//input[@id="pass"]`

	err := chromedp.Run(ctx, chromedp.Tasks{
		network.Enable(),
		chromedp.Navigate(`https://www.facebook.com`),
		chromedp.WaitVisible(selPass),
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

//go:embed more.js
var more string

func awaitPromise(params *runtime.EvaluateParams) *runtime.EvaluateParams {
	return params.WithAwaitPromise(true)
}

// find links to Facebook events from a url, using Chrome so that we do it as a logged-in Facebook user
func getFacebookEventLinks(ctx context.Context, pageUrl string) []string {
	var links []string

	var nodes []*cdp.Node
	waitSelector := "#facebook a"
	linksSelector := "#facebook a"
	var res interface{}
	err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(pageUrl),
		chromedp.WaitReady(waitSelector),
		chromedp.EvaluateAsDevTools(more, &res, awaitPromise),
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

	_, browserContext := browserContext(c.GetString("ChromePath"), c.GetBool("Debug"))
	err := maybeLogin(browserContext, c.GetString("Username"), c.GetString("Password"))
	if err != nil {
		log.Println(err)
	}

	log.Println(c.GetString("FacebookPage"))

	// add events to the calendar
	events := getFacebookEventLinks(browserContext, c.GetString("FacebookPage"))
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

	// Manually cancel the context to gracefully close the browser
	err = chromedp.Cancel(browserContext)
	if err != nil {
		log.Fatalf("error canceling browserContext: %s\n", err)
	}
}
