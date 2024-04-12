package proxy

import (
	"fmt"
	"log"
	"net/http"
)

func (p *ForwardProxyCluster) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodConnect {
		// HTTPS
		p.proxyHTTPSConnect(w, req)
	} else {
		// HTTP
		if req.URL.Scheme != "http" {
			msg := fmt.Sprintf(`unsupported protocal "%s"`, req.URL.Scheme)
			http.Error(w, msg, http.StatusBadRequest)
			log.Println(msg)
			return
		}
		p.proxyHttpConnect(w, req)
	}
}
