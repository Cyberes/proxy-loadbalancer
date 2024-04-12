package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func sendRequestThroughProxy(proxyUrl string, targetURL string) (string, error) {
	parsedProxyUrl, err := url.Parse(proxyUrl)
	if err != nil {
		return "", err
	}

	if IsSmartproxy(proxyUrl) {
		// Set the username and password for proxy authentication if Smartproxy
		parsedProxyUrl.User = url.UserPassword(smartproxyUsername, smartproxyPassword)
	}
	transport := &http.Transport{
		Proxy: http.ProxyURL(parsedProxyUrl),
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   time.Second * 10,
	}

	response, err := client.Get(targetURL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return "", err
		}
		return string(bodyBytes), nil
	}
	return "", fmt.Errorf("bad response code %d", response.StatusCode)
}
