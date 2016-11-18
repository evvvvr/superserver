package main

import (
	"log"
	"net"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"
)

const listenerTimeout = time.Millisecond * 500

type superserverConfig struct {
	serviceTerminationTimeout time.Duration
	limit                     uint
	*config
}

type superserver struct {
	stopListening chan struct{}
	listenerGroup errgroup.Group
	supervisor    *Supervisor
}

func StartSuperserver(config superserverConfig) *superserver {
	supervisor := NewSupervisor(SupervisorConfig{
		serviceTerminationTimeout: config.serviceTerminationTimeout,
		limit: config.limit,
	})
	server := &superserver{
		stopListening: make(chan struct{}),
		listenerGroup: errgroup.Group{},
		supervisor:    supervisor,
	}

	for _, cfg := range config.Services {
		serviceConfig := cfg
		server.listenerGroup.Go(func() error {
			server.listenForServiceConnections(serviceConfig)

			return nil
		})
	}

	return server
}

func (server *superserver) Stop() {
	close(server.stopListening)
	server.listenerGroup.Wait()
	<-server.supervisor.Exit()
}

func (server *superserver) listenForServiceConnections(config serviceConfig) {
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
		case <-server.stopListening:
			return
		default:
		}

		listener.SetDeadline(time.Now().Add(listenerTimeout))
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
			if err := server.supervisor.StartService(conn, config); err != nil {
				conn.Close()
				log.Printf("Error starting service %s: %v\n", config.Name, err)
			}
		}()
	}
}
