package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gocolly/colly"
	"github.com/mgagliardo91/blacksmith"
)

type DataCollection struct {
	data []string
	name string
	mux  sync.Mutex
}

func (d *DataCollection) append(value string) {
	d.mux.Lock()
	defer d.mux.Unlock()

	d.data = append(d.data, value)
}

func (d *DataCollection) dump() {
	value := strings.Join(d.data, "\n")
	name := fmt.Sprintf("collect/%s.out", d.name)
	ioutil.WriteFile(name, []byte(value), 0644)
}

var addrCollection DataCollection = DataCollection{name: "address"}
var timeCollection DataCollection = DataCollection{name: "time"}
var priceCollection DataCollection = DataCollection{name: "price"}
var re = regexp.MustCompile(`(\s{2,}|[\r\n]+)`)

func collectDetail(task blacksmith.Task) {
	event, ok := task.Payload.(OfflineEvent)
	if !ok {
		task.LogfUsing(log.Panicf, "Unable to collect detail for payload that is not an OfflineEvent: %+v", task.Payload)
		return
	}

	c := createCollector()

	c.OnHTML("div.show__address", func(e *colly.HTMLElement) {
		el := e.DOM
		address := el.Find("address")
		value := strings.TrimSpace(address.Text())
		addrCollection.append(re.ReplaceAllLiteralString(value, " "))
	})

	c.OnHTML("div.show__time", func(e *colly.HTMLElement) {
		value := strings.TrimSpace(e.Text)
		timeCollection.append(re.ReplaceAllLiteralString(value, " "))
	})

	c.OnHTML("div.show__price", func(e *colly.HTMLElement) {
		value := strings.TrimSpace(e.Text)
		priceCollection.append(re.ReplaceAllLiteralString(value, " "))
	})

	c.OnRequest(func(r *colly.Request) {
		task.Logf("visiting: %s", event.URL)
	})

	c.OnResponse(func(r *colly.Response) {
		jsonValue, _ := json.Marshal(event)
		_, err := http.NewRequest("POST", "http://localhost:3000/event", bytes.NewBuffer(jsonValue))
		if err != nil {
			task.LogfUsing(log.Panicf, "Error posting task to server: %v", err)
		}
	})

	c.Visit(event.URL)
	c.Wait()
	task.Logf("Finished visiting %s", event.URL)
}

func DumpCollections() {
	newpath := filepath.Join(".", "collect")
	os.MkdirAll(newpath, os.ModePerm)

	addrCollection.dump()
	timeCollection.dump()
	priceCollection.dump()
}
