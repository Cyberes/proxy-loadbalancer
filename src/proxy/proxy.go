package proxy

import (
	"sync"
	"sync/atomic"
)

type ForwardProxyCluster struct {
	// TODO: mutex rwlock
	ourOnlineProxies        []string
	smartproxyOnlineProxies []string
	smartproxyBrokenProxies []string
	ipAddresses             []string
	BalancerOnline          sync.WaitGroup
	CurrentProxy            int32
}

// TODO: move all smartproxy things to "thirdparty"

func NewForwardProxyCluster() *ForwardProxyCluster {
	p := &ForwardProxyCluster{}
	atomic.StoreInt32(&p.CurrentProxy, 0)
	p.BalancerOnline.Add(1)
	return p
}

func (p *ForwardProxyCluster) getProxy() string {
	// Just round robin
	allValidProxies := append(p.ourOnlineProxies, p.smartproxyOnlineProxies...)
	currentProxy := atomic.LoadInt32(&p.CurrentProxy)
	downstreamProxy := allValidProxies[currentProxy]
	newCurrentProxy := (currentProxy + 1) % int32(len(testProxies))
	atomic.StoreInt32(&p.CurrentProxy, newCurrentProxy)
	return downstreamProxy
}
