package server

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/schollz/pake/v3"
	"golang.org/x/crypto/argon2"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type HubManager struct {
	rooms map[string]*Hub
	sync.RWMutex
}

func NewHubManager() *HubManager {
	return &HubManager{
		rooms: make(map[string]*Hub),
	}
}

func (hm *HubManager) GetOrCreateHub(id string) *Hub {
	hm.Lock()
	defer hm.Unlock()
	if hub, ok := hm.rooms[id]; ok {
		return hub
	}
	newHub := NewHub()
	go newHub.Run()
	hm.rooms[id] = newHub
	return newHub
}

type WebSocketEvent struct {
	Type string `json:"type"`
}

type UserMsg struct {
	Type      string `json:"type"`
	Status    string `json:"status"`
	UserID    string `json:"user_id"`
	Timestamp int64  `json:"timestamp"`
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	history    [][]byte
	userMap    map[string]*Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		history:    make([][]byte, 0),
		userMap:    make(map[string]*Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			for _, msg := range h.history {
				client.Send <- msg
			}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				exitEvent := UserMsg{
					Type:   "USER",
					Status: "disconnected",
					UserID: client.UserData.UserID,
				}
				payload, _ := json.Marshal(exitEvent)

				delete(h.clients, client)
				close(client.Send)

				go func() { h.broadcast <- payload }()
			}
		case message := <-h.broadcast:
			var ev WebSocketEvent
			if err := json.Unmarshal(message, &ev); err != nil {
				continue
			}

			if ev.Type == "MSG" {
				h.history = append(h.history, message)
				if len(h.history) > 100 {
					h.history = h.history[1:]
				}
			}

			if ev.Type == "USER" {
				var uMsg UserMsg
				if err := json.Unmarshal(message, &uMsg); err == nil {
					for c := range h.clients {

						if c.UserData.UserID == "" || c.UserData.UserID == "Unknown" {
							c.UserData = uMsg
						}
						if c.UserData.UserID == uMsg.UserID {
							c.UserData.Status = uMsg.Status
						}
					}
				}
			}

			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
		}
	}
}

type Client struct {
	Hub        *Hub
	Conn       *websocket.Conn
	Send       chan []byte
	UserData   UserMsg
	SessionKey []byte
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		c.Hub.broadcast <- message
	}
}

func (c *Client) writePump() {
	defer c.Conn.Close()

	for {
		message, ok := <-c.Send
		if !ok {
			c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}

func wsHandler(hubManager *HubManager, ctx *gin.Context) {
	roomID := ctx.Param("roomID")
	if roomID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Room ID is required"})
		return
	}
	hub := hubManager.GetOrCreateHub(roomID)

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}

	sharedSecret := ctx.GetHeader("X-Room-Password")
	if sharedSecret == "" {
		conn.Close()
		return
	}

	p, err := pake.InitCurve([]byte(sharedSecret), 0, "p256")
	if err != nil {
		conn.Close()
		return
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, p.Bytes()); err != nil {
		conn.Close()
		return
	}

	msgType, message, err := conn.ReadMessage()
	if err != nil || msgType != websocket.BinaryMessage {
		conn.Close()
		return
	}

	if err := p.Update(message); err != nil {
		conn.Close()
		return
	}

	if _, err = p.SessionKey(); err != nil {
		conn.Close()
		return
	}

	fixedSalt := []byte(SaltMaster)
	roomKey := argon2.IDKey([]byte(sharedSecret), fixedSalt, 1, 64*1024, 4, 32)

	client := &Client{
		Hub:        hub,
		Conn:       conn,
		Send:       make(chan []byte, 256),
		SessionKey: roomKey,
		UserData:   UserMsg{UserID: "Unknown"},
	}

	client.Hub.register <- client
	go client.readPump()
	go client.writePump()
}

func AddWebSocketRoutes(r *gin.Engine, hubManager *HubManager) {
	r.GET("/ws/:roomID", func(ctx *gin.Context) {
		wsHandler(hubManager, ctx)
	})
}
