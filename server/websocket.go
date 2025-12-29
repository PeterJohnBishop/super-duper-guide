package server

import (
	"encoding/json"
	"fmt"
	"log"
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

type Client struct {
	Conn *websocket.Conn
	Send chan []byte
}

type WebSocketEvent struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
	UserID  string `json:"user_id"`
}

func wsHandler(ctx *gin.Context) {
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}

	client := &Client{
		Conn: conn,
		Send: make(chan []byte, 256),
	}

	go client.writePump()
	go client.readPump()
}

// sits in a loop and waits for data to appear in the Send channel
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

// listens for incoming messages from the client
func (c *Client) readPump() {
	defer func() {
		close(c.Send)
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var event WebSocketEvent
		if err := json.Unmarshal(message, &event); err != nil {
			log.Printf("Error decoding JSON: %v", err)
			continue
		}
		fmt.Printf("Received Event: %s from User: %s\n", event.Type, event.UserID)

		switch event.Type {
		case "chat_message":
			log.Printf("Chat message from user %s: %s", event.UserID, event.Payload)
		case "status_update":
			log.Printf("Status update from user %s: %s", event.UserID, event.Payload)
		default:
			log.Printf("Unknown event type: %s", event.Type)
		}

		response := WebSocketEvent{
			Type:    "acknowledgement",
			Payload: "Event received",
			UserID:  event.UserID,
		}
		responseBytes, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error encoding JSON: %v", err)
			continue
		}

		c.Send <- responseBytes
	}
}
