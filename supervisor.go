package main

import (
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type Supervisor struct {
	servicesLock              sync.Mutex
	services                  map[*Service]struct{}
	serviceTerminationTimeout time.Duration
}

func NewSupervisor(serviceTerminationTimeout time.Duration) *Supervisor {
	return &Supervisor{
		servicesLock:              sync.Mutex{},
		services:                  make(map[*Service]struct{}),
		serviceTerminationTimeout: serviceTerminationTimeout,
	}
}

func (supervisor *Supervisor) addService(service *Service) {
	supervisor.servicesLock.Lock()
	supervisor.services[service] = struct{}{}
	supervisor.servicesLock.Unlock()
}

func (supervisor *Supervisor) removeService(service *Service) {
	supervisor.servicesLock.Lock()
	delete(supervisor.services, service)
	supervisor.servicesLock.Unlock()
}

func (supervisor *Supervisor) RunService(conn net.Conn, config serviceConfig) {
	netConn, err := NewNetworkConnection(conn)
	service, err := StartService(config)

	defer netConn.Stop()
	defer func() {
		log.Printf("Exiting runService\n")
		<-service.Stop(supervisor.serviceTerminationTimeout)
		supervisor.removeService(service)
		log.Printf("runService exited\n")
	}()

	if err != nil {
		log.Printf("Error starting service %s: %v\n", config.Name, err)
		return
	}

	supervisor.addService(service)

	netConn.StartReading()
	service.StartReading()
	service.LogStderr()

	for {
		select {
		case netMsg, ok := <-netConn.Output():
			if !ok {
				log.Printf("Network connection has been closed\n")
				return
			}

			service.Send(netMsg)
		case serviceMsg, ok := <-service.Output():
			if !ok || !netConn.Send(serviceMsg) {
				if !ok {
					log.Printf("Can't read data from service\n")
				} else {
					log.Printf("Network connection has been  closed\n")
				}

				return
			}
		}
	}
}

func (supervisor *Supervisor) Stop() <-chan struct{} {
	allStopped := make(chan struct{})

	go func() {
		var closeGroup errgroup.Group

		supervisor.servicesLock.Lock()
		log.Printf("In StopAll\n")
		for item, _ := range supervisor.services {
			service := item
			closeGroup.Go(func() error {
				<-service.Stop(supervisor.serviceTerminationTimeout)
				supervisor.removeService(service)

				return nil
			})
		}

		supervisor.servicesLock.Unlock()
		closeGroup.Wait()
		close(allStopped)
	}()

	return allStopped
}
