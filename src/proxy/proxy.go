package proxy

import (
	"github.com/sirupsen/logrus"
	"main/logging"
	"slices"
	"sync"
	"sync/atomic"
)

type ForwardProxyCluster struct {
	mu                        sync.RWMutex
	ourOnlineProxies          []string
	thirdpartyOnlineProxies   []string
	thirdpartyBrokenProxies   []string
	ipAddresses               []string
	BalancerOnline            WaitGroupCountable
	currentProxyAll           int32
	currentProxyOurs          int32
	currentProxyAllWithBroken int32
}

var log *logrus.Logger

func init() {
	log = logging.GetLogger()
}

func NewForwardProxyCluster() *ForwardProxyCluster {
	p := &ForwardProxyCluster{}
	atomic.StoreInt32(&p.currentProxyAll, 0)
	atomic.StoreInt32(&p.currentProxyOurs, 0)
	atomic.StoreInt32(&p.currentProxyAllWithBroken, 0)
	p.BalancerOnline.Add(1)
	return p
}

func (p *ForwardProxyCluster) cycleProxy(validProxies []string, currentProxy *int32) string {
	// Just round robin
	currProxy := atomic.LoadInt32(currentProxy)
	downstreamProxy := validProxies[currProxy]
	newCurrentProxy := (currProxy + 1) % int32(len(validProxies))
	atomic.StoreInt32(currentProxy, newCurrentProxy)
	return downstreamProxy
}

func (p *ForwardProxyCluster) getProxyFromAll() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	validProxies := removeDuplicates(append(p.ourOnlineProxies, p.thirdpartyOnlineProxies...))
	return p.cycleProxy(validProxies, &p.currentProxyAll)
}

func (p *ForwardProxyCluster) getProxyFromOurs() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	validProxies := p.ourOnlineProxies
	return p.cycleProxy(validProxies, &p.currentProxyOurs)
}

func (p *ForwardProxyCluster) getProxyFromAllWithBroken() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	validProxies := removeDuplicates(slices.Concat(p.ourOnlineProxies, p.thirdpartyBrokenProxies, p.thirdpartyOnlineProxies))
	return p.cycleProxy(validProxies, &p.currentProxyAllWithBroken)

}
