package live

import (
	"encoding/json"
	"log"

	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/models/chat"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/AugustLigh/GoMino/internal/ws"
	"github.com/AugustLigh/GoMino/pkg/config"
	"github.com/gofiber/fiber/v3"
)

var LiveKitConfig config.LiveKitConfig
var wsHub *ws.Hub

func SetConfig(cfg config.LiveKitConfig) {
	LiveKitConfig = cfg
}

func SetHub(hub *ws.Hub) {
	wsHub = hub
}

// WebSocket event types
const (
	WsEventLiveRoomStarted = 200 // Live room started
	WsEventLiveRoomEnded   = 201 // Live room ended
	WsEventLiveRoomUpdated = 202 // Live room updated (participant count, etc.)
)

// broadcastLiveRoomEvent sends a live room event to all thread subscribers
func broadcastLiveRoomEvent(threadID string, eventType int, room *chat.LiveRoom) {
	if wsHub == nil {
		return
	}

	event := map[string]interface{}{
		"t": eventType,
		"o": map[string]interface{}{
			"threadId": threadID,
			"liveRoom": room,
		},
	}

	data, _ := json.Marshal(event)
	wsHub.BroadcastToRoom(threadID, data)
}

// StartRoomRequest - запрос на создание комнаты
type StartRoomRequest struct {
	Title string `json:"title"`
	Type  int    `json:"type"` // 1 = voice, 2 = video
}

// StartRoom godoc
// @Summary Start a live room
// @Description Start a new voice or video room in a chat thread (CoHost/Host only)
// @Tags live
// @Accept json
// @Produce json
// @Param threadId path string true "Thread ID"
// @Param request body StartRoomRequest true "Room details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/live-room [post]
func StartRoom(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	log.Printf("[LiveRoom] StartRoom called: threadId=%s, auid=%s", threadId, auid)

	if auid == "" {
		log.Printf("[LiveRoom] Error: empty AUID")
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	var req StartRoomRequest
	body := c.Body()
	log.Printf("[LiveRoom] Request body: %s", string(body))

	if err := c.Bind().JSON(&req); err != nil {
		log.Printf("[LiveRoom] Bind error: %v, trying manual parse", err)
		// Fallback: try to get type from body directly
		var bodyMap map[string]interface{}
		if jsonErr := json.Unmarshal(body, &bodyMap); jsonErr == nil {
			if t, ok := bodyMap["type"].(float64); ok {
				req.Type = int(t)
			}
			if title, ok := bodyMap["title"].(string); ok {
				req.Title = title
			}
			log.Printf("[LiveRoom] Parsed manually: type=%d, title=%s", req.Type, req.Title)
		} else {
			log.Printf("[LiveRoom] Manual parse failed: %v", jsonErr)
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
		}
	}

	roomType := chat.LiveRoomType(req.Type)
	if roomType != chat.LiveRoomTypeVoice && roomType != chat.LiveRoomTypeVideo {
		log.Printf("[LiveRoom] Invalid room type %d, defaulting to voice", req.Type)
		roomType = chat.LiveRoomTypeVoice
	}

	title := req.Title
	if title == "" {
		if roomType == chat.LiveRoomTypeVoice {
			title = "Voice Chat"
		} else {
			title = "Video Chat"
		}
	}

	log.Printf("[LiveRoom] Creating room: type=%d, title=%s", roomType, title)

	liveService := service.NewLiveKitService(db, LiveKitConfig)
	room, token, err := liveService.StartRoom(threadId, auid, title, roomType)
	if err != nil {
		log.Printf("[LiveRoom] StartRoom error: %v", err)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"api:statuscode": 403,
			"api:message":    err.Error(),
		})
	}

	log.Printf("[LiveRoom] Room created successfully: roomId=%s", room.RoomID)

	// Broadcast to all thread subscribers
	broadcastLiveRoomEvent(threadId, WsEventLiveRoomStarted, room)

	// Notify all thread members about live room start
	go func() {
		notificationSvc := service.NewNotificationService(db, wsHub)
		_ = notificationSvc.NotifyThreadMembersAboutLiveRoom(threadId, room, auid)
	}()

	return c.JSON(fiber.Map{
		"liveRoom": room,
		"token":    token,
		"url":      liveService.GetLiveKitURL(),
	})
}

