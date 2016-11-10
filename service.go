package main

import (
	"bufio"
	"io"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

const (
	defaultExitTimeout = time.Millisecond * 500
	maxReadSize        = 65535
)

type Service struct {
	Name              string
	cmd               exec.Cmd
	stdin             io.WriteCloser
	stdout            io.ReadCloser
	stderr            io.ReadCloser
	output            chan []byte
	wasStopCalled     bool
	wasStopCalledLock sync.RWMutex
	isStopped         chan struct{}
}

func StartService(cfg serviceConfig) (*Service, error) {
	cmd := exec.Cmd{
		Path: cfg.Program,
		Env:  []string{},
		Args: cfg.ProgramArgs,
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	return &Service{
		Name:              cfg.Name,
		cmd:               cmd,
		stdin:             stdin,
		stdout:            stdout,
		stderr:            stderr,
		output:            make(chan []byte),
		wasStopCalledLock: sync.RWMutex{},
		isStopped:         make(chan struct{}),
	}, nil
}

func (service *Service) Stop(timeout time.Duration) <-chan struct{} {
	service.wasStopCalledLock.RLock()
	if service.wasStopCalled {
		log.Printf("service %s: already stopping", service.Name)
		service.wasStopCalledLock.RUnlock()
		return service.isStopped
	}

	service.wasStopCalledLock.RUnlock()
	service.wasStopCalledLock.Lock()
	service.wasStopCalled = true
	service.wasStopCalledLock.Unlock()

	log.Printf("service %s: stopping", service.Name)

	go func() {
		serviceExited := make(chan struct{})

		go func() {
			service.cmd.Wait()
			close(serviceExited)
		}()

		log.Printf("service %s: closing stdin\n", service.Name)
		service.stdin.Close()

		select {
		case <-serviceExited:
			log.Printf("service %s: Exited normally\n", service.Name)
			close(service.isStopped)
			return
		case <-time.After(defaultExitTimeout):
		}

		process := service.cmd.Process

		err := process.Signal(syscall.SIGTERM)
		if err != nil {
			log.Printf("service %s: Error sending SIGTERM to process %d: %v\n",
				service.Name, process.Pid, err)
		}

		select {
		case <-serviceExited:
			log.Printf("service %s: Process %d terminated after SIGTERM\n",
				service.Name, process.Pid)
			close(service.isStopped)
			return
		case <-time.After(timeout):
		}

		err = process.Signal(syscall.SIGKILL)
		if err != nil {
			log.Printf("service %s: Error sending SIGKILL to process %d: %v\n",
				service.Name, process.Pid, err)
			close(service.isStopped)
			return
		}

		select {
		case <-serviceExited:
			log.Printf("service %s: Process %d terminated after SIGKILL\n",
				service.Name, process.Pid)
			close(service.isStopped)
			return
		case <-time.After(defaultExitTimeout):
		}

		log.Printf("service %s: Process %d did not terminated after SIGKILL\n",
			service.Name, process.Pid)
		close(service.isStopped)
	}()

	return service.isStopped
}

func (service *Service) Send(data []byte) {
	service.stdin.Write(data)
}

func (service *Service) Output() <-chan []byte {
	return service.output
}

func (service *Service) StartReading() {
	go func() {
		buff := make([]byte, maxReadSize)

		for {
			numBytesRead, err := service.stdout.Read(buff)
			if err != nil {
				if err != io.EOF {
					log.Printf("Unexpected error while reading from service stdout: %v\n",
						err)
				}

				break
			}

			service.output <- buff[:numBytesRead]
		}

		close(service.output)
	}()
}

func (service *Service) LogStderr() {
	go func() {
		stderrReader := bufio.NewReader(service.stderr)

		for {
			msg, err := stderrReader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("Unexpected error while reading from service stderr: %v\n",
						err)
				}

				return
			}

			log.Printf("service %s: %s", service.Name, msg)
		}
	}()
}
