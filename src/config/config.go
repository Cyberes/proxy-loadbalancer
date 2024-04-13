package config

import (
	"errors"
	"github.com/spf13/viper"
	"time"
)

// The global, read-only config variable.
var cfg *Config

type Config struct {
	HTTPPort                string
	IpCheckerURL            string
	MaxProxyCheckers        int
	ProxyConnectTimeout     time.Duration
	ProxyPoolOurs           []string
	ProxyPoolThirdparty     []string
	ThirdpartyTestUrls      []string
	ThirdpartyBypassDomains []string
}

func SetConfig(configFile string) (*Config, error) {
	// Only allow the config to be set once.
	if cfg != nil {
		panic("Config has already been set!")
	}

	viper.SetConfigFile(configFile)
	viper.SetDefault("http_port", "5000")
	viper.SetDefault("proxy_checkers", 50)
	viper.SetDefault("proxy_connect_timeout", 10)
	viper.SetDefault("proxy_pool_ours", make([]string, 0))
	viper.SetDefault("proxy_pool_thirdparty", make([]string, 0))
	viper.SetDefault("thirdparty_test_urls", make([]string, 0))
	viper.SetDefault("thirdparty_bypass_domains", make([]string, 0))

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	config := &Config{
		HTTPPort:                viper.GetString("http_port"),
		IpCheckerURL:            viper.GetString("ip_checker_url"),
		MaxProxyCheckers:        viper.GetInt("proxy_checkers"),
		ProxyPoolOurs:           viper.GetStringSlice("proxy_pool_ours"),
		ProxyPoolThirdparty:     viper.GetStringSlice("proxy_pool_thirdparty"),
		ThirdpartyTestUrls:      viper.GetStringSlice("thirdparty_test_urls"),
		ThirdpartyBypassDomains: viper.GetStringSlice("thirdparty_bypass_domains"),
	}

	if config.IpCheckerURL == "" {
		return nil, errors.New("ip_checker_url is required")
	}

	timeout := viper.GetInt("proxy_connect_timeout")
	if timeout <= 0 {
		return nil, errors.New("proxy_connect_timeout must be greater than 0")
	}
	config.ProxyConnectTimeout = time.Duration(timeout) * time.Second

	proxyPoolOursErr := validateProxies(config.ProxyPoolOurs)
	if proxyPoolOursErr != nil {
		return nil, proxyPoolOursErr
	}

	proxyPoolThirdpartyErr := validateProxies(config.ProxyPoolThirdparty)
	if proxyPoolThirdpartyErr != nil {
		return nil, proxyPoolThirdpartyErr
	}

	cfg = config
	return config, nil
}

func GetConfig() *Config {
	if cfg == nil {
		panic("Config has not been set!")
	}
	return cfg
}
