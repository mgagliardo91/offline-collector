package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sync"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

type ProxyList struct {
	ProxyURLs []*url.URL `json:"proxyUrls"`
	mux       sync.RWMutex
}

const (
	ProxyURLKey string = "PROXY_URL"
)

var (
	maxProxyUrls = getEnvInt("MAX_PROXY_URLS", 5)
	proxyList    ProxyList
	proxyRemoved chan bool
	proxyReady   chan bool
)

func createCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.AllowedDomains("get-offline.com", "www.get-offline.com"),
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
			log.Fatal(err)
		}

		defer storage.Client.Close()
	}
}

func setupProxy(c *colly.Collector) {
	c.SetProxyFunc((&proxyList).GetRandom)
}

func startProxyService() {
	proxyRemoved := make(chan bool)
	proxyReady := make(chan bool)
	proxyList = ProxyList{
		ProxyURLs: make([]*url.URL, 0),
	}

	if jsonBlob, err := ioutil.ReadFile("proxyList.json"); err == nil {
		json.Unmarshal(jsonBlob, &proxyList)
	}

	go func() {
		for {
			select {
			case <-proxyRemoved:
				{
					for {
						proxyList.mux.RLock()
						l := len(proxyList.ProxyURLs)
						proxyList.mux.RUnlock()

						if l >= maxProxyUrls {
							proxyReady <- true
							break
						}

						requestNextProxy()
					}
				}
			}
		}
	}()

	proxyRemoved <- true
	<-proxyReady
}

func requestNextProxy() {
	// resp, err := http.Get("https://gimmeproxy.com/api/getProxy?protocol=socks5,http,https&curl=true&maxCheckPeriod=300")
	proxyList.Add(fmt.Sprintf("http://random%d.com", rand.Intn(100)))
}

func (p *ProxyList) GetRandom(pr *http.Request) (*url.URL, error) {
	p.mux.RLock()
	defer proxyList.mux.RUnlock()

	l := len(p.ProxyURLs)

	if l == 0 {
		return nil, errors.New("No proxy urls available")
	}

	i := rand.Intn(len(p.ProxyURLs) - 1)
	proxyUrl := p.ProxyURLs[i]
	pr.Header.Add(ProxyURLKey, proxyUrl.String())

	return p.ProxyURLs[i], nil
}

func (p *ProxyList) Add(urlString string) {
	urlItem, err := url.Parse(urlString)
	if err != nil {
		log.Panicf("Found unparseable proxy host %s", urlString)
		return
	}

	p.mux.Lock()
	defer p.mux.Unlock()

	p.ProxyURLs = append(p.ProxyURLs, urlItem)
	p.Save()
}

func (p *ProxyList) Remove(urlEntry interface{}) {
	var urlToRemove *url.URL

	if s, ok := urlEntry.(string); ok {
		if urlItem, err := url.Parse(s); err == nil {
			urlToRemove = urlItem
		}
	} else if urlItem, ok := urlEntry.(url.URL); ok {
		urlToRemove = &urlItem
	}

	if urlToRemove != nil {
		p.mux.Lock()
		defer p.mux.Unlock()

		j := 0
		for _, n := range p.ProxyURLs {
			if n.String() != urlToRemove.String() {
				p.ProxyURLs[j] = n
				j++
			}
		}
		p.ProxyURLs = p.ProxyURLs[:j]
		p.Save()
		proxyRemoved <- true
	}
}

func (p ProxyList) Save() {
	proxyJSON, _ := json.Marshal(p)
	ioutil.WriteFile("proxyList.json", proxyJSON, 0644)
}
