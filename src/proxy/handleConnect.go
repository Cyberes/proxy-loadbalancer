package proxy

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"main/config"
	"net"
	"net/http"
	"net/url"
	"slices"
	"time"
)

var (
	HeaderThirdpartyIncludeBroken = "Thirdparty-Include-Broken"
	HeaderThirdpartyBypass        = "Thirdparty-Bypass"
)

func logProxyRequest(remoteAddr string, proxyHost string, targetHost string, returnCode *int, proxyConnectMode string, requestStartTime time.Time) {
	log.Infof(`%s -> %s -> %s -> %d -- %s -- %d ms`, remoteAddr, proxyHost, targetHost, *returnCode, proxyConnectMode, time.Since(requestStartTime).Milliseconds())
}

func (p *ForwardProxyCluster) validateRequestAndGetProxy(w http.ResponseWriter, req *http.Request) (string, string, string, string, *url.URL, error) {
	if p.BalancerReady.GetCount() != 0 {
		errStr := "proxy is not ready"
		http.Error(w, errStr, http.StatusServiceUnavailable)
		return "", "", "", "", nil, errors.New(errStr)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.ourOnlineProxies) == 0 && len(p.thirdpartyOnlineProxies) == 0 {
		errStr := "no valid backends"
		http.Error(w, errStr, http.StatusServiceUnavailable)
		return "", "", "", "", nil, errors.New(errStr)
	}

	headerIncludeBrokenThirdparty := req.Header.Get(HeaderThirdpartyIncludeBroken)
	req.Header.Del(HeaderThirdpartyIncludeBroken)
	headerBypassThirdparty := req.Header.Get(HeaderThirdpartyBypass)
	req.Header.Del(HeaderThirdpartyBypass)
	if headerBypassThirdparty != "" && headerIncludeBrokenThirdparty != "" {
		errStr := "duplicate options headers detected, rejecting request"
		http.Error(w, errStr, http.StatusBadRequest)
		return "", "", "", "", nil, errors.New(errStr)
	}

	var selectedProxy string
	if slices.Contains(config.GetConfig().ThirdpartyBypassDomains, req.URL.Hostname()) {
		selectedProxy = p.getProxyFromOurs()
	} else {
		if headerIncludeBrokenThirdparty != "" {
			selectedProxy = p.getProxyFromAllWithBroken()
		} else if headerBypassThirdparty != "" {
			selectedProxy = p.getProxyFromOurs()
		} else {
			selectedProxy = p.getProxyFromAll()
		}
	}
	if selectedProxy == "" {
		panic("selected proxy was empty!")
	}

	proxyUser, proxyPass, proxyHost, parsedProxyUrl, err := splitProxyURL(selectedProxy)
	if err != nil {
		errStr := "failed to parse downstream proxy assignment"
		http.Error(w, errStr, http.StatusBadRequest)
		return "", "", "", "", nil, errors.New(fmt.Sprintf(`%s: %s`, errStr, err.Error()))
	}

	return selectedProxy, proxyUser, proxyPass, proxyHost, parsedProxyUrl, nil

}

func (p *ForwardProxyCluster) proxyHttpConnect(w http.ResponseWriter, req *http.Request) {
	requestStartTime := time.Now()
	remoteAddr, _, _ := net.SplitHostPort(req.RemoteAddr)
	_, proxyUser, proxyPass, proxyHost, parsedProxyUrl, err := p.validateRequestAndGetProxy(w, req)
	if err != nil {
		// Error has already been handled, just log and return.
		log.Debugf(`%s -> %s -- HTTP -- Rejecting request: %s`, remoteAddr, proxyHost, err)
		return
	}
	var returnCode *int
	returnCode = new(int)
	*returnCode = -1
	defer logProxyRequest(remoteAddr, proxyHost, req.Host, returnCode, "HTTP", requestStartTime)

	parsedProxyUrl.Scheme = "http"
	if proxyUser != "" && proxyPass != "" {
		parsedProxyUrl.User = url.UserPassword(proxyUser, proxyPass)
	}
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(parsedProxyUrl),
		},
		Timeout: config.GetConfig().ProxyConnectTimeout,
	}

	proxyReq, err := http.NewRequest(req.Method, req.URL.String(), req.Body)
	if err != nil {
		log.Errorf(`Failed to make %s request to "%s": %s`, req.Method, req.URL.String(), err)
		http.Error(w, "failed to make request to downstream", http.StatusInternalServerError)
		return
	}

	copyHeader(proxyReq.Header, req.Header)
	proxyReq.Header.Set("X-Forwarded-For", req.RemoteAddr)

	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Errorf(`Failed to execute %s request to "%s": %s`, req.Method, req.URL.String(), err)
		http.Error(w, "failed to execute request to downstream", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	*returnCode = resp.StatusCode

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *ForwardProxyCluster) proxyHttpsConnect(w http.ResponseWriter, req *http.Request) {
	requestStartTime := time.Now()
	remoteAddr, _, _ := net.SplitHostPort(req.RemoteAddr)
	targetHost, _, _ := net.SplitHostPort(req.Host)
	_, proxyUser, proxyPass, proxyHost, _, err := p.validateRequestAndGetProxy(w, req)
	if err != nil {
		// Error has already been handled, just log and return.
		log.Debugf(`%s -> %s -- CONNECT -- Rejecting request: %s`, remoteAddr, proxyHost, err)
		return
	}
	var returnCode *int
	returnCode = new(int)
	*returnCode = -1
	defer logProxyRequest(remoteAddr, proxyHost, targetHost, returnCode, "CONNECT", requestStartTime)

	// Start a connection to the downstream proxy server.
	proxyConn, err := net.DialTimeout("tcp", proxyHost, config.GetConfig().ProxyConnectTimeout)
	if err != nil {
		log.Errorf(`Failed to dial proxy %s - %s`, proxyHost, err)
		http.Error(w, "failed to make request to downstream", http.StatusServiceUnavailable)
		return
	}

	// Proxy authentication
	auth := fmt.Sprintf("%s:%s", proxyUser, proxyPass)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	authHeader := "Proxy-Authorization: Basic " + encodedAuth

	// Send a new CONNECT request to the downstream proxy
	_, err = fmt.Fprintf(proxyConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n%s\r\n\r\n", req.Host, req.Host, authHeader)
	if err != nil {
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(proxyConn), req)
	if resp == nil {
		log.Errorf(`Failed to CONNECT to %s using proxy %s: %s`, req.Host, proxyHost, err)
		http.Error(w, "failed to execute request to downstream", http.StatusServiceUnavailable)
		return
	} else if err != nil {
		log.Warnf(`Error while performing CONNECT to %s using proxy %s: %s`, req.Host, proxyHost, err)
	}
	*returnCode = resp.StatusCode

	w.WriteHeader(http.StatusOK)
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Errorf(`Failed to forward connection to %s using proxy %s`, req.Host, proxyHost)
		http.Error(w, "failed to forward connection to downstream", http.StatusServiceUnavailable)
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Errorf(`Failed to execute connection forwarding to %s using proxy %s`, req.Host, proxyHost)
		http.Error(w, "failed to execute connection forwarding to downstream", http.StatusServiceUnavailable)
		return
	}

	go tunnelConn(proxyConn, clientConn)
	go tunnelConn(clientConn, proxyConn)
}

func tunnelConn(dst io.WriteCloser, src io.ReadCloser) {
	io.Copy(dst, src)
	dst.Close()
	src.Close()
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
