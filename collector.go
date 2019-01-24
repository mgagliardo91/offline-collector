package main

import (
	"net/http"
	"net/url"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

func createCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.AllowedDomains("get-offline.com", "www.get-offline.com"),
		colly.Async(true),
		colly.AllowURLRevisit(),
	)

	setupStorage(c)
	setupProxy(c)
	extensions.RandomUserAgent(c)

	return c
}

func setupStorage(c *colly.Collector) {
	storage := createRedisStorage()
	if storage != nil {
		err := c.SetStorage(storage)

		if err != nil {
			panic(err)
		}

		if err := storage.Clear(); err != nil {
			panic(err)
		}

		defer storage.Client.Close()
	}
}

func setupProxy(c *colly.Collector) {
	c.SetProxyFunc(getRandom)
}

func getRandom(pr *http.Request) (*url.URL, error) {
	proxyURL, err := proxyList.GetRandom()

	if err != nil {
		return nil, err
	}

	pr.Header.Add(ProxyURLKey, proxyURL.String())

	return proxyURL, nil
}
