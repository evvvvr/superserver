package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	DefaultConfigPath         = "superserver.toml"
	ListenerTimeout           = time.Millisecond * 500
	DefaultTerminationTimeout = time.Millisecond * 3000
)

func main() {
	configPath := flag.String("f", DefaultConfigPath, "Config file path")
	terminationTimeout := flag.Duration("t", DefaultTerminationTimeout, "Child services termination timeout")
	flag.Parse()

	log.Printf("Reading config from file %s\n", *configPath)
	config, err := readConfigFromFile(*configPath)
	if err != nil {
		log.Fatalf("Error reading config: %v\n", err)
	}

	if len(config.Services) == 0 {
		log.Printf("No services specified. Exiting.\n")
		return
	}

	supervisor := NewSupervisor(*terminationTimeout)
	stopListening := make(chan struct{})
	listenerGroup := errgroup.Group{}

	for _, cfg := range config.Services {
		serviceConfig := cfg
		listenerGroup.Go(func() error {
			listen(supervisor, serviceConfig, stopListening)

			return nil
		})
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Printf("Stopping...\n")

	close(stopListening)
	listenerGroup.Wait()

	<-supervisor.Stop()
	log.Printf("Stopped\n")
}

func listen(supervisor *Supervisor, config serviceConfig, stopListening <-chan struct{}) {
	addrString := ":" + strconv.Itoa(config.Port)
	addr, err := net.ResolveTCPAddr("tcp", addrString)
	if err != nil {
		log.Fatalf("Error resolving address %s : %v", addrString, err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatalf("Error listening port %d: %v", config.Port, err)
	}
	defer listener.Close()

	log.Printf("Listening on port %d for service %s\n", config.Port, config.Name)

	for {
		select {
		case <-stopListening:
			return
		default:
		}

		listener.SetDeadline(time.Now().Add(ListenerTimeout))
		conn, err := listener.Accept()
		if err != nil {
			if netErr, ok := err.(*net.OpError); ok && netErr.Timeout() {
				continue
			}

			log.Printf("Error accepting connection: %v\n", err)
			continue
		}

		log.Printf("Connection has been accepted. Starting child service.\n")
		go func() {
			supervisor.RunService(conn, config)
		}()
	}
}
