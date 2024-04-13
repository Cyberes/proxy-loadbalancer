package proxy

import (
	"context"
	"golang.org/x/sync/semaphore"
	"main/config"
	"math/rand"
	"slices"
	"sync"
	"time"
)

func (p *ForwardProxyCluster) ValidateProxiesThread() {
	log.Infoln("Doing initial backend check, please wait...")
	started := false
	var sem = semaphore.NewWeighted(int64(config.GetConfig().MaxProxyCheckers))
	ctx := context.TODO()

	for {
		// TODO: need to have these be temp vars and then copy them over when finished
		allProxies := removeDuplicates(append(config.GetConfig().ProxyPoolOurs, config.GetConfig().ProxyPoolThirdparty...))
		p.ourOnlineProxies = make([]string, 0)
		p.thirdpartyOnlineProxies = make([]string, 0)
		p.thirdpartyBrokenProxies = make([]string, 0)
		p.ipAddresses = make([]string, 0)

		var wg sync.WaitGroup
		for _, pxy := range allProxies {
			wg.Add(1)
			// TODO: semaphore to limit active checks
			go func(pxy string) {
				defer wg.Done()

				if err := sem.Acquire(ctx, 1); err != nil {
					log.Errorf("Validate - failed to acquire semaphore: %v\n", err)
					return
				}
				defer sem.Release(1)

				_, _, proxyHost, _, err := splitProxyURL(pxy)
				if err != nil {
					log.Errorf(`Invalid proxy "%s"`, pxy)
					return
				}

				// Test the proxy.
				ipAddr, testErr := sendRequestThroughProxy(pxy, config.GetConfig().IpCheckerURL)
				if testErr != nil {
					log.Warnf("Validate - proxy %s failed: %s", proxyHost, testErr)
					return
				}
				if slices.Contains(p.ipAddresses, ipAddr) {
					log.Warnf("Validate - duplicate IP Address %s for proxy %s", ipAddr, proxyHost)
					return
				}
				p.ipAddresses = append(p.ipAddresses, ipAddr)

				// Sort the proxy into the right groups.
				if isThirdparty(pxy) {
					p.mu.Lock()
					p.thirdpartyOnlineProxies = append(p.thirdpartyOnlineProxies, pxy)
					p.mu.Unlock()

					for _, d := range config.GetConfig().ThirdpartyTestUrls {
						_, bv3hiErr := sendRequestThroughProxy(pxy, d)
						if bv3hiErr != nil {
							log.Debugf("Validate - Third-party %s failed: %s\n", proxyHost, bv3hiErr)
							p.thirdpartyBrokenProxies = append(p.thirdpartyBrokenProxies, pxy)
						}
					}
				} else {
					p.mu.Lock()
					p.ourOnlineProxies = append(p.ourOnlineProxies, pxy)
					p.mu.Unlock()
				}
			}(pxy)
		}
		wg.Wait()

		if !started {
			p.mu.Lock()
			p.ourOnlineProxies = shuffle(p.ourOnlineProxies)
			p.thirdpartyOnlineProxies = shuffle(p.thirdpartyOnlineProxies)
			p.mu.Unlock()
			started = true
			p.BalancerOnline.Done()
		}

		p.mu.RLock()
		log.Infof("Our Endpoints Online: %d, Third-Party Endpoints Online: %d, Third-Party Broken Endpoints: %d, Total Valid: %d\n",
			len(p.ourOnlineProxies), len(p.thirdpartyOnlineProxies), len(p.thirdpartyBrokenProxies), len(p.ourOnlineProxies)+(len(p.thirdpartyOnlineProxies)-len(p.thirdpartyBrokenProxies)))
		p.mu.RUnlock()

		time.Sleep(60 * time.Second)
	}
}

func shuffle(vals []string) []string {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ret := make([]string, len(vals))
	n := len(vals)
	for i := 0; i < n; i++ {
		randIndex := r.Intn(len(vals))
		ret[i] = vals[randIndex]
		vals = append(vals[:randIndex], vals[randIndex+1:]...)
	}
	return ret
}

func removeDuplicates(elements []string) []string {
	encountered := map[string]bool{}
	var result []string

	for v := range elements {
		if encountered[elements[v]] == true {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[elements[v]] = true
			// Append to result slice.
			result = append(result, elements[v])
		}
	}
	// Return the new slice.
	return result
}
