package chat

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/models/chat"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/AugustLigh/GoMino/internal/ws"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

var Hub *ws.Hub

func SetHub(h *ws.Hub) {
	Hub = h
}

// Helper to get NDC ID from context or params
func getNdcId(c fiber.Ctx) int {
	// Try parsing from URL param (e.g. /x:comId/...)
	comIdStr := c.Params("comId")
	if comIdStr != "" {
		if id, err := strconv.Atoi(comIdStr); err == nil {
			return id
		}
	}
	// Try parsing from query param (e.g. ?ndcId=...) - common in Amino
	if ndcIdStr := c.Query("ndcId"); ndcIdStr != "" {
		if id, err := strconv.Atoi(ndcIdStr); err == nil {
			return id
		}
	}
	return 0 // Global
}

func getMemberRole(db *gorm.DB, threadId, userId string) (chat.MemberRole, error) {
	var member chat.ThreadMember
	if err := db.Where("thread_id = ? AND user_uid = ?", threadId, userId).First(&member).Error; err != nil {
		return -1, err
	}
	return member.Role, nil
}

// SendMessage godoc
// @Summary Send a message to a thread
// @Description Send a new message to a specific chat thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   request body SendMessageRequest true "Message details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/message [post]
// @Router /x{comId}/s/chat/thread/{threadId}/message [post]
func SendMessage(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	// Проверка: является ли пользователь участником чата
	if _, err := getMemberRole(db, threadId, auid); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	// Block check: if organizer blocked the sender, deny message
	chatService := service.NewChatService(db)
	userSvc := service.NewUserService(db)
	thread, err := chatService.GetThread(threadId, false)
	if err == nil && thread.UID != "" && thread.UID != auid {
		if userSvc.IsBlockedEither(thread.UID, auid) {
			return c.Status(fiber.StatusForbidden).JSON(response.NewErrorWithMessage(response.StatusNoPermission, "You are blocked by the organizer or have blocked them"))
		}
	}

	var req SendMessageRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	// Подготавливаем входные данные для сервиса
	input := service.SendMessageInput{
		Content:        req.Content,
		Type:           req.Type,
		MediaType:      req.MediaType,
		MediaValue:     req.MediaValue,
		StickerID:      req.StickerId,
		ReplyMessageID: req.ReplyMessageId,
		ClientRefID:    req.ClientRefId,
		Extensions:     req.Extensions,
	}

	// Извлекаем UID-ы упомянутых пользователей из extensions map
	if mentionedArray, ok := req.Extensions["mentionedArray"].([]interface{}); ok {
		uids := make([]string, 0, len(mentionedArray))
		for _, m := range mentionedArray {
			if mMap, ok := m.(map[string]interface{}); ok {
				if uid, ok := mMap["uid"].(string); ok {
					uids = append(uids, uid)
				}
			}
		}
		input.MentionedUIDs = uids
	}

	msg, err := chatService.SendMessage(threadId, auid, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Broadcast message via WS
	if Hub != nil {
		ndcId := getNdcId(c) // Capture NDC ID safely here
		go func() {
			members, err := chatService.GetAllMemberUIDs(threadId)
			if err == nil {
				// Amino message format for WS usually wraps the message object.
				// We'll send the raw message object for now as JSON.
				// Or follows a specific event structure: { "t": 1000, "o": { ...message... } }
				// Given "1:1 like Amino", I should probably approximate the event structure.
				// Amino uses type 1000 for chat messages.
				event := map[string]interface{}{
					"t": 1000,
					"o": map[string]interface{}{
						"chatMessage":      msg,
						"ndcId":            ndcId,
						"alertOption":      1, // Default?
						"membershipStatus": 1,
					},
				}
				
				jsonMsg, _ := json.Marshal(event)
				for _, uid := range members {
					Hub.BroadcastToUser(uid, jsonMsg)
				}

				// Get thread for push notifications
				var thread chat.Thread
				if err := db.First(&thread, "thread_id = ?", threadId).Error; err == nil {
					// Send push notifications to offline members
					chatService.SendMessagePushNotifications(msg, &thread, auid, Hub)
				}
			}
		}()
	}

	return c.JSON(fiber.Map{
		"message": msg,
	})
}

// DeleteMessage godoc
// @Summary Delete a message
// @Description Delete a specific message from a thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   messageId path string true "Message ID"
// @Success 200 {object} nil
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/message/{messageId} [delete]
// @Router /x{comId}/s/chat/thread/{threadId}/message/{messageId} [delete]
func DeleteMessage(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	messageId := c.Params("messageId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	// Проверка: является ли пользователь участником чата
	role, err := getMemberRole(db, threadId, auid)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	// Проверка прав: автор сообщения, помощник или организатор
	var message chat.Message
	if err := db.Preload("Author").Where("thread_id = ? AND message_id = ?", threadId, messageId).First(&message).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if message.UID != auid {
		if role != chat.MemberRoleHost && role != chat.MemberRoleCoHost {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
	}

	chatService := service.NewChatService(db)

	// Получаем имя автора для текста удаления
	authorName := "Пользователь"
	if message.Author.Nickname != "" {
		authorName = message.Author.Nickname
	}

	// Помечаем сообщение как удалённое (меняем type и content, сохраняем тот же ID)
	deletedMsg, err := chatService.MarkMessageAsDeleted(threadId, messageId, auid, authorName+" удалил сообщение")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Отправляем обновлённое сообщение по WebSocket
	if deletedMsg != nil && Hub != nil {
		ndcId := getNdcId(c)
		go func() {
			members, err := chatService.GetAllMemberUIDs(threadId)
			if err == nil {
				event := map[string]interface{}{
					"t": 1000,
					"o": map[string]interface{}{
						"chatMessage":      deletedMsg,
						"ndcId":            ndcId,
						"alertOption":      1,
						"membershipStatus": 1,
					},
				}
				jsonMsg, _ := json.Marshal(event)
				for _, uid := range members {
					Hub.BroadcastToUser(uid, jsonMsg)
				}
			}
		}()
	}

	return nil
}

// EditMessage godoc
// @Summary Edit a message
// @Description Edit the content of a specific message in a thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   messageId path string true "Message ID"
// @Param   request body EditMessageRequest true "New message content"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/message/{messageId} [post]
// @Router /x{comId}/s/chat/thread/{threadId}/message/{messageId} [post]
func EditMessage(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	messageId := c.Params("messageId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	// Проверка: является ли пользователь участником чата
	if _, err := getMemberRole(db, threadId, auid); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	var req EditMessageRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	if req.Content == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	chatService := service.NewChatService(db)
	editedMsg, err := chatService.EditMessage(threadId, messageId, auid, req.Content)
	if err != nil {
		if err.Error() == "permission denied: only the author can edit" {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err.Error() == "message not found" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusInvalidRequest))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Broadcast edited message via WS (type 1001 = message edited)
	if Hub != nil {
		ndcId := getNdcId(c)
		go func() {
			members, err := chatService.GetAllMemberUIDs(threadId)
			if err == nil {
				event := map[string]interface{}{
					"t": 1001,
					"o": map[string]interface{}{
						"chatMessage":      editedMsg,
						"ndcId":            ndcId,
						"alertOption":      1,
						"membershipStatus": 1,
					},
				}
				jsonMsg, _ := json.Marshal(event)
				for _, uid := range members {
					Hub.BroadcastToUser(uid, jsonMsg)
				}
			}
		}()
	}

	return c.JSON(fiber.Map{
		"message": editedMsg,
	})
}

// GetThreadInfo godoc
// @Summary Get thread info
// @Description Get details of a specific chat thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId} [get]
// @Router /x{comId}/s/chat/thread/{threadId} [get]
func GetThreadInfo(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)
	ndcId := getNdcId(c)

	chatService := service.NewChatService(db)
	thread, err := chatService.GetThread(threadId, true, auid)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
	}

	// Проверяем что тред принадлежит запрашиваемому контексту (глобал или сообщество)
	threadNdcId := 0
	if thread.NdcID != nil {
		threadNdcId = *thread.NdcID
	}
	if threadNdcId != ndcId {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
	}

	// Проверяем членство пользователя в чате
	if auid != "" {
		if _, err := getMemberRole(db, threadId, auid); err == nil {
			ms := 1
			thread.MembershipStatus = &ms
		}
	}

	// Для непубличных чатов требуется членство
	if thread.Type != nil && *thread.Type != 0 {
		if thread.MembershipStatus == nil || *thread.MembershipStatus != 1 {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
	}

	return c.JSON(fiber.Map{
		"thread": thread,
	})
}

// GetMembers godoc
// @Summary Get thread members
// @Description Get list of members in a chat thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   start query int false "Start offset"
// @Param   size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member [get]
// @Router /x{comId}/s/chat/thread/{threadId}/member [get]
func GetMembers(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	// Для непубличных чатов проверяем членство
	chatService := service.NewChatService(db)
	thread, err := chatService.GetThread(threadId, false)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
	}
	if thread.Type != nil && *thread.Type != 0 {
		if _, err := getMemberRole(db, threadId, auid); err != nil {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
	}

	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))

	members, err := chatService.GetMembers(threadId, start, size, getNdcId(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"memberList": members,
	})
}

// GetMessages godoc
// @Summary Get thread messages
// @Description Get list of messages in a chat thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   start query int false "Start offset"
// @Param   size query int false "Page size"
// @Param   before query string false "Get messages before this time (RFC3339)"
// @Param   around query string false "Get messages around this message ID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/message [get]
// @Router /x{comId}/s/chat/thread/{threadId}/message [get]
func GetMessages(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	// Для непубличных чатов проверяем членство
	chatService := service.NewChatService(db)
	thread, err := chatService.GetThread(threadId, false)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
	}
	if thread.Type != nil && *thread.Type != 0 {
		if _, err := getMemberRole(db, threadId, auid); err != nil {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
	}

	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))

	// Новые параметры для курсорной пагинации и контекста
	beforeStr := c.Query("before")
	aroundMsgId := c.Query("around")

	var beforeTime *time.Time
	if beforeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			beforeTime = &t
		}
	}

	var around *string
	if aroundMsgId != "" {
		around = &aroundMsgId
	}

	opts := service.GetMessagesOptions{
		Start:           start,
		Size:            size,
		NdcID:           getNdcId(c),
		BeforeTime:      beforeTime,
		AroundMessageID: around,
	}

	messages, err := chatService.GetMessages(threadId, opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"messageList": messages,
	})
}

