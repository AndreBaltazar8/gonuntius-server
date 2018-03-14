package main

import (
	"bufio"
	"fmt"
	"net"

	"github.com/AndreBaltazar8/autorpc"
)

// Client interface
type Client interface {
	Close()
}

type clientImpl struct {
	srv           *serverImpl
	conn          net.Conn
	streamingMode bool
	streamingTo   *clientImpl
	rpcConn       autorpc.Connection
	nuntiusClient
}

func newClient(srv *serverImpl, conn net.Conn) *clientImpl {
	client := clientImpl{
		srv:           srv,
		conn:          conn,
		streamingMode: false,
		streamingTo:   nil,
	}
	return &client
}

func (client *clientImpl) Close() {
	client.conn.Close()
}

func (client *clientImpl) readSocket() {
	err := client.srv.service.HandleConnection(client.conn, func(conn autorpc.Connection) {
		err := conn.AssignValue(client)
		if err != nil {
			panic(fmt.Sprint("failed to assign client:", err))
		}

		client.rpcConn = conn
	})

	if err != nil {
		if rpcErr, ok := err.(*autorpc.RPCError); ok {
			fmt.Println("rpc error:", rpcErr.ActualErr)
		} else {
			fmt.Println(err)
		}

		client.Close()
		return
	}

	reader := bufio.NewReader(client.conn)
	buf := make([]byte, 1024)
	for client.streamingMode {
		len, err := reader.Read(buf)

		if err != nil {
			fmt.Println(err)
			client.Close()
			return
		}

		if len != 0 {
			client.streamingTo.conn.Write(buf[0:len])
		}
	}
}

func (client *clientImpl) handleConnection() {
	//client.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	fmt.Println("got connection from " + client.conn.RemoteAddr().String())
	client.readSocket()
}
