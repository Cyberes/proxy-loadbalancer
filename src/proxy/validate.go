package proxy

import (
	"context"
	"fmt"
	"golang.org/x/sync/semaphore"
	"log"
	"math/rand"
	"slices"
	"sync"
	"time"
)

func (p *ForwardProxyCluster) ValidateProxiesThread() {
	log.Println("Doing initial backend check, please wait...")
	started := false

	// TODO: config value
	var sem = semaphore.NewWeighted(int64(50))
	ctx := context.TODO()

	for {
		// TODO: need to have these be temp vars and then copy them over when finished
		allProxies := removeDuplicates(append(testProxies, testSmartproxyPool...))
		p.ourOnlineProxies = make([]string, 0)
		p.smartproxyOnlineProxies = make([]string, 0)
		p.smartproxyBrokenProxies = make([]string, 0)
		p.ipAddresses = make([]string, 0)

		var wg sync.WaitGroup
		for _, pxy := range allProxies {
			wg.Add(1)
			// TODO: semaphore to limit active checks
			go func(pxy string) {
				defer wg.Done()

				if err := sem.Acquire(ctx, 1); err != nil {
					fmt.Printf("Failed to acquire semaphore: %v\n", err)
					return
				}
				defer sem.Release(1)

				// Test the proxy.
				ipAddr, testErr := sendRequestThroughProxy(pxy, testTargetUrl)
				if testErr != nil {
					fmt.Printf("Proxy %s failed: %s\n", pxy, testErr)
					return
				}
				if slices.Contains(p.ipAddresses, ipAddr) {
					fmt.Printf("Duplicate IP Address %s for proxy %s\n", ipAddr, pxy)
					return
				}
				p.ipAddresses = append(p.ipAddresses, ipAddr)

				// Sort the proxy into the right groups.
				if IsSmartproxy(pxy) {
					p.smartproxyOnlineProxies = append(p.smartproxyOnlineProxies, pxy)
					for _, d := range testSmartproxyBV3HIFix {
						_, bv3hiErr := sendRequestThroughProxy(pxy, d)
						if bv3hiErr != nil {
							fmt.Printf("Smartproxy %s failed: %s\n", pxy, bv3hiErr)
							p.smartproxyBrokenProxies = append(p.smartproxyBrokenProxies, pxy)
						}
					}
				} else {
					p.ourOnlineProxies = append(p.ourOnlineProxies, pxy)
				}
			}(pxy)
		}
		wg.Wait()

		if !started {
			p.ourOnlineProxies = shuffle(p.ourOnlineProxies)
			p.smartproxyOnlineProxies = shuffle(p.smartproxyOnlineProxies)
			started = true
			p.BalancerOnline.Done()
		}

		log.Printf("Our Endpoints Online: %d, Smartproxy Endpoints Online: %d, Smartproxy Broken Backends: %d, Total Online: %d\n",
			len(p.ourOnlineProxies), len(p.smartproxyOnlineProxies), len(p.smartproxyBrokenProxies), len(p.ourOnlineProxies)+(len(p.smartproxyOnlineProxies)-len(p.smartproxyBrokenProxies)))

		time.Sleep(60 * time.Second)
	}
}

func getKeysFromMap(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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
