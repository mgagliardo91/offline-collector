package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/mgagliardo91/offline/proxy"
)

type ProxyRequester func(requestCount int) ProxyListResponse

type ProxyList struct {
	ProxyURLs  []*url.URL `json:"proxyUrls"`
	mux        sync.RWMutex
	requestMux sync.Mutex
}

type ProxyListResponse []string

const (
	ProxyURLKey string = "PROXY_URL"
)

var (
	maxProxyUrls         = getEnvInt("MAX_PROXY_URLS", 2)
	proxyScanMin         = getEnvInt("PROXY_VALIDATOR_SCAN_MIN", 1)
	proxyList            ProxyList
	proxyRequest         chan bool
	proxyStop            chan chan ChannelStop
	proxyValidatorStop   chan chan ChannelStop
	proxyValidatorTicker *time.Timer
)

func startProxyService() {
	proxyRequest = make(chan bool)
	proxyStop = make(chan chan ChannelStop)
	proxyList = ProxyList{
		ProxyURLs: make([]*url.URL, 0),
	}

	if jsonBlob, err := ioutil.ReadFile("proxyList.json"); err == nil {
		json.Unmarshal(jsonBlob, &proxyList)
	}

	go func() {
		log.Println("[ProxyService]: Starting")
		for {
			select {
			case <-proxyRequest:
				{
					if proxyList.Len() < maxProxyUrls {
						requestNewProxies()
						checkProxyCount()
					}
				}
			case stop := <-proxyStop:
				{
					log.Println("[ProxyService]: Exiting")
					close(stop)
					return
				}
			}
		}
	}()

	// initialize
	checkProxyCount()

	for {
		if proxyList.Len() >= maxProxyUrls {
			break
		}

		log.Println("Waiting for full proxy list to start...")
		time.Sleep(2 * time.Second)
	}

	startProxyValidator()
}

func stopProxyService() {
	proxyValidatorTicker.Stop()
	stopChannel(proxyValidatorStop)
	stopChannel(proxyStop)
}

func startProxyValidator() {
	proxyValidatorTicker = time.NewTimer(time.Duration(proxyScanMin) * time.Minute)
	proxyValidatorStop = make(chan chan ChannelStop)

	go func() {
		log.Println("[ProxyValidatorService]: Starting")
		for {
			select {
			case <-proxyValidatorTicker.C:
				validateProxies()
				log.Println("[ProxyValidatorService]: Resetting timer")
			case stop := <-proxyValidatorStop:
				proxyValidatorTicker.Stop()
				close(stop)
				log.Println("[ProxyValidatorService]: Exiting")
				return
			}
		}
	}()

	validateProxies()
}

func validateProxies() {
	needsCheck := false
	proxyValidatorTicker.Stop()

	proxyList.mux.RLock()
	urlsToValidate := make([]*url.URL, len(proxyList.ProxyURLs))
	copy(urlsToValidate[:], proxyList.ProxyURLs)
	proxyList.mux.RUnlock()

	for _, url := range urlsToValidate {
		var timeout = time.Duration(15 * time.Second)
		client := &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(url)},
			Timeout:   timeout}
		_, err := client.Get("http://free.timeanddate.com/ts.php?t=1543866700891")

		if err != nil {
			proxyList.Remove(url)
			needsCheck = true
		} else {
			log.Printf("Url validated: %s\n", url.String())
		}
	}

	if needsCheck {
		checkProxyCount()
	}

	proxyValidatorTicker.Reset(time.Duration(proxyScanMin) * time.Minute)
}

func checkProxyCount() {
	go func() {
		proxyRequest <- true
	}()
}

func requestNewProxies() {
	proxyList.requestMux.Lock()
	defer proxyList.requestMux.Unlock()

	log.Println("[ProxyService]: Obtaining new proxies")
	count := maxProxyUrls - proxyList.Len()

	if count > 20 {
		count = 20
	} else if count <= 0 {
		return
	}

	proxyListResponse := proxy.RequestGetProxy(count)

	for _, result := range proxyListResponse {
		proxyList.Add(result)
	}

	validateProxies()
}

func (p *ProxyList) Len() int {
	p.mux.RLock()
	defer p.mux.RUnlock()

	return len(p.ProxyURLs)
}

func (p *ProxyList) GetRandom() (*url.URL, error) {
	p.mux.RLock()
	defer p.mux.RUnlock()

	l := len(p.ProxyURLs)

	if l <= 0 {
		return nil, errors.New("No proxy urls available")
	} else if l == 1 {
		return p.ProxyURLs[0], nil
	}

	i := rand.Intn(l - 1)

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
	} else if urlItem, ok := urlEntry.(*url.URL); ok {
		urlToRemove = urlItem
	}

	if urlToRemove != nil {
		log.Printf("[ProxyService]: Removing URL %s\n", urlToRemove.Path)
		p.mux.Lock()

		j := 0
		for _, n := range p.ProxyURLs {
			if n.String() != urlToRemove.String() {
				p.ProxyURLs[j] = n
				j++
			}
		}
		p.ProxyURLs = p.ProxyURLs[:j]
		p.Save()

		p.mux.Unlock()
		checkProxyCount()
	}
}

func (p *ProxyList) Save() {
	proxyJSON, _ := json.Marshal(p)
	ioutil.WriteFile("proxyList.json", proxyJSON, 0644)
}
