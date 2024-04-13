package proxy

import (
	"main/config"
	"net/url"
	"slices"
	"strings"
)

func splitProxyURL(proxyURL string) (string, string, string, *url.URL, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return "", "", "", nil, err
	}

	var username, password string
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	host := u.Host

	return username, password, host, u, nil
}

func isThirdparty(proxyUrl string) bool {
	return slices.Contains(config.GetConfig().ProxyPoolThirdparty, proxyUrl)
}

func stripHTTP(url string) string {
	var newStr string
	newStr = strings.Replace(url, "http://", "", -1)
	newStr = strings.Replace(newStr, "https://", "", -1)
	return newStr
}