// JoinRoom godoc
// @Summary Join a live room
// @Description Join an existing live room
// @Tags live
// @Accept json
// @Produce json
// @Param threadId path string true "Thread ID"
// @Param roomId path string true "Room ID"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/live-room/{roomId}/join [post]
func JoinRoom(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	roomId := c.Params("roomId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	liveService := service.NewLiveKitService(db, LiveKitConfig)
	room, token, err := liveService.JoinRoom(roomId, auid)
	if err != nil {
		status := fiber.StatusForbidden
		if err.Error() == "room not found" {
			status = fiber.StatusNotFound
		}
		return c.Status(status).JSON(fiber.Map{
			"api:statuscode": status,
			"api:message":    err.Error(),
		})
	}

	// Broadcast participant count update to all thread subscribers
	if room != nil {
		broadcastLiveRoomEvent(room.ThreadID, WsEventLiveRoomUpdated, room)
	}

	return c.JSON(fiber.Map{
		"liveRoom": room,
		"token":    token,
		"url":      liveService.GetLiveKitURL(),
	})
}

// EndRoom godoc
// @Summary End a live room
// @Description End an active live room (CoHost/Host only)
// @Tags live
// @Accept json
// @Produce json
// @Param threadId path string true "Thread ID"
// @Param roomId path string true "Room ID"
// @Success 200 {object} nil
// @Failure 403 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/live-room/{roomId}/end [post]
func EndRoom(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	roomId := c.Params("roomId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	liveService := service.NewLiveKitService(db, LiveKitConfig)

	// Get room info before ending for broadcast
	room, _ := liveService.GetRoomByID(roomId)
	threadId := ""
	if room != nil {
		threadId = room.ThreadID
	}

	if err := liveService.EndRoom(roomId, auid); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"api:statuscode": 403,
			"api:message":    err.Error(),
		})
	}

	// Broadcast room ended
	if threadId != "" {
		broadcastLiveRoomEvent(threadId, WsEventLiveRoomEnded, room)
	}

	return c.JSON(fiber.Map{
		"api:statuscode": 0,
		"api:message":    "Room ended",
	})
}

// LockRoom godoc
// @Summary Lock/Unlock a live room
// @Description Lock or unlock a live room to prevent new participants
// @Tags live
// @Accept json
// @Produce json
// @Param threadId path string true "Thread ID"
// @Param roomId path string true "Room ID"
// @Success 200 {object} nil
// @Failure 403 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/live-room/{roomId}/lock [post]
func LockRoom(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	roomId := c.Params("roomId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	var req struct {
		Lock bool `json:"lock"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	liveService := service.NewLiveKitService(db, LiveKitConfig)
	if err := liveService.LockRoom(roomId, auid, req.Lock); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"api:statuscode": 403,
			"api:message":    err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"api:statuscode": 0,
		"api:message":    "Room lock updated",
	})
}

// GetActiveRoom godoc
// @Summary Get active live room
// @Description Get the currently active live room in a thread (if any)
// @Tags live
// @Accept json
// @Produce json
// @Param threadId path string true "Thread ID"
// @Success 200 {object} map[string]interface{}
// @Router /g/s/chat/thread/{threadId}/live-room [get]
func GetActiveRoom(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")

	liveService := service.NewLiveKitService(db, LiveKitConfig)
	room, err := liveService.GetActiveRoom(threadId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"liveRoom": room,
		"url":      liveService.GetLiveKitURL(),
	})
}

// LeaveRoom godoc
// @Summary Leave a live room
// @Description Leave a live room and auto-end if empty
// @Tags live
// @Accept json
// @Produce json
// @Param threadId path string true "Thread ID"
// @Param roomId path string true "Room ID"
// @Success 200 {object} nil
// @Router /g/s/chat/thread/{threadId}/live-room/{roomId}/leave [post]
func LeaveRoom(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	roomId := c.Params("roomId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	liveService := service.NewLiveKitService(db, LiveKitConfig)
	room, shouldEnd, err := liveService.LeaveRoom(roomId, auid)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"api:statuscode": 400,
			"api:message":    err.Error(),
		})
	}

	// Broadcast event to all thread subscribers
	if room != nil {
		if shouldEnd {
			// Room is now empty - broadcast end event
			broadcastLiveRoomEvent(room.ThreadID, WsEventLiveRoomEnded, room)
		} else {
			// Room still has participants - broadcast update event
			broadcastLiveRoomEvent(room.ThreadID, WsEventLiveRoomUpdated, room)
		}
	}

	return c.JSON(fiber.Map{
		"api:statuscode": 0,
		"api:message":    "Left room",
		"roomEnded":      shouldEnd,
	})
}
