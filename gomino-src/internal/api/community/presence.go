package community

import (
	"strconv"

	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/AugustLigh/GoMino/internal/ws"
	"github.com/gofiber/fiber/v3"
)

var Hub *ws.Hub

func SetHub(h *ws.Hub) {
	Hub = h
}

// GetOnlineActivity godoc
// @Summary Get community online activity
// @Description Get online users count and top active chats in a community
// @Tags community
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   limit query int false "Limit for top chats and online users (default 10)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/community/online-activity [get]
func GetOnlineActivity(c fiber.Ctx) error {
	comIdStr := c.Params("comId")
	ndcId, err := strconv.Atoi(comIdStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	if limit > 50 {
		limit = 50
	}

	if Hub == nil || Hub.Presence == nil {
		return c.JSON(fiber.Map{
			"onlineCount":    0,
			"topChats":       []interface{}{},
			"topBlogs":       []interface{}{},
			"onlineMembers":  []interface{}{},
		})
	}

	activity := Hub.Presence.GetCommunityActivity(ndcId, limit)

	db := middleware.GetDBFromContext(c)
	chatSvc := service.NewChatService(db)
	blogSvc := service.NewBlogService(db)
	userSvc := service.NewUserService(db)

	// Enrich top chats with thread info from database
	enrichedChats := make([]fiber.Map, 0, len(activity.TopChats))
	for _, chat := range activity.TopChats {
		threadInfo, err := chatSvc.GetThread(chat.ChatID, false)
		chatData := fiber.Map{
			"threadId":    chat.ChatID,
			"activeCount": chat.ActiveCount,
		}
		if err == nil && threadInfo != nil {
			chatData["title"] = threadInfo.Title
			chatData["icon"] = threadInfo.Icon
		}
		enrichedChats = append(enrichedChats, chatData)
	}

	// Enrich top blogs with blog info from database
	enrichedBlogs := make([]fiber.Map, 0, len(activity.TopBlogs))
	for _, blog := range activity.TopBlogs {
		blogInfo, err := blogSvc.GetBlog(blog.BlogID)
		blogData := fiber.Map{
			"blogId":      blog.BlogID,
			"activeCount": blog.ActiveCount,
		}
		if err == nil && blogInfo != nil {
			blogData["title"] = blogInfo.Title
			if blogInfo.MediaList != nil && len(*blogInfo.MediaList) > 0 {
				blogData["icon"] = (*blogInfo.MediaList)[0]
			}
		}
		enrichedBlogs = append(enrichedBlogs, blogData)
	}

	// Get full user profiles for online users
	onlineMembers := make([]fiber.Map, 0, len(activity.OnlineUsers))
	for _, onlineUser := range activity.OnlineUsers {
		userProfile, err := userSvc.GetUserByID(onlineUser.UserID, ndcId, true)
		if err == nil && userProfile != nil {
			onlineMembers = append(onlineMembers, fiber.Map{
				"uid":          userProfile.UID,
				"nickname":     userProfile.Nickname,
				"icon":         userProfile.Icon,
				"level":        userProfile.Level,
				"onlineStatus": 1,
			})
		}
	}

	return c.JSON(fiber.Map{
		"onlineCount":    activity.OnlineCount,
		"topChats":       enrichedChats,
		"topBlogs":       enrichedBlogs,
		"onlineMembers":  onlineMembers,
	})
}

// GetChatOnlineCount godoc
// @Summary Get chat online count
// @Description Get the number of users currently in a specific chat
// @Tags community
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   chatId path string true "Chat/Thread ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /x{comId}/s/chat/thread/{chatId}/online [get]
func GetChatOnlineCount(c fiber.Ctx) error {
	chatId := c.Params("threadId")
	if chatId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	count := 0
	if Hub != nil && Hub.Presence != nil {
		count = Hub.Presence.GetChatOnlineCount(chatId)
	}

	return c.JSON(fiber.Map{
		"onlineCount": count,
	})
}