// GetThreads godoc
// @Summary Get threads list
// @Description Get list of threads for the user or public threads
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   type query string false "Thread type (joined-all, public-all)"
// @Param   start query int false "Start offset"
// @Param   size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread [get]
// @Router /x{comId}/s/chat/thread [get]
func GetThreads(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	auid := middleware.GetAUIDFromContext(c)
	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	threadType := c.Query("type", "joined-all")
	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))

	chatService := service.NewChatService(db)
	ndcId := getNdcId(c)

	if threadType == "public-all" {
		filterType := c.Query("filterType", "recent")
		threads, err := chatService.GetPublicThreads(ndcId, filterType, start, size, auid)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
		}
		return c.JSON(fiber.Map{
			"threadList": threads,
		})
	}

	threads, err := chatService.ListUserThreads(auid, ndcId, start, size)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"threadList": threads,
	})
}

// JoinThread godoc
// @Summary Join a thread
// @Description Join a specific chat thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   userId path string true "User ID (usually self)"
// @Success 200 {object} nil
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/{userId} [post]
// @Router /x{comId}/s/chat/thread/{threadId}/member/{userId} [post]
func JoinThread(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	userId := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)

	// Получаем информацию о чате для проверки типа
	chatService := service.NewChatService(db)
	thread, err := chatService.GetThread(threadId, false)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
	}

	// Если userId не совпадает с auid, значит кто-то пытается добавить другого
	if userId != auid {
		role, err := getMemberRole(db, threadId, auid)
		if err != nil || (role != chat.MemberRoleHost && role != chat.MemberRoleCoHost) {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
	} else if thread.Type != nil && *thread.Type != 0 {
		// Для непубличных чатов запрещаем самостоятельный вход — только хост/со-хост может добавлять
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	// Block check: if organizer blocked the joining user, deny entry
	if thread.UID != "" && thread.UID != userId {
		userSvc := service.NewUserService(db)
		if userSvc.IsBlocked(thread.UID, userId) {
			return c.Status(fiber.StatusForbidden).JSON(response.NewErrorWithMessage(response.StatusNoPermission, "You are blocked by the organizer"))
		}
	}

	// Проверка: не является ли пользователь уже участником
	if _, err := getMemberRole(db, threadId, userId); err == nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if err := chatService.AddMember(threadId, userId, chat.MemberRoleMember); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Создаём системное сообщение о вступлении
	systemMsg, err := chatService.CreateSystemMessage(threadId, userId, chat.MessageTypeUserJoined, "")
	if err == nil && Hub != nil {
		ndcId := getNdcId(c)
		go func() {
			members, err := chatService.GetAllMemberUIDs(threadId)
			if err == nil {
				event := map[string]interface{}{
					"t": 1000,
					"o": map[string]interface{}{
						"chatMessage":      systemMsg,
						"ndcId":            ndcId,
						"alertOption":      1,
						"membershipStatus": 1,
					},
				}
				jsonMsg, _ := json.Marshal(event)
				for _, uid := range members {
					Hub.BroadcastToUser(uid, jsonMsg)
				}
			}
		}()
	}

	return nil
}

// DoNotDisturb godoc
// @Summary Set Do Not Disturb
// @Description Set alert options for a thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   userId path string true "User ID"
// @Param   request body map[string]int true "Alert Option"
// @Success 200 {object} nil
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/{userId}/alert [post]
// @Router /x{comId}/s/chat/thread/{threadId}/member/{userId}/alert [post]
func DoNotDisturb(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	userId := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)

	// В Amino можно менять настройки уведомлений только для себя
	if userId != auid {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	// Проверка: является ли пользователь участником
	if _, err := getMemberRole(db, threadId, auid); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	var req struct {
		AlertOption int `json:"alertOption"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	if err := db.Model(&chat.ThreadMember{}).
		Where("thread_id = ? AND user_uid = ?", threadId, userId).
		Update("alert_option", req.AlertOption).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// SetBackgroundImage godoc
// @Summary Set background image
// @Description Set background image for a thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   userId path string true "User ID"
// @Param   request body map[string]string true "Background Image URL"
// @Success 200 {object} nil
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/{userId}/background [post]
// @Router /x{comId}/s/chat/thread/{threadId}/member/{userId}/background [post]
func SetBackgroundImage(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	// Проверка прав: помощники, организатор, или любой участник ЛС
	role, err := getMemberRole(db, threadId, auid)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}
	if role != chat.MemberRoleHost && role != chat.MemberRoleCoHost {
		// Для ЛС (type=1) разрешаем любому участнику
		var t chat.Thread
		if err := db.Select("type").First(&t, "thread_id = ?", threadId).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		if t.Type == nil || *t.Type != 1 {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
	}

	var req struct {
		BackgroundImage string        `json:"backgroundImage"`
		Media           []interface{} `json:"media"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	// Получить текущий чат
	var thread chat.Thread
	if err := db.First(&thread, "thread_id = ?", threadId).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
	}

	// Инициализировать Extensions если nil
	if thread.Extensions == nil {
		thread.Extensions = &chat.ThreadExtensions{}
	}

	if len(req.Media) >= 2 {
		// Если пришел массив media, используем его напрямую
		thread.Extensions.BM = req.Media
	} else if req.BackgroundImage != "" {
		// Если пришла просто строка, формируем стандартный массив
		thread.Extensions.BM = []interface{}{
			100,
			req.BackgroundImage,
			nil,
		}
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	// Сохранить изменения
	if err := db.Model(&thread).Update("extensions", thread.Extensions).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// LeaveThread godoc
// @Summary Leave thread
// @Description Leave a chat thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   userId path string true "User ID"
// @Success 200 {object} nil
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/{userId} [delete]
// @Router /x{comId}/s/chat/thread/{threadId}/member/{userId} [delete]
func LeaveThread(c fiber.Ctx) error {
	if c.Query("allowRejoin") != "" {
		return KickMember(c)
	}

	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	userId := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)

	// Пользователь может покинуть чат только сам (userId == auid)
	// Если userId != auid, это должен быть KickMember
	if userId != auid {
		return KickMember(c)
	}

	// Проверка: является ли пользователь участником
	role, err := getMemberRole(db, threadId, userId)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusInvalidRequest))
	}

	chatService := service.NewChatService(db)

	// Если организатор покидает непубличный/публичный чат (не DM) — удаляем чат
	if role == chat.MemberRoleHost {
		thread, err := chatService.GetThread(threadId, false)
		if err == nil && (thread.Type == nil || *thread.Type == 0 || *thread.Type == 2) {
			// Собираем список участников до удаления для WS-уведомления
			if Hub != nil {
				ndcId := getNdcId(c)
				go func() {
					members, err := chatService.GetAllMemberUIDs(threadId)
					if err == nil {
						event := map[string]interface{}{
							"t": 1000,
							"o": map[string]interface{}{
								"ndcId":  ndcId,
								"chatMessage": map[string]interface{}{
									"threadId":  threadId,
									"type":      chat.MessageTypeUserLeft,
									"content":   "Chat deleted by organizer",
								},
								"alertOption":      1,
								"membershipStatus": 0,
							},
						}
						jsonMsg, _ := json.Marshal(event)
						for _, uid := range members {
							Hub.BroadcastToUser(uid, jsonMsg)
						}
					}
				}()
			}

			if err := chatService.DeleteThread(threadId, userId); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
			}
			return nil
		}
	}

	// Создаём системное сообщение о выходе ДО удаления из участников
	systemMsg, _ := chatService.CreateSystemMessage(threadId, userId, chat.MessageTypeUserLeft, "")

	if err := chatService.RemoveMember(threadId, userId); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Отправляем системное сообщение по WebSocket
	if systemMsg != nil && Hub != nil {
		ndcId := getNdcId(c)
		go func() {
			members, err := chatService.GetAllMemberUIDs(threadId)
			if err == nil {
				event := map[string]interface{}{
					"t": 1000,
					"o": map[string]interface{}{
						"chatMessage":      systemMsg,
						"ndcId":            ndcId,
						"alertOption":      1,
						"membershipStatus": 1,
					},
				}
				jsonMsg, _ := json.Marshal(event)
				for _, uid := range members {
					Hub.BroadcastToUser(uid, jsonMsg)
				}
			}
		}()
	}

	return nil
}

