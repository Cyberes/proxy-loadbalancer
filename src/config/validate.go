package config

import (
	"errors"
	"fmt"
	"strings"
)

func validateProxies(proxies []string) error {
	for _, proxy := range proxies {
		if !strings.HasPrefix(proxy, "http://") {
			return errors.New(fmt.Sprintf(`Proxy URLs must start with "http://" - "%s"`, proxy))
		}
	}
	return nil
}
