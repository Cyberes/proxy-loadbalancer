package main

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"main/config"
	"main/logging"
	"main/proxy"
	"net/http"
	"os"
	"path/filepath"
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

	if cliArgs.debug {
		logging.InitLogger(logrus.DebugLevel)
	} else {
		logging.InitLogger(logrus.InfoLevel)
	}
	log := logging.GetLogger()
	log.Debugln("Initializing...")

	if cliArgs.configFile == "" {
		exePath, err := os.Executable()
		if err != nil {
			panic(err)
		}
		exeDir := filepath.Dir(exePath)

		if _, err := os.Stat(filepath.Join(exeDir, "config.yml")); err == nil {
			if _, err := os.Stat(filepath.Join(exeDir, "config.yaml")); err == nil {
				log.Fatalln("Both config.yml and config.yaml exist in the executable directory. Please specify one with the --config flag.")
			}
			cliArgs.configFile = filepath.Join(exeDir, "config.yml")
		} else if _, err := os.Stat(filepath.Join(exeDir, "config.yaml")); err == nil {
			cliArgs.configFile = filepath.Join(exeDir, "config.yaml")
		} else {
			log.Fatalln("No config file found in the executable directory. Please provide one with the --config flag.")
		}
	}
	configData, err := config.SetConfig(cliArgs.configFile)
	if err != nil {
		log.Fatalf(`Failed to load config: %s`, err)
	}

	proxyCluster := proxy.NewForwardProxyCluster()
	go proxyCluster.ValidateProxiesThread()
	proxyCluster.BalancerOnline.Wait()
	go func() {
		log.Fatal(http.ListenAndServe(":"+configData.HTTPPort, proxyCluster))
	}()
	log.Infof("-> Server started on 0.0.0.0:%s and accepting requests <-", configData.HTTPPort)
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
