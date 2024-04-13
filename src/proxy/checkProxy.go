package proxy

import (
	"fmt"
	"io"
	"main/config"
	"net/http"
	"net/url"
)

func sendRequestThroughProxy(pxy string, targetURL string) (string, error) {
	proxyUser, proxyPass, _, parsedProxyUrl, err := splitProxyURL(pxy)
	if err != nil {
		return "", err
	}

	if proxyUser != "" && proxyPass != "" {
		parsedProxyUrl.User = url.UserPassword(proxyUser, proxyPass)
	}
	transport := &http.Transport{
		Proxy: http.ProxyURL(parsedProxyUrl),
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   config.GetConfig().ProxyConnectTimeout,
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
