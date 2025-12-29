package server

import (
	"encoding/json"
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
	Type string `json:"type"`
}

type MessageEvent struct {
	Content   string `json:"content"`
	UserID    string `json:"user_id"`
	Timestamp int64  `json:"timestamp"`
}

type StatusUpdateEvent struct {
	Status    string `json:"status"`
	UserID    string `json:"user_id"`
	Timestamp int64  `json:"timestamp"`
}

type CommandEvent struct {
	Command string          `json:"command"`
	Data    json.RawMessage `json:"data"`
}

type AcknowledgementEvent struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
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

		var response AcknowledgementEvent

		switch event.Type {
		case "MESSAGE":
			var messageEvent MessageEvent
			if err := json.Unmarshal(message, &messageEvent); err != nil {
				log.Printf("Error decoding MESSAGE event: %v", err)
				continue
			}
			log.Printf("Received MESSAGE event: %+v", messageEvent)
			response = AcknowledgementEvent{
				Type:    "acknowledgement",
				Payload: "Event received",
			}
		case "STATUS":
			var statusEvent StatusUpdateEvent
			if err := json.Unmarshal(message, &statusEvent); err != nil {
				log.Printf("Error decoding STATUS event: %v", err)
				continue
			}
			log.Printf("Received STATUS event: %+v", statusEvent)
			response = AcknowledgementEvent{
				Type:    "acknowledgement",
				Payload: "Event received",
			}
		case "COMMAND":
			var commandEvent CommandEvent
			if err := json.Unmarshal(message, &commandEvent); err != nil {
				log.Printf("Error decoding COMMAND event: %v", err)
				continue
			}
			// switch through commandEvent.Command to act on different commands
			log.Printf("Received COMMAND event: %+v", commandEvent)
			response = AcknowledgementEvent{
				Type:    "acknowledgement",
				Payload: "Event received",
			}
		default:
			log.Printf("Unknown event type: %s", event.Type)
			response = AcknowledgementEvent{
				Type:    "acknowledgement",
				Payload: "Event received",
			}
		}

		responseBytes, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error encoding JSON: %v", err)
			continue
		}

		c.Send <- responseBytes
	}
}
