package main

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/AndreBaltazar8/autorpc"
	"github.com/gorilla/websocket"
)

type ServerConfig struct {
	Host   string
	Port   uint16
	WSPort uint16
}

type Server interface {
	Start() error
	Clients() []Client
}

type serverImpl struct {
	service  autorpc.Service
	shutdown chan struct{}
	cfg      *ServerConfig
	clients  map[*clientImpl]bool
	nuntius  *nuntius
}

func checkConfig(cfg *ServerConfig) error {
	if cfg == nil {
		*cfg = ServerConfig{}
	}

	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}

	if cfg.Port == 0 {
		cfg.Port = 7331
	}

	if cfg.WSPort == 0 {
		cfg.WSPort = 7332
	}

	return nil
}

func NewServer(cfg *ServerConfig) (Server, error) {
	err := checkConfig(cfg)
	if err != nil {
		return nil, err
	}

	service := autorpc.NewServiceBuilder(&connectionAPI{}).EachConnectionAssign(&clientImpl{}, nil).UseRemote(clientRemoteAPI{}).Build()

	return &serverImpl{
		service:  service,
		shutdown: make(chan struct{}),
		cfg:      cfg,
		clients:  make(map[*clientImpl]bool),
		nuntius:  newNuntius(),
	}, nil
}

func (server *serverImpl) initWebSocketServer(addr string) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Println(err)
			return
		}

		client := newClient(server, newWebSocketConn(conn))
		server.addClient(client)
		go client.handleConnection()
	})

	fmt.Println("WebSocket server listening on", addr)
	go http.ListenAndServe(addr, nil)
}

func (server *serverImpl) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprint(server.cfg.Host, ":", server.cfg.Port))
	if err != nil {
		return fmt.Errorf("error listening: %s", err.Error())
	}

	defer listener.Close()

	fmt.Println("Listening on " + server.cfg.Host + ":" + strconv.Itoa(int(server.cfg.Port)))

	go func() {
		<-server.shutdown
		for client := range server.clients {
			fmt.Println("shutting down client")
			client.Close()
		}
	}()

	server.initWebSocketServer(fmt.Sprint(server.cfg.Host, ":", server.cfg.WSPort))
	for {
		conn, err := listener.Accept()
		if err != nil {
			close(server.shutdown)
			return fmt.Errorf("error accepting: %s", err.Error())
		}

		client := newClient(server, conn)
		server.addClient(client)
		go client.handleConnection()
	}
}

func (server *serverImpl) Clients() []Client {
	i := 0
	clients := make([]Client, len(server.clients))
	for k := range server.clients {
		clients[i] = k
		i++
	}
	return clients
}

func (server *serverImpl) addClient(client *clientImpl) {
	server.clients[client] = true
}
