package main

import (
	"flag"
	"fmt"
	"log"
	"main/proxy"
	"net/http"
	"os"
	"runtime/debug"
)

type cliConfig struct {
	configFile         string
	initialCrawl       bool
	debug              bool
	disableElasticSync bool
	help               bool
	version            bool
}

var Version = "development"
var VersionDate = "not set"

func main() {
	fmt.Println("=== Proxy Load Balancer ===")
	cliArgs := parseArgs()
	if cliArgs.help {
		flag.Usage()
		os.Exit(0)
	}
	if cliArgs.version {
		buildInfo, ok := debug.ReadBuildInfo()

		if ok {
			buildSettings := make(map[string]string)
			for i := range buildInfo.Settings {
				buildSettings[buildInfo.Settings[i].Key] = buildInfo.Settings[i].Value
			}
			fmt.Printf("Version: %s\n\n", Version)
			fmt.Printf("Date Compiled: %s\n", VersionDate)
			fmt.Printf("Git Revision: %s\n", buildSettings["vcs.revision"])
			fmt.Printf("Git Revision Date: %s\n", buildSettings["vcs.time"])
			fmt.Printf("Git Modified: %s\n", buildSettings["vcs.modified"])
		} else {
			fmt.Println("Build info not available")
		}
		os.Exit(0)
	}

	proxyCluster := proxy.NewForwardProxyCluster()
	go proxyCluster.ValidateProxiesThread()
	proxyCluster.BalancerOnline.Wait()
	go func() {
		log.Fatal(http.ListenAndServe(":5000", proxyCluster))
	}()
	fmt.Println("Server started!")
	select {}
}

func parseArgs() cliConfig {
	var cliArgs cliConfig
	flag.StringVar(&cliArgs.configFile, "config", "", "Path to the config file")
	flag.BoolVar(&cliArgs.debug, "d", false, "Enable debug mode")
	flag.BoolVar(&cliArgs.debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&cliArgs.version, "v", false, "Print version and exit")
	flag.Parse()
	return cliArgs
}
