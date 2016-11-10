package main

import (
	"log"
	"net"
)

const netReadSize = 65535

type NetworkConnection struct {
	conn      net.Conn
	output    chan []byte
	isStopped chan struct{}
}

func NewNetworkConnection(conn net.Conn) (*NetworkConnection, error) {
	return &NetworkConnection{
		conn:      conn,
		output:    make(chan []byte),
		isStopped: make(chan struct{}),
	}, nil
}

func (connection *NetworkConnection) Output() <-chan []byte {
	return connection.output
}

func (connection *NetworkConnection) StartReading() {
	go func() {
		buff := make([]byte, netReadSize)

		for {
			numBytesRead, err := connection.conn.Read(buff)
			if err != nil {
				break
			}

			connection.output <- buff[:numBytesRead]
		}

		close(connection.output)
	}()
}

func (connection *NetworkConnection) Send(data []byte) bool {
	_, err := connection.conn.Write(data)

	if err != nil {
		log.Printf("Error writing to network: %v\n", err)
	}

	return err == nil
}

func (connection *NetworkConnection) Stop() <-chan struct{} {
	connection.conn.Close()
	close(connection.isStopped)

	return connection.isStopped
}
