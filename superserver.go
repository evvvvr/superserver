package main

import (
	"flag"
	"golang.org/x/sync/errgroup"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	ListenerTimeout        = time.Millisecond * 500
	ServicesDefaultTimeout = time.Millisecond * 3000
)

type servicesList struct {
	services map[int]struct{}
	lock     sync.Mutex
}

func NewServicesList() *servicesList {
	return &servicesList{services: make(map[int]struct{})}
}

func (services *servicesList) Add(pid int ) {
	services.lock.Lock()
	services.services[pid] = struct{}{}
	services.lock.Unlock()
}

func (services *servicesList) Remove(pid int) {
	services.lock.Lock()
	delete(services.services, pid)
	services.lock.Unlock()
}

func (services *servicesList) Signal(signal syscall.Signal) {
	services.lock.Lock()
	for pid, _ := range services.services {
		log.Printf("Sending signal %d for process %d\n", signal, pid)
		if err := syscall.Kill(pid, signal); err != nil {
			log.Printf("Error sending signal %d for process %d: %v\n", signal, pid, err)
		}
	}
	services.lock.Unlock()
}

var listenerGroup errgroup.Group
var services *servicesList = NewServicesList()

func main() {
	port := flag.Int("p", -1, "Port number to listen to")
	e := flag.String("e", "", "Program to be executed")
	servicesTerminationTimeout := flag.Duration("t", ServicesDefaultTimeout, "Child services termination timeout")
	flag.Parse()

	if *port == -1 {
		log.Fatal("Specify port number")
	}

	if *e == "" {
		log.Fatal("Specify program to execute")
	}

	stopListening := make(chan struct{})
	listenerGroup.Go(func() error {
		listen(*port, *e, stopListening)
		return nil
	})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	close(stopListening)
	listenerGroup.Wait()
	log.Printf("Stopped accepting new connections\n")

	log.Printf("Terminating child services\n")
	services.Signal(syscall.SIGTERM)
	log.Printf("Waiting for child services to terminate\n")
	<-time.After(*servicesTerminationTimeout)
	log.Printf("Killing remaining child services\n")
	services.Signal(syscall.SIGKILL)

	log.Printf("Stopped\n")
}

func listen(port int, program string, stopListening <-chan struct{}) {
	addrString := ":" + strconv.Itoa(port)
	addr, err := net.ResolveTCPAddr("tcp", addrString)
	if err != nil {
		log.Fatalf("Error resolving address %s : %v", addrString, err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatalf("Error listening port %d: %v", port, err)
	}
	defer listener.Close()

	log.Printf("Listening on port %d\n", port)

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
			runProgram(conn, program)
		}()
	}
}

func runProgram(conn net.Conn, programPath string) error {
	defer conn.Close()

	cmd := exec.Cmd{
		Path:   programPath,
		Env:    []string{},
		Stdin:  conn,
		Stdout: conn,
		Stderr: conn,
	}

	err := cmd.Start()
	if err != nil {
		log.Printf("Error starting command: %v\n", err)
	}

	log.Printf("Child service pid is %d\n", cmd.Process.Pid)
	services.Add(cmd.Process.Pid)

	err = cmd.Wait()
	if err != nil {
		log.Printf("Error running command: %v\n", err)
	}

	log.Printf("Child service had stopped. Pid: %d\n", cmd.Process.Pid)
	services.Remove(cmd.Process.Pid)

	return err
}
