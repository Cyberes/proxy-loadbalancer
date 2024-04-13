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
		p.refreshInProgress = true
		allProxies := removeDuplicates(append(config.GetConfig().ProxyPoolOurs, config.GetConfig().ProxyPoolThirdparty...))
		newOurOnlineProxies := make([]string, 0)
		newOurOfflineProxies := make([]string, 0)
		newThirdpartyOnlineProxies := make([]string, 0)
		newThirdpartyBrokenProxies := make([]string, 0)
		newThirdpartyOfflineProxies := make([]string, 0)
		newIpAddresses := make([]string, 0)

		var wg sync.WaitGroup
		for _, pxy := range allProxies {
			wg.Add(1)
			go func(pxy string) {
				defer wg.Done()

				if err := sem.Acquire(ctx, 1); err != nil {
					log.Errorf("Validate - failed to acquire semaphore: %v", err)
					if isThirdparty(pxy) {
						newThirdpartyOfflineProxies = append(newThirdpartyOfflineProxies, pxy)
					} else {
						newOurOfflineProxies = append(newOurOfflineProxies, pxy)
					}
					return
				}
				defer sem.Release(1)

				_, _, proxyHost, _, err := splitProxyURL(pxy)
				if err != nil {
					log.Errorf(`Invalid proxy "%s"`, pxy)
					if isThirdparty(pxy) {
						newThirdpartyOfflineProxies = append(newThirdpartyOfflineProxies, pxy)
					} else {
						newOurOfflineProxies = append(newOurOfflineProxies, pxy)
					}
					return
				}

				// Test the proxy.
				ipAddr, testErr := sendRequestThroughProxy(pxy, config.GetConfig().IpCheckerURL)
				if testErr != nil {
					log.Warnf("Validate - proxy %s failed: %s", proxyHost, testErr)
					if isThirdparty(pxy) {
						newThirdpartyOfflineProxies = append(newThirdpartyOfflineProxies, pxy)
					} else {
						newOurOfflineProxies = append(newOurOfflineProxies, pxy)
					}
					return
				}
				if slices.Contains(newIpAddresses, ipAddr) {
					log.Warnf("Validate - duplicate IP Address %s for proxy %s", ipAddr, proxyHost)
					if isThirdparty(pxy) {
						newThirdpartyOfflineProxies = append(newThirdpartyOfflineProxies, pxy)
					} else {
						newOurOfflineProxies = append(newOurOfflineProxies, pxy)
					}
					return
				}
				newIpAddresses = append(newIpAddresses, ipAddr)

				// Sort the proxy into the right groups.
				if isThirdparty(pxy) {
					newThirdpartyOnlineProxies = append(newThirdpartyOnlineProxies, pxy)

					for _, d := range config.GetConfig().ThirdpartyTestUrls {
						_, bv3hiErr := sendRequestThroughProxy(pxy, d)
						if bv3hiErr != nil {
							log.Debugf("Validate - Third-party %s failed: %s", proxyHost, bv3hiErr)
							newThirdpartyBrokenProxies = append(newThirdpartyBrokenProxies, pxy)
						}
					}
				} else {
					newOurOnlineProxies = append(newOurOnlineProxies, pxy)
				}
			}(pxy)
		}
		wg.Wait()

		p.mu.Lock()
		p.ourOnlineProxies = removeDuplicates(newOurOnlineProxies)
		p.ourOfflineProxies = newOurOfflineProxies
		p.thirdpartyOnlineProxies = removeDuplicates(newThirdpartyOnlineProxies)
		p.thirdpartyBrokenProxies = removeDuplicates(newThirdpartyBrokenProxies)
		p.thirdpartyOfflineProxies = newThirdpartyOfflineProxies
		p.ipAddresses = removeDuplicates(newIpAddresses)
		p.BalancerOnline = len(slices.Concat(p.ourOnlineProxies, p.thirdpartyOnlineProxies, p.thirdpartyBrokenProxies)) > 0 // Online only if there are active and online proxies.
		p.mu.Unlock()

		if config.GetConfig().ShuffleProxies {
			p.mu.Lock()
			p.ourOnlineProxies = shuffle(p.ourOnlineProxies)
			p.thirdpartyOnlineProxies = shuffle(p.thirdpartyOnlineProxies)
			p.mu.Unlock()
		}

		if !started {
			started = true
			p.BalancerReady.Done()
		}

		p.mu.RLock()
		log.Infof("Our Endpoints Online: %d, Third-Party Endpoints Online: %d, Third-Party Broken Endpoints: %d, Total Valid: %d",
			len(p.ourOnlineProxies), len(p.thirdpartyOnlineProxies), len(p.thirdpartyBrokenProxies), len(p.ourOnlineProxies)+(len(p.thirdpartyOnlineProxies)-len(p.thirdpartyBrokenProxies)))
		p.mu.RUnlock()

		p.refreshInProgress = false
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
			encountered[elements[v]] = true
			result = append(result, elements[v])
		}
	}
	return result
}
