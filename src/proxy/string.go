package proxy

import (
	"net/url"
	"strings"
)

func IsSmartproxy(proxyUrl string) bool {
	parsedProxyUrl, err := url.Parse(proxyUrl)
	if err != nil {
		panic(err)
	}
	return strings.Split(parsedProxyUrl.Host, ":")[0] == "dc.smartproxy.com"
}
