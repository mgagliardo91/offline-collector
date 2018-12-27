package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly"
	"github.com/mgagliardo91/blacksmith"
	"github.com/mgagliardo91/go-utils"
	"github.com/mgagliardo91/offline-collector/proxy"
)

type OfflineEvent struct {
	Date    time.Time `json:"date"`
	Title   string    `json:"title"`
	Details string    `json:"details"`
	URL     string    `json:"url"`
}

type FailureMap struct {
	failures map[string]uint64
	mux      sync.Mutex
}

func (f *FailureMap) increment(val string) uint64 {
	f.mux.Lock()
	defer f.mux.Unlock()

	v, ok := f.failures[val]

	if !ok {
		f.failures[val] = 0
	} else {
		f.failures[val] = v + 1
	}

	return f.failures[val]
}

const (
	EventDetail blacksmith.TaskName = iota
)

const calendarFormat = "2006-01-02"
const calendarURL = "https://www.get-offline.com/raleigh/calendar"

var (
	MaxWorker = utils.GetEnvInt("MAX_WORKERS", 10)
)

var stopDate, _ = time.Parse(calendarFormat, "2018-12-27")
var events map[time.Time][]OfflineEvent

var failureMap = FailureMap{
	failures: make(map[string]uint64),
}

func canVisitNextDate(date time.Time) bool {
	return !stopDate.Equal(date) || stopDate.After(date)
}

func nextDateURL(date time.Time) string {
	nextRequestDate := date.AddDate(0, 0, 1)
	return createCalendarURL(nextRequestDate.Format(calendarFormat))
}

func createCalendarURL(dateString string) string {
	return fmt.Sprintf("%s?date=%s", calendarURL, dateString)
}

func main() {
	startProxyService(proxy.RequestGetProxy)
	defer stopProxyService()

	c := createCollector()

	blacksmith := blacksmith.New(MaxWorker)
	blacksmith.SetHandler(EventDetail, collectDetail).Run()

	c.OnHTML(".experience-thumb--calendar > a[href]", func(e *colly.HTMLElement) {
		var date time.Time
		if dateVal, ok := e.Request.Ctx.GetAny("date").(time.Time); ok {
			date = dateVal
		} else {
			return
		}

		link := e.Attr("href")
		el := e.DOM

		event := OfflineEvent{
			Date:    date,
			Title:   strings.TrimSpace(el.Find("div[class$=\"_title\"]").Text()),
			Details: strings.TrimSpace(el.Find("div[class$=\"_details\"]").Text()),
			URL:     e.Request.AbsoluteURL(link),
		}

		blacksmith.QueueTask(EventDetail, event)
	})

	c.OnRequest(func(r *colly.Request) {
		log.Println("visiting", r.URL.String())
	})

	c.OnResponse(func(r *colly.Response) {
		dateString := r.Request.URL.Query().Get("date")
		date, _ := time.Parse(calendarFormat, dateString)
		r.Request.Ctx.Put("date", date)

		if canVisitNextDate(date) {
			nextURL := nextDateURL(date)
			c.Visit(nextURL)
		}
	})

	// Set error handler
	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)

		proxyURL := r.Request.Headers.Get(ProxyURLKey)
		if len(proxyURL) > 0 {
			failures := failureMap.increment(proxyURL)

			if failures > 2 {
				proxyList.Remove(proxyURL)
			}
		} else {
			time.Sleep(2 * time.Second)
		}

		// try again
		c.Visit(r.Request.URL.String())
	})

	// Start scraping
	c.Visit(createCalendarURL("2018-12-27"))
	c.Wait()

	blacksmith.Stop()
	DumpCollections()
}
