package proxy

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

func (p *ForwardProxyCluster) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodConnect {
		// HTTPS
		p.proxyHttpsConnect(w, req)
	} else {
		// HTTP
		if req.URL.Scheme != "http" {
			//msg := fmt.Sprintf(`unsupported protocal "%s"`, req.URL.Scheme)
			//log.Errorf(msg)
			//http.Error(w, msg, http.StatusBadRequest)
			rand.New(rand.NewSource(time.Now().Unix()))
			fmt.Fprint(w, "proxy-loadbalancer\n<https://git.evulid.cc/cyberes/proxy-loadbalancer>\n\n"+retardation[rand.Intn(len(retardation))])
			return
		}
		p.proxyHttpConnect(w, req)
	}
}
