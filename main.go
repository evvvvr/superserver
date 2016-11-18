package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	defaultConfigPath         = "superserver.toml"
	defaultTerminationTimeout = time.Millisecond * 3000
)

func main() {
	config := getConfig()
	if len(config.Services) == 0 {
		log.Printf("No services specified. Exiting.\n")
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	superserver := StartSuperserver(config)
	<-sigChan
	log.Printf("Stopping...\n")
	superserver.Stop()
	log.Printf("Stopped\n")
}

func getConfig() superserverConfig {
	configPath := flag.String("f", defaultConfigPath, "Config file path")
	terminationTimeout := flag.Duration("t", defaultTerminationTimeout, "Child services termination timeout")
	limit := flag.Uint("limit", 0, "Maximum number of concurrently running processes that can be started")
	flag.Parse()

	log.Printf("Reading config from file %s\n", *configPath)
	fileConfig, err := readConfigFromFile(*configPath)
	if err != nil {
		log.Fatalf("Error reading config: %v\n", err)
	}

	return superserverConfig{
		serviceTerminationTimeout: *terminationTimeout,
		limit:  *limit,
		config: fileConfig,
	}
}
