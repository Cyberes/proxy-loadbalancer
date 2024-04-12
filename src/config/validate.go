package config

import (
	"errors"
	"fmt"
	"strings"
)

func ValidateProxies(proxies []string) error {
	for _, proxy := range proxies {
		if !strings.HasPrefix("http://", proxy) {
			return errors.New(fmt.Sprintf(`proxy "%s" must start with http://`, proxy))
		}
	}
	return nil
}
