package ws

import (
	"strings"

	"github.com/AugustLigh/GoMino/internal/ctxutils"
	"github.com/AugustLigh/GoMino/pkg/jwt"
	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
)

// ServeWs handles websocket requests from the peer.
func ServeWs(hub *Hub) fiber.Handler {
	return func(c fiber.Ctx) error {
		// 1. Validate Headers or Query Params
		// headers = { "NDCDEVICEID": ..., "NDCAUTH": ..., "NDC-MSG-SIG": ... }
		// query = ?deviceId=...&sid=...
		
		deviceId := c.Get("NDCDEVICEID")
		if deviceId == "" {
			deviceId = c.Query("deviceId")
		}

		auth := c.Get("NDCAUTH")
		var sid string

		// Try header first
		if auth != "" {
			if strings.HasPrefix(auth, "sid=") {
				sid = strings.TrimPrefix(auth, "sid=")
			} else {
				sid = auth
			}
		} else {
			// Try query param
			sid = c.Query("sid")
			// Handle potential "sid=" prefix in query param too, just in case
			if strings.HasPrefix(sid, "sid=") {
				sid = strings.TrimPrefix(sid, "sid=")
			}
		}

		if sid == "" {
			return c.Status(fiber.StatusUnauthorized).SendString("Missing NDCAUTH or sid")
		}

		cfg := ctxutils.GetConfigFromContext(c)
		if cfg == nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Server Config Error")
		}

		claims, err := jwt.ValidateToken(sid, cfg.JWT.Secret)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).SendString("Invalid Token")
		}
        
        userID := claims.UserID

		// 2. Upgrade
		err = upgrader.Upgrade(c.RequestCtx(), func(conn *websocket.Conn) {
			client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256), UserID: userID, DeviceID: deviceId}
			client.hub.register <- client

			go client.writePump()
			client.readPump()
		})
        
        if err != nil {
            // Upgrade failed
            return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
        }
        
        return nil
	}
}
