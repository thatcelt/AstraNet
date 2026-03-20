package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
)

// ClientMessage represents an incoming message from the client
type ClientMessage struct {
	Type        int    `json:"t"`                       // Message type
	CommunityID int    `json:"communityId,omitempty"`   // Community ID for presence
	TargetID    string `json:"targetId,omitempty"`      // Chat/Blog/Profile ID
	Nickname    string `json:"nickname,omitempty"`      // User's nickname for display
	Icon        string `json:"icon,omitempty"`          // User's icon for display
}

// Client message types
const (
	MsgTypePresenceOnline    = 300 // User is online (in community)
	MsgTypePresenceInChat    = 301 // User entered a chat
	MsgTypePresenceInBlog    = 302 // User is viewing a blog
	MsgTypePresenceInProfile = 303 // User is viewing a profile
	MsgTypePresenceLeave     = 304 // User left current view
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 51200 // 50KB
)

var upgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
		return true
	},
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// UserID associated with this client
	UserID string

	// DeviceID
	DeviceID string
}

// readPump pumps messages from the websocket connection to the hub.
// The application runs readPump in a per-connection goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// Parse incoming client message
		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to parse client message: %v", err)
			continue
		}

		// Handle presence updates
		c.handlePresenceMessage(msg)
	}
}

// handlePresenceMessage processes presence-related messages from the client
func (c *Client) handlePresenceMessage(msg ClientMessage) {
	switch msg.Type {
	case MsgTypePresenceOnline:
		// User is online in a community
		c.hub.Presence.UpdatePresence(c.UserID, msg.CommunityID, PresenceOnline, "", msg.Nickname, msg.Icon)
		log.Printf("User %s is now online in community %d", c.UserID, msg.CommunityID)

	case MsgTypePresenceInChat:
		// User entered a chat - subscribe to the chat room for live room events
		c.hub.Presence.UpdatePresence(c.UserID, msg.CommunityID, PresenceInChat, msg.TargetID, msg.Nickname, msg.Icon)
		// Subscribe to the chat room to receive live room events (start/end/update)
		if msg.TargetID != "" {
			c.hub.Subscribe(c, msg.TargetID)
			log.Printf("User %s subscribed to chat room %s", c.UserID, msg.TargetID)
		}
		log.Printf("User %s entered chat %s in community %d", c.UserID, msg.TargetID, msg.CommunityID)

	case MsgTypePresenceInBlog:
		// User is viewing a blog
		c.hub.Presence.UpdatePresence(c.UserID, msg.CommunityID, PresenceInBlog, msg.TargetID, msg.Nickname, msg.Icon)
		log.Printf("User %s is viewing blog %s in community %d", c.UserID, msg.TargetID, msg.CommunityID)

	case MsgTypePresenceInProfile:
		// User is viewing a profile
		c.hub.Presence.UpdatePresence(c.UserID, msg.CommunityID, PresenceInProfile, msg.TargetID, msg.Nickname, msg.Icon)
		log.Printf("User %s is viewing profile %s in community %d", c.UserID, msg.TargetID, msg.CommunityID)

	case MsgTypePresenceLeave:
		// User left current view, set back to just online
		// Get the previous targetID from presence to unsubscribe
		if prevPresence := c.hub.Presence.GetUserPresence(c.UserID); prevPresence != nil && prevPresence.TargetID != "" {
			c.hub.Unsubscribe(c, prevPresence.TargetID)
			log.Printf("User %s unsubscribed from room %s", c.UserID, prevPresence.TargetID)
		}
		c.hub.Presence.UpdatePresence(c.UserID, msg.CommunityID, PresenceOnline, "", msg.Nickname, msg.Icon)
		log.Printf("User %s left current view in community %d", c.UserID, msg.CommunityID)
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
