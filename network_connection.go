package main

import (
	"net"
)

const readSize = 65535

type NetworkConnection struct {
	conn   net.Conn
	output chan []byte
}

func NewNetworkConnection(conn net.Conn) *NetworkConnection {
	return &NetworkConnection{
		conn:   conn,
		output: make(chan []byte),
	}
}

func (connection *NetworkConnection) Output() <-chan []byte {
	return connection.output
}

func (connection *NetworkConnection) StartReading() {
	go func() {
		buff := make([]byte, readSize)

		for {
			numBytesRead, err := connection.conn.Read(buff)
			if err != nil {
				break
			}

			connection.output <- append(make([]byte, 0, numBytesRead), buff[:numBytesRead]...)
		}

		close(connection.output)
	}()
}

func (connection *NetworkConnection) Send(data []byte) bool {
	_, err := connection.conn.Write(data)

	return err == nil
}

func (connection *NetworkConnection) Close() {
	connection.conn.Close()
}
