package main

import (
	"log"

	"github.com/gocolly/colly"
)

func collectDetail(job Job) {
	event, ok := job.Payload.(OfflineEvent)
	if !ok {
		job.Logf(log.Panicf, "Unable to collect detail for payload that is not an OfflineEvent: %+v", job.Payload)
		return
	}

	c := colly.NewCollector(
		colly.AllowedDomains("get-offline.com", "www.get-offline.com"),
	)

	c.OnHTML("", func(e *colly.HTMLElement) {

	})

	c.OnRequest(func(r *colly.Request) {
		job.Log(log.Println, "visiting", event.URL)
	})

	c.OnResponse(func(r *colly.Response) {

	})

	c.Visit(event.URL)
}
