package proxy

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
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
			if req.URL.Path == "/" {
				rand.New(rand.NewSource(time.Now().Unix()))
				fmt.Fprint(w, "proxy-loadbalancer <https://git.evulid.cc/cyberes/proxy-loadbalancer>\nSee /json for status info.\n\n\n\n"+retardation[rand.Intn(len(retardation))])
				return
			} else if req.URL.Path == "/json" {
				p.mu.RLock()
				response := map[string]interface{}{
					"uptime": int(math.Round(time.Since(startTime).Seconds())),
					"online": p.BalancerOnline.GetCount() == 0,
					"proxies": map[string]interface{}{
						"totalOnline": len(p.ourOnlineProxies) + len(p.thirdpartyOnlineProxies),
						"ours":        p.ourOnlineProxies,
						"thirdParty": map[string]interface{}{
							"online": p.thirdpartyOnlineProxies,
							"broken": p.thirdpartyBrokenProxies,
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
