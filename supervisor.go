package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type limitCounter struct {
	limit   uint
	lock    sync.Mutex
	counter uint
}

func newLimitCounter(limit uint) *limitCounter {
	return &limitCounter{
		limit: limit,
		lock:  sync.Mutex{},
	}
}

func (counter *limitCounter) tryIncrement() bool {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	if counter.limit != 0 && counter.counter+1 > counter.limit {
		return false
	}

	counter.counter++

	return true
}

func (counter *limitCounter) decrement() {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	if counter.counter-1 < 0 {
		return
	}

	counter.counter--
}

type SupervisorConfig struct {
	serviceTerminationTimeout time.Duration
	limit                     uint
}

type Supervisor struct {
	servicesLock   sync.Mutex
	services       map[*Service]struct{}
	config         SupervisorConfig
	serviceCounter *limitCounter
}

func NewSupervisor(config SupervisorConfig) *Supervisor {
	return &Supervisor{
		servicesLock:   sync.Mutex{},
		services:       make(map[*Service]struct{}),
		config:         config,
		serviceCounter: newLimitCounter(config.limit),
	}
}

func (supervisor *Supervisor) registerService(service *Service) {
	supervisor.servicesLock.Lock()
	supervisor.services[service] = struct{}{}
	supervisor.servicesLock.Unlock()
}

func (supervisor *Supervisor) removeService(service *Service) bool {
	supervisor.servicesLock.Lock()
	_, exists := supervisor.services[service]
	delete(supervisor.services, service)
	supervisor.servicesLock.Unlock()
	return exists
}

func (supervisor *Supervisor) StartService(conn net.Conn, config serviceConfig) error {
	if !supervisor.serviceCounter.tryIncrement() {
		return fmt.Errorf("Service limit has been reached")
	}

	network := NewNetworkConnection(conn)
	service, err := StartService(config)

	if err != nil {
		network.Close()
		supervisor.serviceCounter.decrement()
		return err
	}

	supervisor.registerService(service)
	supervisor.runService(network, service)
	network.Close()
	supervisor.stopService(service)

	return nil
}

func (supervisor *Supervisor) runService(network *NetworkConnection, service *Service) {
	network.StartReading()
	service.StartReading()
	service.LogStderr()

	for {
		select {
		case networkMsg, ok := <-network.Output():
			if !ok {
				log.Printf("Network connection has been closed\n")
				return
			}

			service.Send(networkMsg)
		case serviceMsg, ok := <-service.Output():
			if !ok || !network.Send(serviceMsg) {
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

func (supervisor *Supervisor) Exit() <-chan struct{} {
	allStopped := make(chan struct{})

	go func() {
		var closeGroup errgroup.Group

		supervisor.servicesLock.Lock()
		for item, _ := range supervisor.services {
			service := item
			closeGroup.Go(func() error {
				supervisor.stopService(service)
				return nil
			})
		}
		supervisor.servicesLock.Unlock()

		closeGroup.Wait()
		close(allStopped)
	}()

	return allStopped
}

func (supervisor *Supervisor) stopService(service *Service) {
	<-service.Stop(supervisor.config.serviceTerminationTimeout)
	if supervisor.removeService(service) {
		supervisor.serviceCounter.decrement()
	}
}
