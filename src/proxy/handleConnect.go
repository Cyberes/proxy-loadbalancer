package proxy

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
)

func (p *ForwardProxyCluster) proxyHttpConnect(w http.ResponseWriter, req *http.Request) {
	proxyURLParsed, _ := url.Parse(p.getProxy())
	proxyURLParsed.Scheme = "http"

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURLParsed),
		},
	}
	proxyReq, err := http.NewRequest(req.Method, req.URL.String(), req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	copyHeader(proxyReq.Header, req.Header)
	proxyReq.Header.Set("X-Forwarded-For", req.RemoteAddr)

	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *ForwardProxyCluster) proxyHTTPSConnect(w http.ResponseWriter, req *http.Request) {
	log.Printf("CONNECT requested to %v (from %v)", req.Host, req.RemoteAddr)

	allValidProxies := append(p.ourOnlineProxies, p.smartproxyOnlineProxies...)
	currentProxy := atomic.LoadInt32(&p.CurrentProxy)
	downstreamProxy := allValidProxies[currentProxy]
	downstreamProxy = strings.Replace(downstreamProxy, "http://", "", -1)
	downstreamProxy = strings.Replace(downstreamProxy, "https://", "", -1)
	newCurrentProxy := (currentProxy + 1) % int32(len(testProxies))
	atomic.StoreInt32(&p.CurrentProxy, newCurrentProxy)

	// Connect to the downstream proxy server instead of the target host
	proxyConn, err := net.Dial("tcp", downstreamProxy)
	if err != nil {
		log.Println("failed to dial to proxy", downstreamProxy, err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Send a new CONNECT request to the downstream proxy
	_, err = fmt.Fprintf(proxyConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", req.Host, req.Host)
	if err != nil {
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(proxyConn), req)
	if err != nil || resp.StatusCode != 200 {
		log.Println("failed to CONNECT to target", req.Host)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Fatal("http server doesn't support hijacking connection")
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Fatal("http hijacking failed")
	}

	log.Println("tunnel established")
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
