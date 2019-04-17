package main

import (
	"errors"
	"flag"
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

type OfflineEventRequest struct {
	Date    time.Time `json:"date"`
	Title   string    `json:"title"`
	Details string    `json:"details"`
	URL     string    `json:"url"`
}

type FailureMap struct {
	failures map[string]uint64
	mux      sync.Mutex
}

type SimpleDate time.Time

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

var events map[time.Time][]OfflineEventRequest

var failureMap = FailureMap{
	failures: make(map[string]uint64),
}

func nextDateURL(date time.Time) string {
	nextRequestDate := date.AddDate(0, 0, 1)
	return createCalendarURL(nextRequestDate.Format(calendarFormat))
}

func createCalendarURL(dateString string) string {
	return fmt.Sprintf("%s?date=%s", calendarURL, dateString)
}

func (s *SimpleDate) String() string {
	return time.Time(*s).Format(calendarFormat)
}

func (s *SimpleDate) Set(value string) error {
	dateVal, err := time.Parse(calendarFormat, value)
	if err != nil {
		return errors.New("Unable to parse to date value. Format YYYY-MM-DD")
	}

	*s = SimpleDate(dateVal)
	return nil
}

func main() {
	var startDate, endDate SimpleDate
	flag.Var(&startDate, "start", "Start date YYYY-MM-DD")
	flag.Var(&endDate, "end", "End date YYYY-MM-DD")
	flag.Parse()

	if startDate == (SimpleDate{}) {
		startDate.Set(time.Now().Format(calendarFormat))
	}

	if endDate == (SimpleDate{}) {
		endDate.Set(startDate.String())
	}

	log.Printf("Collecting offline events between %s and %s \n", startDate.String(), endDate.String())

	utils.SetLoggerLevel(blacksmith.LoggerName, "info")
	startProxyService(proxy.RequestGetProxy)
	defer stopProxyService()

	c := createCollector()

	var dateSet sync.Map
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

		event := OfflineEventRequest{
			Date:    date,
			Title:   strings.TrimSpace(el.Find("div[class$=\"_title\"]").Text()),
			Details: strings.TrimSpace(el.Find("div[class$=\"_details\"]").Text()),
			URL:     e.Request.AbsoluteURL(link),
		}

		blacksmith.QueueTask(EventDetail, event)
	})

	c.OnHTML(".calender-sliders__date", func(e *colly.HTMLElement) {
		date := e.Attr("id")

		if _, hasDate := dateSet.Load(date); !hasDate {
			dateSet.Store(date, true)

			dateVal, err := time.Parse(calendarFormat, date)
			if err != nil {
				GetLogger().Errorf("Unable to parse date ID as date: %s. %s", date, err)
			}

			if (time.Time(endDate).Equal(dateVal) || time.Time(endDate).After(dateVal)) && (time.Time(startDate).Before(dateVal)) {
				nextURL := nextDateURL(dateVal)
				c.Visit(nextURL)
			}
		}
	})

	c.OnRequest(func(r *colly.Request) {
		GetLogger().Infoln("visiting", r.URL.String())
	})

	c.OnResponse(func(r *colly.Response) {
		dateString := r.Request.URL.Query().Get("date")
		date, _ := time.Parse(calendarFormat, dateString)
		r.Request.Ctx.Put("date", date)

		dateSet.Store(dateString, true)
	})

	// Set error handler
	c.OnError(func(r *colly.Response, err error) {
		GetLogger().Infoln("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)

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
	c.Visit(createCalendarURL(startDate.String()))
	c.Wait()

	blacksmith.Stop()
}

var logger *utils.LogWrapper

func GetLogger() *utils.LogWrapper {
	if logger == nil {
		logger = utils.NewLogger("collector")
	}

	return logger
}
