package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"github.com/mgagliardo91/blacksmith"
	"github.com/mgagliardo91/offline-common"
)

var re = regexp.MustCompile(`(\s{2,}|[\r\n]+)`)

func collectDetail(task blacksmith.Task) {
	eventRequest, ok := task.Payload.(OfflineEventRequest)
	if !ok {
		task.LogfUsing(GetLogger().Errorf, "Unable to collect detail for payload that is not an OfflineEventRequest: %+v", task.Payload)
		return
	}

	c := createCollector()
	offlineEvent := common.NewOfflineEvent()
	offlineEvent.OfflineDate = eventRequest.Date
	offlineEvent.OfflineURL = eventRequest.URL

	c.OnHTML("div.show__address", func(e *colly.HTMLElement) {
		address := e.DOM.Find("address")
		value := re.ReplaceAllLiteralString(strings.TrimSpace(address.Text()), " ")

		offlineEvent.LockAndUpdate(func() {
			offlineEvent.LocationRaw = value
		})
	})

	c.OnHTML(".show__title", func(e *colly.HTMLElement) {
		values := make([]string, 0)
		e.DOM.Contents().Each(func(i int, s *goquery.Selection) {
			if goquery.NodeName(s) == "#text" {
				values = append(values, re.ReplaceAllLiteralString(strings.TrimSpace(s.Text()), " "))
			}
		})

		offlineEvent.LockAndUpdate(func() {
			offlineEvent.Title = strings.TrimSpace(strings.Join(values[:], " "))
		})
	})

	c.OnHTML(".show__teaser", func(e *colly.HTMLElement) {
		value := re.ReplaceAllLiteralString(strings.TrimSpace(e.Text), " ")
		offlineEvent.LockAndUpdate(func() {
			offlineEvent.Teaser = value
		})
	})

	c.OnHTML("div.show__description", func(e *colly.HTMLElement) {
		values := make([]string, 0)
		e.DOM.Find("p").Each(func(idx int, elem *goquery.Selection) {
			values = append(values, re.ReplaceAllLiteralString(strings.TrimSpace(elem.Text()), " "))
		})

		offlineEvent.LockAndUpdate(func() {
			offlineEvent.Description = strings.Join(values[:], " ")
		})
	})

	c.OnHTML("div.show__description a", func(e *colly.HTMLElement) {
		value := e.Attr("href")

		offlineEvent.LockAndUpdate(func() {
			offlineEvent.ReferralURLs = append(offlineEvent.ReferralURLs, value)
		})
	})

	c.OnHTML("div.show__image-wrap > img", func(e *colly.HTMLElement) {
		value := e.Attr("src")

		offlineEvent.LockAndUpdate(func() {
			offlineEvent.ImageURL = value
		})
	})

	c.OnHTML("div.show__schedule > div.show__time", func(e *colly.HTMLElement) {
		value := re.ReplaceAllLiteralString(strings.TrimSpace(e.Text), " ")
		offlineEvent.LockAndUpdate(func() {
			offlineEvent.DateRaw = value
		})
	})

	c.OnHTML("div.show__hours > div.show__time", func(e *colly.HTMLElement) {
		value := re.ReplaceAllLiteralString(strings.TrimSpace(e.Text), " ")
		offlineEvent.LockAndUpdate(func() {
			offlineEvent.TimeRaw = value
		})
	})

	c.OnHTML("div.show__price", func(e *colly.HTMLElement) {
		value := re.ReplaceAllLiteralString(strings.TrimSpace(e.Text), " ")
		offlineEvent.LockAndUpdate(func() {
			offlineEvent.PriceRaw = value
		})
	})

	c.OnHTML("div.show__website > a", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		url := e.Request.AbsoluteURL(href)

		task.LogfUsing(GetLogger().Tracef, "Visiting redirect URL at %s", url)
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := client.Get(e.Request.AbsoluteURL(href))

		if err != nil {
			task.LogfUsing(GetLogger().Errorf, "Error requesting event URL: %v", err)
			return
		}

		if resp.StatusCode == 302 {
			location := resp.Header.Get("Location")
			offlineEvent.LockAndUpdate(func() {
				offlineEvent.EventURL = location
			})
		} else {
			task.LogfUsing(GetLogger().Errorf, "Found wrong status code for url %s: %v", href, resp.StatusCode)
		}
	})

	c.OnRequest(func(r *colly.Request) {
		task.LogfUsing(GetLogger().Infof, "visiting: %s", eventRequest.URL)
	})

	c.Visit(eventRequest.URL)
	c.Wait()

	jsonValue, _ := json.Marshal(offlineEvent)
	_, err := http.Post("http://localhost:3000/event", "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		GetLogger().Errorf("Error posting task to server: %v", err)
	}

	task.LogfUsing(GetLogger().Infof, "Finished visiting %s", eventRequest.URL)
}

type updateFunc func(event *common.RawOfflineEvent)

func updateEvent(event *common.RawOfflineEvent, lock *sync.Mutex, update updateFunc) {
	lock.Lock()
	defer lock.Unlock()

	update(event)
}
