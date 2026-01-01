package server

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
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
			json.Unmarshal(message, &ev)

			if ev.Type == "MSG" {
				h.history = append(h.history, message)
				if len(h.history) > 100 {
					h.history = h.history[1:]
				}
			}

			if ev.Type == "USER" {
				var uMsg UserMsg
				json.Unmarshal(message, &uMsg)
				for c := range h.clients {
					if c.UserData.UserID == uMsg.UserID {
						c.UserData.Status = uMsg.Status
						break
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
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	UserData UserMsg
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
			// The hub closed the channel.
			c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}

func wsHandler(hub *Hub, ctx *gin.Context) {
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}

	client := &Client{
		Hub:  hub,
		Conn: conn,
		Send: make(chan []byte, 256),
	}

	client.Hub.register <- client

	go client.readPump()
	go client.writePump()
}

func AddWebSocketRoutes(r *gin.Engine, hub *Hub) {
	r.GET("/ws", func(ctx *gin.Context) {
		wsHandler(hub, ctx)
	})
}
