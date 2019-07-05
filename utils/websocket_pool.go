package utils

import (
	"sync"

	"github.com/gorilla/websocket"
)

type WebsocketPool struct {
	messageHandler WebsocketMessageHandler
	objectToConn   map[interface{}]*websocket.Conn
	connObjects    map[*websocket.Conn][]interface{}
	rwMutex        sync.RWMutex
}

type WebsocketMessageHandler func(int, []byte, error)

func NewWebsocketPool(messageHandler WebsocketMessageHandler) *WebsocketPool {
	return &WebsocketPool{
		messageHandler: messageHandler,
		objectToConn:   make(map[interface{}]*websocket.Conn),
		connObjects:    make(map[*websocket.Conn][]interface{}),
	}
}

func (pool *WebsocketPool) NewConn(url string, connectedObjects []interface{}) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			messageType, message, err := conn.ReadMessage()
			pool.messageHandler(messageType, message, err)
		}
	}()

	pool.rwMutex.Lock()
	defer pool.rwMutex.Unlock()

	pool.connObjects[conn] = connectedObjects
	for _, obj := range connectedObjects {
		pool.objectToConn[obj] = conn
	}

	return conn, err
}

func (pool *WebsocketPool) GetConnByObject(obj interface{}) (*websocket.Conn, bool) {
	pool.rwMutex.RLock()
	defer pool.rwMutex.RLock()
	conn, found := pool.objectToConn[obj]
	return conn, found
}
