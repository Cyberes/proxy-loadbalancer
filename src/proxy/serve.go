package proxy

import (
	"fmt"
	"net/http"
)

func (p *ForwardProxyCluster) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodConnect {
		// HTTPS
		p.proxyHttpsConnect(w, req)
	} else {
		// HTTP
		if req.URL.Scheme != "http" {
			msg := fmt.Sprintf(`unsupported protocal "%s"`, req.URL.Scheme)
			log.Errorf(msg)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		p.proxyHttpConnect(w, req)
	}
}
