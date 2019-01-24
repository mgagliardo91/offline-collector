package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mgagliardo91/go-utils"
)

type GetProxyResponseItem struct {
	Protocol string `json:"protocol"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
}

func RequestGetProxy(_ int, logger *utils.LogWrapper) []string {
	resp, err := http.Get("https://api.getproxylist.com/proxy?allowsHttps=1&country[]=US")

	if err != nil {
		logger.Errorf("Unable to complete request to obtain proxy. Err = %+v\n", err.Error())
		return nil
	}

	var responseItem = GetProxyResponseItem{}
	err = json.NewDecoder(resp.Body).Decode(&responseItem)

	if err != nil {
		logger.Errorf("Unable to parse proxy request. Err = %+v\n", err.Error())
		return nil
	}

	retVal := make([]string, 1)
	retVal[0] = fmt.Sprintf("%s://%s:%d", responseItem.Protocol, responseItem.IP, responseItem.Port)

	return retVal
}
