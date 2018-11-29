package main

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gocolly/colly"
)

type OfflineEvent struct {
	Date    time.Time `json:"date"`
	Title   string    `json:"title"`
	Details string    `json:"details"`
	URL     string    `json:"url"`
}

const calendarFormat = "2006-01-02"
const calendarURL = "https://www.get-offline.com/raleigh/calendar"

var stopDate, _ = time.Parse(calendarFormat, "2018-11-27")
var events map[time.Time][]OfflineEvent
var failures uint64

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
	startProxyService()
	c := createCollector()

	dispatcher := NewDispatcher(MaxWorker)
	defer dispatcher.Stop()
	dispatcher.Run()

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

		JobQueue <- Job{Payload: event, TaskName: EventDetail}
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
		atomic.AddUint64(&failures, 1)

		if failureCount := atomic.LoadUint64(&failures); failureCount > 5 {
			return
		}

		proxyUrl := r.Request.Headers.Get(ProxyURLKey)
		if len(proxyUrl) > 0 {
			proxyList.Remove(proxyUrl)
		}

		c.Visit(r.Request.URL.String())
	})

	// Start scraping
	c.Visit(createCalendarURL("2018-11-27"))
}
