package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

type pendingConn struct {
	rejected             bool
	accepted             bool
	clientAccepted       *clientImpl
	clientAcceptedRemote *clientRemoteAPI
	idOne                []byte
	idTwo                []byte
}

type nuntius struct {
	clients            sync.Map
	pendingConnections sync.Map
}

func newNuntius() *nuntius {
	return &nuntius{}
}

func (nuntius *nuntius) getPendingConnection(connID []byte) (*pendingConn, bool) {
	val, ok := nuntius.pendingConnections.Load(hex.EncodeToString(connID))
	if !ok {
		return nil, false
	}
	if v, ok := val.(*pendingConn); ok {
		return v, true
	}
	panic("stored pending connection has wrong type")
}

func (nuntius *nuntius) getClientFor(appID []byte, id []byte) (*clientRemoteAPI, bool) {
	val, ok := nuntius.clients.Load(hex.EncodeToString(id))
	if !ok {
		return nil, false
	}
	if v, ok := val.(*clientRemoteAPI); ok {
		return v, true
	}
	panic("stored client has wrong type")
}

func (nuntius *nuntius) generateConnection() ([]byte, []byte, error) {
	connID := make([]byte, 32)
	connIDTwo := make([]byte, 32)
	pendingConn := pendingConn{}
	for {
		n, err := rand.Read(connID)
		if err != nil {
			log.Println("connection id generation error:", err)
			return nil, nil, errors.New("could not generate connection id")
		} else if n != 32 {
			return nil, nil, errors.New("could not generate connection id")
		}

		if _, exists := nuntius.pendingConnections.LoadOrStore(hex.EncodeToString(connID), &pendingConn); !exists {
			break
		}
	}

	for {
		n, err := rand.Read(connIDTwo)
		if err != nil {
			log.Println("connection id generation error:", err)
			return nil, nil, errors.New("could not generate connection id")
		} else if n != 32 {
			return nil, nil, errors.New("could not generate connection id")
		}

		if bytes.Compare(connID, connIDTwo) == 0 {
			continue
		}

		if _, exists := nuntius.pendingConnections.LoadOrStore(hex.EncodeToString(connIDTwo), &pendingConn); !exists {
			break
		}
	}

	pendingConn.idOne = connID
	pendingConn.idTwo = connIDTwo
	return connID, connIDTwo, nil
}

type nuntiusClient struct {
	authenticated bool
	appID         []byte
	publicID      []byte
}

type nuntiusInternalErr struct {
	Err string
}

func (e *nuntiusInternalErr) Error() string {
	return fmt.Sprintf("nuntius internal error: %s", e.Err)
}

type clientRemoteAPI struct {
	IncomingConnection   func(connID []byte, pendingID int, remoteID []byte, fn func(error))
	InitializeConnection func(connID []byte, fn func(error))
	ErrorConnection      func(connID []byte, err string, fn func(error))
}

type connectionAPI struct {
}

func (conn *connectionAPI) Version(version byte, client *clientImpl) error {
	client.conn.SetReadDeadline(time.Time{})
	return nil
}

func (conn *connectionAPI) Register(appID []byte, publicID []byte, registrationKey []byte) ([]byte, error) {
	secretKey := make([]byte, 32)
	n, err := rand.Read(secretKey)
	if err != nil || n != 32 {
		if err != nil {
			log.Printf("failed to generate key for app %v error: %s", appID, err)
		}
		return nil, &nuntiusInternalErr{"could not generate key"}
	}

	return secretKey, nil
}

func (conn *connectionAPI) Authenticate(appID []byte, publicID []byte, secretKey []byte, client *clientImpl, remote *clientRemoteAPI) error {
	client.nuntiusClient.authenticated = true
	client.nuntiusClient.publicID = publicID
	fmt.Println("storing", string(publicID))
	client.srv.nuntius.clients.Store(hex.EncodeToString(publicID), remote)
	return nil
}

func (conn *connectionAPI) ConnectTo(remoteID []byte, uniqueID int, client *clientImpl, remote *clientRemoteAPI) error {
	nuntius := client.srv.nuntius
	nuntiusClient := client.nuntiusClient
	if !nuntiusClient.authenticated {
		return errors.New("must be authenticated to connect to other user")
	}

	thisID := nuntiusClient.publicID
	clientOther, _ := nuntius.getClientFor(nuntiusClient.appID, remoteID)
	fmt.Println("looking for", string(remoteID))
	if clientOther == nil {
		return errors.New("could not connect - user not found")
	}

	connID, connIDTwo, err := nuntius.generateConnection()
	if err != nil {
		return err
	}

	remote.IncomingConnection(connID, uniqueID, thisID, func(err error) {
		if err != nil {
			fmt.Println("failed to deliver self", err)
		}
	})

	clientOther.IncomingConnection(connIDTwo, 0, thisID, func(err error) {
		if err != nil {
			fmt.Println("failed to deliver other", err)
		}
	})
	return nil
}

func (conn *connectionAPI) AcceptConnection(connID []byte, client *clientImpl, remote *clientRemoteAPI) error {
	nuntius := client.srv.nuntius
	pendingConn, exists := nuntius.getPendingConnection(connID)
	if !exists {
		return errors.New("unknown connection")
	}
	nuntius.pendingConnections.Delete(hex.EncodeToString(connID))
	if pendingConn.rejected {
		return errors.New("could not connect - rejected")
	}

	otherID := pendingConn.idOne
	if bytes.Compare(otherID, connID) == 0 {
		otherID = pendingConn.idTwo
	}
	if pendingConn.accepted {
		pendingConn.clientAcceptedRemote.InitializeConnection(otherID, func(error) {
			pendingConn.clientAccepted.streamingTo = client
			pendingConn.clientAccepted.streamingMode = true
			pendingConn.clientAccepted.rpcConn.StopHandling()
		})
		remote.InitializeConnection(connID, func(error) {
			client.streamingTo = pendingConn.clientAccepted
			client.streamingMode = true
			client.rpcConn.StopHandling()
		})
	} else {
		pendingConn.clientAccepted = client
		pendingConn.clientAcceptedRemote = remote
		pendingConn.accepted = true
	}
	return nil
}

func (conn *connectionAPI) RejectConnection(connID []byte, client *clientImpl) error {
	nuntius := client.srv.nuntius
	pendingConn, exists := nuntius.getPendingConnection(connID)
	if !exists {
		return errors.New("unknown connection")
	}
	nuntius.pendingConnections.Delete(hex.EncodeToString(connID))
	otherID := pendingConn.idOne
	if bytes.Compare(otherID, connID) == 0 {
		otherID = pendingConn.idTwo
	}
	if pendingConn.accepted {
		pendingConn.clientAcceptedRemote.ErrorConnection(otherID, "could not connect - rejected by second", func(error) {})
	} else {
		pendingConn.rejected = true
	}
	return nil
}