func KickMember(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	userId := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)

	// Нельзя кикнуть самого себя через этот метод (нужно использовать Leave)
	if userId == auid {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	// Проверка прав инициатора (auid)
	initiatorRole, err := getMemberRole(db, threadId, auid)
	if err != nil || (initiatorRole != chat.MemberRoleHost && initiatorRole != chat.MemberRoleCoHost) {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	// Проверка роли цели (userId)
	targetRole, err := getMemberRole(db, threadId, userId)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusInvalidRequest))
	}

	// Нельзя кикнуть организатора
	if targetRole == chat.MemberRoleHost {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	// Помощник не может кикнуть другого помощника
	if initiatorRole == chat.MemberRoleCoHost && targetRole == chat.MemberRoleCoHost {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	chatService := service.NewChatService(db)

	// Создаём системное сообщение о выходе (кик = выход) ДО удаления
	systemMsg, _ := chatService.CreateSystemMessage(threadId, userId, chat.MessageTypeUserLeft, "")

	if err := chatService.RemoveMember(threadId, userId); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Отправляем системное сообщение по WebSocket
	if systemMsg != nil && Hub != nil {
		ndcId := getNdcId(c)
		go func() {
			members, err := chatService.GetAllMemberUIDs(threadId)
			if err == nil {
				event := map[string]interface{}{
					"t": 1000,
					"o": map[string]interface{}{
						"chatMessage":      systemMsg,
						"ndcId":            ndcId,
						"alertOption":      1,
						"membershipStatus": 1,
					},
				}
				jsonMsg, _ := json.Marshal(event)
				for _, uid := range members {
					Hub.BroadcastToUser(uid, jsonMsg)
				}
			}
		}()
	}

	return nil
}

// SetCoHost godoc
// @Summary Set co-host
// @Description Assign co-host role to a member
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   userId path string true "User ID"
// @Success 200 {object} nil
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/{userId}/co-host [post]
// @Router /x{comId}/s/chat/thread/{threadId}/member/{userId}/co-host [post]
func SetCoHost(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	userId := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)

	// Проверка прав: только организатор может назначать помощников
	role, err := getMemberRole(db, threadId, auid)
	if err != nil || role != chat.MemberRoleHost {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	// Проверка: нельзя назначить человека помощником, если он уже помощник или организатор
	targetRole, err := getMemberRole(db, threadId, userId)
	if err == nil && (targetRole == chat.MemberRoleCoHost || targetRole == chat.MemberRoleHost) {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	chatService := service.NewChatService(db)
	if err := chatService.SetMemberRole(threadId, userId, chat.MemberRoleCoHost); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// RemoveCoHost godoc
// @Summary Remove co-host
// @Description Remove co-host role from a member
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   userId path string true "User ID"
// @Success 200 {object} nil
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/{userId}/co-host [delete]
// @Router /x{comId}/s/chat/thread/{threadId}/member/{userId}/co-host [delete]
func RemoveCoHost(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	userId := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)

	// Только организатор может убирать помощников
	role, err := getMemberRole(db, threadId, auid)
	if err != nil || role != chat.MemberRoleHost {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	// Проверка: является ли цель помощником
	targetRole, err := getMemberRole(db, threadId, userId)
	if err != nil || targetRole != chat.MemberRoleCoHost {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	chatService := service.NewChatService(db)
	if err := chatService.SetMemberRole(threadId, userId, chat.MemberRoleMember); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// CreateThread godoc
// @Summary Create a thread
// @Description Create a new chat thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   request body CreateThreadRequest true "Thread details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread [post]
// @Router /x{comId}/s/chat/thread [post]
func CreateThread(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)

	var req CreateThreadRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	auid := middleware.GetAUIDFromContext(c)
	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	// DM (type=1) требует ровно одного собеседника
	if req.Type == 1 {
		if len(req.InviteeUids) != 1 {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
		}
		// Нельзя создать DM с самим собой
		if req.InviteeUids[0] == auid {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
		}
		// Block check for DMs
		userSvc := service.NewUserService(db)
		if userSvc.IsBlockedEither(auid, req.InviteeUids[0]) {
			return c.Status(fiber.StatusForbidden).JSON(response.NewErrorWithMessage(response.StatusNoPermission, "Cannot create DM: user is blocked"))
		}
	}

	ndcId := getNdcId(c)

	chatService := service.NewChatService(db)

	// Для DM проверяем существующий чат
	if req.Type == 1 {
		existingThread, err := chatService.FindExistingDM(auid, req.InviteeUids[0], ndcId)
		if err == nil && existingThread != nil {
			return c.JSON(map[string]interface{}{
				"thread": existingThread,
			})
		}
	}

	newThread, err := chatService.CreateThread(auid, req.Title, req.Content, req.Icon, req.Type, ndcId, req.InviteeUids)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Notify followers about new public chat (type 0 or 2)
	if req.Type == 0 || req.Type == 2 {
		go func() {
			notificationSvc := service.NewNotificationService(db, Hub)
			_ = notificationSvc.NotifyFollowersAboutNewChat(auid, ndcId, newThread)
		}()
	}

	return c.JSON(map[string]interface{}{
		"thread": newThread,
	})
}

// UpdateThread godoc
// @Summary Update a thread
// @Description Update details of a chat thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   request body UpdateThreadRequest true "Update details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId} [post]
// @Router /x{comId}/s/chat/thread/{threadId} [post]
func UpdateThread(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	// Проверка прав: только помощники или организатор
	role, err := getMemberRole(db, threadId, auid)
	if err != nil || (role != chat.MemberRoleHost && role != chat.MemberRoleCoHost) {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	var req UpdateThreadRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	var thread chat.Thread
	if err := db.First(&thread, "thread_id = ?", threadId).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
	}

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.Keywords != nil {
		updates["keywords"] = *req.Keywords
	}

	// Обработка Extensions
	if thread.Extensions == nil {
		thread.Extensions = &chat.ThreadExtensions{}
	}

	extChanged := false
	if req.Extensions.Announcement != nil {
		thread.Extensions.Announcement = req.Extensions.Announcement
		extChanged = true
	}
	if req.Extensions.PinAnnouncement != nil {
		thread.Extensions.PinAnnouncement = req.Extensions.PinAnnouncement
		extChanged = true
	}
	if req.Extensions.FansOnly != nil {
		thread.Extensions.FansOnly = req.Extensions.FansOnly
		extChanged = true
	}
	if req.Extensions.Language != nil {
		thread.Extensions.Language = req.Extensions.Language
		extChanged = true
	}
	if req.Extensions.MembersCanInvite != nil {
		thread.Extensions.MembersCanInvite = req.Extensions.MembersCanInvite
		extChanged = true
	}
	if req.Extensions.BM != nil {
		thread.Extensions.BM = req.Extensions.BM
		extChanged = true
	}

	if extChanged {
		updates["extensions"] = thread.Extensions
	}

	if len(updates) > 0 {
		if err := db.Model(&thread).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
		}
	}

	return c.JSON(fiber.Map{
		"thread": thread,
	})
}

// InviteToThread godoc
// @Summary Invite to thread
// @Description Invite users to a chat thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   request body InviteRequest true "Invite details"
// @Success 200 {object} nil
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/invite [post]
// @Router /x{comId}/s/chat/thread/{threadId}/member/invite [post]
func InviteToThread(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	var thread chat.Thread
	if err := db.Where("thread_id = ?", threadId).First(&thread).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
	}

	// DM чаты (type == 1) — приглашения запрещены
	if thread.Type != nil && *thread.Type == 1 {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	// Проверка прав на приглашение: если membersCanInvite выключен, только помощники/организатор
	canInvite := true
	if thread.Extensions != nil && thread.Extensions.MembersCanInvite != nil {
		canInvite = *thread.Extensions.MembersCanInvite
	}

	if !canInvite {
		role, err := getMemberRole(db, threadId, auid)
		if err != nil || (role != chat.MemberRoleHost && role != chat.MemberRoleCoHost) {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
	}

	var req InviteRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if len(req.Uids) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	chatService := service.NewChatService(db)
	ndcId := getNdcId(c)
	for _, uid := range req.Uids {
		if uid == "" {
			continue
		}
		if err := chatService.AddMember(threadId, uid, chat.MemberRoleMember); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
		}

		// Создаём системное сообщение о вступлении
		systemMsg, err := chatService.CreateSystemMessage(threadId, uid, chat.MessageTypeUserJoined, "")
		if err == nil && Hub != nil {
			go func(invitedUID string) {
				members, err := chatService.GetAllMemberUIDs(threadId)
				if err == nil {
					event := map[string]interface{}{
						"t": 1000,
						"o": map[string]interface{}{
							"chatMessage":      systemMsg,
							"ndcId":            ndcId,
							"alertOption":      1,
							"membershipStatus": 1,
						},
					}
					jsonMsg, _ := json.Marshal(event)
					for _, memberUID := range members {
						Hub.BroadcastToUser(memberUID, jsonMsg)
					}
				}
			}(uid)
		}
	}

	return c.JSON(fiber.Map{
		"api:statuscode": 0,
		"api:message":    "OK",
	})
}

// DeleteThread godoc
// @Summary Delete thread
// @Description Delete a chat thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Success 200 {object} nil
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId} [delete]
// @Router /x{comId}/s/chat/thread/{threadId} [delete]
func DeleteThread(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	// Проверка прав: только организатор может удалить чат
	role, err := getMemberRole(db, threadId, auid)
	if err != nil || role != chat.MemberRoleHost {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	chatService := service.NewChatService(db)
	if err := chatService.DeleteThread(threadId, auid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// EnableViewOnlyMode godoc
// @Summary Enable view only
// @Description Enable view-only mode for a thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Success 200 {object} nil
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/view-only/enable [post]
// @Router /x{comId}/s/chat/thread/{threadId}/member/view-only/enable [post]
func EnableViewOnlyMode(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	// Проверка прав: только помощники или организатор
	role, err := getMemberRole(db, threadId, auid)
	if err != nil || (role != chat.MemberRoleHost && role != chat.MemberRoleCoHost) {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	chatService := service.NewChatService(db)
	updates := map[string]interface{}{
		"extensions": map[string]interface{}{
			"viewOnly": true,
		},
	}
	if err := chatService.UpdateThread(threadId, auid, updates); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// DisableViewOnlyMode godoc
// @Summary Disable view only
// @Description Disable view-only mode for a thread
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Success 200 {object} nil
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/view-only/disable [post]
// @Router /x{comId}/s/chat/thread/{threadId}/member/view-only/disable [post]
func DisableViewOnlyMode(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	// Проверка прав: только помощники или организатор
	role, err := getMemberRole(db, threadId, auid)
	if err != nil || (role != chat.MemberRoleHost && role != chat.MemberRoleCoHost) {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	chatService := service.NewChatService(db)
	updates := map[string]interface{}{
		"extensions": map[string]interface{}{
			"viewOnly": false,
		},
	}
	if err := chatService.UpdateThread(threadId, auid, updates); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// EnableCanInvite godoc
// @Summary Enable can invite
// @Description Allow members to invite others
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Success 200 {object} nil
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/members-can-invite/enable [post]
// @Router /x{comId}/s/chat/thread/{threadId}/member/members-can-invite/enable [post]
func EnableCanInvite(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	// Проверка прав: только помощники или организатор
	role, err := getMemberRole(db, threadId, auid)
	if err != nil || (role != chat.MemberRoleHost && role != chat.MemberRoleCoHost) {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	chatService := service.NewChatService(db)
	updates := map[string]interface{}{
		"extensions": map[string]interface{}{
			"membersCanInvite": true,
		},
	}
	if err := chatService.UpdateThread(threadId, auid, updates); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// DisableCanInvite godoc
// @Summary Disable can invite
// @Description Disallow members to invite others
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Success 200 {object} nil
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/members-can-invite/disable [post]
// @Router /x{comId}/s/chat/thread/{threadId}/member/members-can-invite/disable [post]
func DisableCanInvite(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	// Проверка прав: только помощники или организатор
	role, err := getMemberRole(db, threadId, auid)
	if err != nil || (role != chat.MemberRoleHost && role != chat.MemberRoleCoHost) {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	chatService := service.NewChatService(db)
	updates := map[string]interface{}{
		"extensions": map[string]interface{}{
			"membersCanInvite": false,
		},
	}
	if err := chatService.UpdateThread(threadId, auid, updates); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// TransferOrganizer godoc
// @Summary Transfer organizer
// @Description Transfer thread ownership to another member
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   request body map[string]string true "Target User UID"
// @Success 200 {object} nil
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/member/transfer-organizer [post]
// @Router /x{comId}/s/chat/thread/{threadId}/member/transfer-organizer [post]
func TransferOrganizer(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	// Проверка прав: только организатор может передать права
	role, err := getMemberRole(db, threadId, auid)
	if err != nil || role != chat.MemberRoleHost {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	var req struct {
		UID string `json:"uid"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	// Проверка: является ли цель участником чата
	if _, err := getMemberRole(db, threadId, req.UID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	chatService := service.NewChatService(db)
	if err := chatService.TransferOwnership(threadId, auid, req.UID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// MarkThreadAsRead godoc
// @Summary Mark thread as read
// @Description Mark all messages in a thread as read
// @Tags chat
// @Accept  json
// @Produce  json
// @Param   threadId path string true "Thread ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Success 200 {object} nil
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/chat/thread/{threadId}/mark-as-read [post]
// @Router /x{comId}/s/chat/thread/{threadId}/mark-as-read [post]
func MarkThreadAsRead(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	// Проверка: является ли пользователь участником
	if _, err := getMemberRole(db, threadId, auid); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	chatService := service.NewChatService(db)
	if err := chatService.MarkThreadAsRead(threadId, auid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// ==================== Moderation ====================

// HideThread godoc
// @Summary Hide a chat thread
// @Description Hide a chat thread from regular users (Curator+ required)
// @Tags chat-moderation
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   threadId path string true "Thread ID"
// @Param   request body ModerationRequest false "Reason for hiding"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/chat/thread/{threadId}/hide [post]
func HideThread(c fiber.Ctx) error {
	ndcId := getNdcId(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req ModerationRequest
	c.Bind().Body(&req)

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	if err := svc.HideThread(ndcId, auid, threadId, req.Reason); err != nil {
		if err == service.ErrPermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err.Error() == "thread not found" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Thread hidden",
	})
}

// UnhideThread godoc
// @Summary Unhide a chat thread
// @Description Make a hidden chat thread visible again (Curator+ required)
// @Tags chat-moderation
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   threadId path string true "Thread ID"
// @Param   request body ModerationRequest false "Reason for unhiding"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/chat/thread/{threadId}/unhide [post]
func UnhideThread(c fiber.Ctx) error {
	ndcId := getNdcId(c)
	threadId := c.Params("threadId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req ModerationRequest
	c.Bind().Body(&req)

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	if err := svc.UnhideThread(ndcId, auid, threadId, req.Reason); err != nil {
		if err == service.ErrPermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err.Error() == "thread not found" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Thread unhidden",
	})
}

// ModerationRequest for hide/unhide operations
type ModerationRequest struct {
	Reason string `json:"reason"`
}
