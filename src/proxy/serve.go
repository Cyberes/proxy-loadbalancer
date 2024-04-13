package proxy

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"time"
)

var startTime time.Time

func init() {
	startTime = time.Now()
}

func (p *ForwardProxyCluster) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodConnect {
		// HTTPS
		p.proxyHttpsConnect(w, req)
	} else {
		// HTTP
		if req.URL.Scheme == "" {
			// When the client connects using the server as a web server.
			remoteAddr, _, _ := net.SplitHostPort(req.RemoteAddr)
			defer log.Infof(`%s -- %s`, remoteAddr, req.URL.Path)
			if req.URL.Path == "/" {
				rand.New(rand.NewSource(time.Now().Unix()))
				fmt.Fprint(w, "proxy-loadbalancer <https://git.evulid.cc/cyberes/proxy-loadbalancer>\nSee /json for status info.\n\n\n\n"+retardation[rand.Intn(len(retardation))])
				return
			} else if req.URL.Path == "/json" {
				p.mu.RLock()
				response := map[string]interface{}{
					"uptime":            int(math.Round(time.Since(startTime).Seconds())),
					"online":            p.BalancerOnline && p.BalancerReady.GetCount() == 0,
					"refreshInProgress": p.refreshInProgress,
					"proxies": map[string]interface{}{
						"totalOnline": len(p.ourOnlineProxies) + len(p.thirdpartyOnlineProxies),
						"ours": map[string]interface{}{
							"online":  removeCredentials(p.ourOnlineProxies),
							"offline": removeCredentials(p.ourOfflineProxies),
						},
						"thirdParty": map[string]interface{}{
							"online":  removeCredentials(p.thirdpartyOnlineProxies),
							"broken":  removeCredentials(p.thirdpartyBrokenProxies),
							"offline": removeCredentials(p.thirdpartyOfflineProxies),
						},
					},
				}
				p.mu.RUnlock()
				jsonResponse, err := json.MarshalIndent(response, "", "  ")
				if err != nil {
					log.Errorln(err)
					http.Error(w, "Path not found", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Content-Type", "application/json")
				w.Write(jsonResponse)
				return
			} else {
				http.Error(w, "Path not found", http.StatusNotFound)
				return
			}
		} else {
			// When the client connects using the server as a proxy.
			p.proxyHttpConnect(w, req)
		}
	}
}

func removeCredentials(proxyURLs []string) []string {
	var newURLs []string
	for _, proxyURL := range proxyURLs {
		u, err := url.Parse(proxyURL)
		if err != nil {
			// Skip if invalid.
			continue
		}
		u.User = nil
		newURLs = append(newURLs, u.String())
	}
	if len(newURLs) == 0 {
		newURLs = make([]string, 0)
	}
	return newURLs
}
