package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

type PubProxyListResponse struct {
	Data  []PubProxyResponseItem `json:"data"`
	Count uint64                 `json:"count"`
}

type PubProxyResponseItem struct {
	Protocol string `json:"type"`
	Path     string `json:"ipPort"`
}

func RequestPubProxy(requestCount int) []string {
	resp, err := http.Get("http://pubproxy.com/api/proxy?type=https,socks4,socks5&country=US&limit=" + strconv.Itoa(requestCount))

	if err != nil {
		log.Panicf("Unable to complete request to obtain proxy. Err = %+v\n", err.Error())
		return nil
	}

	var listResponse = PubProxyListResponse{}
	err = json.NewDecoder(resp.Body).Decode(&listResponse)

	if err != nil {
		log.Panicf("Unable to parse proxy request. Err = %+v\n", err.Error())
		return nil
	}

	retVal := make([]string, 0)

	for _, result := range listResponse.Data {
		retVal = append(retVal, fmt.Sprintf("%s://%s", result.Protocol, result.Path))
	}

	return retVal
}
