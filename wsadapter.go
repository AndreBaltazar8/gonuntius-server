package main

import (
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type webSocketConn struct {
	conn *websocket.Conn
	rd   io.Reader
}

func newWebSocketConn(conn *websocket.Conn) *webSocketConn {
	wconn := webSocketConn{
		conn: conn,
		rd:   nil,
	}
	return &wconn
}

func (conn *webSocketConn) Read(b []byte) (n int, err error) {
	if conn.rd == nil {
		_, conn.rd, err = conn.conn.NextReader()
		if err != nil {
			return 0, err
		}
	}

	len := len(b)
	n, err = conn.rd.Read(b)
	for err == io.EOF {
		_, conn.rd, err = conn.conn.NextReader()
		if err != nil {
			return n, err
		}

		bleft := make([]byte, len-n)
		r, err := conn.rd.Read(bleft)
		if r > 0 {
			copy(b[n:n+r], bleft[0:r])
		}

		if err != nil && err != io.EOF {
			return n, err
		}
		n = n + r
	}
	return n, err
}

func (conn *webSocketConn) Write(b []byte) (n int, err error) {
	err = conn.conn.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (conn *webSocketConn) Close() error {
	return conn.conn.Close()
}

func (conn *webSocketConn) LocalAddr() net.Addr {
	return conn.conn.LocalAddr()
}
func (conn *webSocketConn) RemoteAddr() net.Addr {
	return conn.conn.RemoteAddr()
}
func (conn *webSocketConn) SetDeadline(t time.Time) error {
	err := conn.conn.SetReadDeadline(t)
	if err != nil {
		return err
	}

	return conn.conn.SetWriteDeadline(t)
}
func (conn *webSocketConn) SetReadDeadline(t time.Time) error {
	return conn.conn.SetReadDeadline(t)
}
func (conn *webSocketConn) SetWriteDeadline(t time.Time) error {
	return conn.conn.SetWriteDeadline(t)
}
