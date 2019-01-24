package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	utils "github.com/mgagliardo91/go-utils"
)

type ProxyRequester func(requestCount int, logger *utils.LogWrapper) []string

type ProxyEntry struct {
	url           *url.URL
	lastValidated time.Time
}

type ProxyList struct {
	proxyURLs  []ProxyEntry
	mux        sync.RWMutex
	requestMux sync.Mutex
}

const (
	ProxyURLKey string = "PROXY_URL"
)

var (
	maxProxyUrls         = utils.GetEnvInt("MAX_PROXY_URLS", 2)
	proxyScanMin         = utils.GetEnvInt("PROXY_VALIDATOR_SCAN_MIN", 1)
	proxyList            ProxyList
	proxyRequest         chan bool
	proxyStop            utils.StopChannel
	proxyValidatorStop   utils.StopChannel
	proxyValidatorTicker *time.Timer
	proxyRequesterFunc   ProxyRequester
)

func startProxyService(proxyRequester ProxyRequester) {
	proxyRequesterFunc = proxyRequester
	proxyRequest = make(chan bool)
	proxyStop = utils.NewStopChannel()
	proxyList = ProxyList{
		proxyURLs: make([]ProxyEntry, 0),
	}

	proxyList.Load()

	go func() {
		GetLogger().Infoln("[ProxyService]: Starting")
		for {
			select {
			case <-proxyRequest:
				{
					if proxyList.Len() < maxProxyUrls {
						requestNewProxies()
						checkProxyCount()
					}
				}
			case <-proxyStop.OnRequest:
				{
					GetLogger().Infoln("[ProxyService]: Exiting")
					proxyStop.Stop()
					return
				}
			}
		}
	}()

	// initialize
	if proxyList.Len() < maxProxyUrls {
		checkProxyCount()
	} else {
		validateProxies()
	}

	for {
		if proxyList.Len() >= maxProxyUrls {
			break
		}

		GetLogger().Infoln("Waiting for full proxy list to start...")
		time.Sleep(2 * time.Second)
	}

	startProxyValidator()
}

func stopProxyService() {
	proxyValidatorTicker.Stop()
	proxyValidatorStop.RequestStop()
	proxyStop.RequestStop()
}

func startProxyValidator() {
	proxyValidatorTicker = time.NewTimer(time.Duration(proxyScanMin) * time.Minute)
	proxyValidatorStop = utils.NewStopChannel()

	go func() {
		GetLogger().Infoln("[ProxyValidatorService]: Starting")
		for {
			select {
			case <-proxyValidatorTicker.C:
				validateProxies()
				GetLogger().Infoln("[ProxyValidatorService]: Resetting timer")
			case <-proxyValidatorStop.OnRequest:
				proxyValidatorTicker.Stop()
				GetLogger().Infoln("[ProxyValidatorService]: Exiting")
				proxyValidatorStop.Stop()
				return
			}
		}
	}()
}

func validateProxies() {
	needsCheck := false

	if proxyValidatorTicker != nil {
		proxyValidatorTicker.Stop()
	}

	proxyList.mux.RLock()
	entriesToValidate := make([]*ProxyEntry, 0)
	for i := range proxyList.proxyURLs {
		entry := &proxyList.proxyURLs[i]
		if entry.lastValidated.IsZero() || time.Since(entry.lastValidated) > time.Duration(15*time.Second) {
			entriesToValidate = append(entriesToValidate, entry)
		}
	}
	proxyList.mux.RUnlock()

	for _, urlEntry := range entriesToValidate {
		var timeout = time.Duration(15 * time.Second)
		client := &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(urlEntry.url)},
			Timeout:   timeout}
		req, _ := http.NewRequest("GET", "http://free.timeanddate.com/ts.php", nil)
		req.Close = true

		_, err := client.Do(req)
		if err != nil {
			proxyList.Remove(urlEntry.url)
			needsCheck = true
		} else {
			GetLogger().Infof("Url validated: %s\n", urlEntry.url.String())
			urlEntry.lastValidated = time.Now()
		}
	}

	if needsCheck {
		checkProxyCount()
	}

	if proxyValidatorTicker != nil {
		proxyValidatorTicker.Reset(time.Duration(proxyScanMin) * time.Minute)
	}
}

func checkProxyCount() {
	go func() {
		proxyRequest <- true
	}()
}

func requestNewProxies() {
	proxyList.requestMux.Lock()
	defer proxyList.requestMux.Unlock()

	GetLogger().Infoln("[ProxyService]: Obtaining new proxies")
	count := maxProxyUrls - proxyList.Len()

	if count > 20 {
		count = 20
	} else if count <= 0 {
		return
	}

	proxyListResponse := proxyRequesterFunc(count, GetLogger())

	for _, result := range proxyListResponse {
		proxyList.Add(result)
	}

	validateProxies()
}

func (p *ProxyList) Len() int {
	p.mux.RLock()
	defer p.mux.RUnlock()

	return len(p.proxyURLs)
}

func (p *ProxyList) GetRandom() (*url.URL, error) {
	p.mux.RLock()
	defer p.mux.RUnlock()

	l := len(p.proxyURLs)

	if l <= 0 {
		return nil, errors.New("No proxy urls available")
	} else if l == 1 {
		return p.proxyURLs[0].url, nil
	}

	i := rand.Intn(l - 1)

	return p.proxyURLs[i].url, nil
}

func (p *ProxyList) Add(urlString string) {
	p.mux.Lock()
	defer p.mux.Unlock()

	p.add(urlString)
	p.Save()
}

func (p *ProxyList) add(urlString string) {
	urlItem, err := url.Parse(urlString)
	if err != nil {
		GetLogger().Errorf("Found unparseable proxy host %s", urlString)
		return
	}

	proxyEntry := ProxyEntry{
		url: urlItem,
	}

	p.proxyURLs = append(p.proxyURLs, proxyEntry)
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
		GetLogger().Infof("[ProxyService]: Removing URL %s\n", urlToRemove.Path)
		p.mux.Lock()

		j := 0
		for _, n := range p.proxyURLs {
			if n.url.String() != urlToRemove.String() {
				p.proxyURLs[j] = n
				j++
			}
		}
		p.proxyURLs = p.proxyURLs[:j]
		p.Save()

		p.mux.Unlock()
		checkProxyCount()
	}
}

func (p *ProxyList) Load() {
	urls := make([]string, 0)

	if jsonBlob, err := ioutil.ReadFile("proxyList.json"); err == nil {
		json.Unmarshal(jsonBlob, &urls)
	}

	for _, urlString := range urls {
		p.add(urlString)
	}

	p.Save()
}

func (p *ProxyList) Save() {
	urls := make([]string, 0)

	for _, entry := range p.proxyURLs {
		urls = append(urls, entry.url.String())
	}

	urlsJSON, _ := json.Marshal(urls)
	ioutil.WriteFile("proxyList.json", urlsJSON, 0644)
}
