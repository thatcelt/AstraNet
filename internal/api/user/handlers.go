package user

import (
	"log"
	"strconv"

	"github.com/AugustLigh/GoMino/internal/ctxutils"
	"github.com/AugustLigh/GoMino/internal/middleware"
	utilsModel "github.com/AugustLigh/GoMino/internal/models/utils"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/AugustLigh/GoMino/internal/ws"
	"github.com/gofiber/fiber/v3"
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
	// Try parsing from query param (e.g. ?ndcId=...)
	if ndcIdStr := c.Query("ndcId"); ndcIdStr != "" {
		if id, err := strconv.Atoi(ndcIdStr); err == nil {
			return id
		}
	}
	return 0 // Global
}

// GetUserInfo godoc
// @Summary Get user profile
// @Description Get public user profile information
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId} [get]
// @Router /x{comId}/s/user-profile/{userId} [get]
func GetUserInfo(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	userId := c.Params("userId")
	ndcId := getNdcId(c)

	userService := service.NewUserService(db)

	user, err := userService.GetUserByID(userId, ndcId, true)
	if err != nil {
		log.Printf("Error getting user %s: %v", userId, err)
		return c.Status(fiber.StatusOK).JSON(response.NewError(response.StatusAccountNotExist))
	}

	// Set dynamic online status (0 = offline, 1 = online)
	if Hub != nil && Hub.Presence != nil && Hub.Presence.IsUserOnline(userId) {
		user.OnlineStatus = 1
	} else {
		user.OnlineStatus = 0
	}

	// Check if requesting user is following the target user
	auid := c.Get("AUID")
	if auid != "" && auid != userId {
		if userService.IsFollowing(auid, userId, ndcId) {
			user.MembershipStatus = 1
		} else {
			user.MembershipStatus = 0
		}
	}

	return c.JSON(fiber.Map{
		"userProfile": user,
	})
}

// UpdateUserInfo godoc
// @Summary Update user profile
// @Description Update user profile details
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   request body UpdateUserRequest true "Update data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId} [post]
// @Router /x{comId}/s/user-profile/{userId} [post]
func UpdateUserInfo(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	userId := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)
	ndcId := getNdcId(c)

	// Проверяем, что пользователь обновляет свой профиль
	if auid != userId {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusInvalidRequest))
	}

	var req UpdateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	updates := req.ToMap()
	if len(updates) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	userService := service.NewUserService(db)

	user, err := userService.UpdateUser(userId, ndcId, updates)
	if err != nil {
		log.Printf("Error updating user %s: %v", userId, err)
		return c.Status(fiber.StatusOK).JSON(response.NewError(response.StatusAccountNotExist))
	}

	return c.JSON(fiber.Map{
		"userProfile": user,
	})
}

// GetUserFollowing godoc
// @Summary Get user following list
// @Description Get list of users the target user is following
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   start query int false "Start offset"
// @Param   size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Router /g/s/user-profile/{userId}/joined [get]
// @Router /x{comId}/s/user-profile/{userId}/joined [get]
func GetUserFollowing(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	userId := c.Params("userId")
	ndcId := getNdcId(c)

	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))

	userService := service.NewUserService(db)
	following, err := userService.GetFollowing(userId, ndcId, start, size)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"userProfileList": following,
	})
}

// GetMembers godoc
// @Summary Get user followers list
// @Description Get list of users following the target user
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   start query int false "Start offset"
// @Param   size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Router /g/s/user-profile/{userId}/member [get]
// @Router /x{comId}/s/user-profile/{userId}/member [get]
func GetMembers(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	userId := c.Params("userId")
	ndcId := getNdcId(c)

	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))

	userService := service.NewUserService(db)
	followers, err := userService.GetFollowers(userId, ndcId, start, size)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"userProfileList": followers,
	})
}

// FollowUser godoc
// @Summary Follow user
// @Description Follow the target user
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID to follow"
// @Param   comId path string false "Community ID (NDC ID)"
// @Success 200 {object} nil
// @Failure 401 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId}/member [post]
// @Router /x{comId}/s/user-profile/{userId}/member [post]
func FollowUser(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	userId := c.Params("userId") // На кого подписываемся
	auid := middleware.GetAUIDFromContext(c)
	comId := ctxutils.GetComIdFromContext(c) // Получаем ID сообщества из контекста

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	userService := service.NewUserService(db)

	// Check if target user has blocked us
	if userService.IsBlocked(userId, auid) {
		return c.Status(fiber.StatusForbidden).JSON(response.BlockedByUser())
	}

	if err := userService.FollowUser(auid, userId, comId); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Send notification about new follower only for global context
	if comId == 0 {
		go func() {
			notificationService := service.NewNotificationService(db, Hub)
			_ = notificationService.NotifyUserAboutNewFollower(userId, auid)
		}()
	}

	return nil
}

// UnfollowUser godoc
// @Summary Unfollow user
// @Description Unfollow the target user
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID to unfollow"
// @Param   comId path string false "Community ID (NDC ID)"
// @Success 200 {object} nil
// @Failure 401 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId}/member [delete]
// @Router /x{comId}/s/user-profile/{userId}/member [delete]
func UnfollowUser(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	userId := c.Params("userId") // От кого отписываемся
	auid := middleware.GetAUIDFromContext(c)
	comId := ctxutils.GetComIdFromContext(c) // Получаем ID сообщества из контекста

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	userService := service.NewUserService(db)
	if err := userService.UnfollowUser(auid, userId, comId); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// GetWallComments godoc
// @Summary Get wall comments
// @Description Get comments on the user's wall
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   sort query string false "Sort order (newest/oldest)"
// @Param   start query int false "Start offset"
// @Param   size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Router /g/s/user-profile/{userId}/g-comment [get]
// @Router /x{comId}/s/user-profile/{userId}/g-comment [get]
func GetWallComments(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	userId := c.Params("userId")

	ndcId := getNdcId(c)
	userService := service.NewUserService(db)
	comments, err := userService.GetWallComments(userId, ndcId, "newest", 0, 25)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"commentList": comments,
	})
}

// AddWallComment godoc
// @Summary Add wall comment
// @Description Add a comment to the user's wall
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   request body CreateCommentRequest true "Comment data"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId}/g-comment [post]
// @Router /x{comId}/s/user-profile/{userId}/g-comment [post]
func AddWallComment(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	targetUID := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	var req CreateCommentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	userService := service.NewUserService(db)

	// Check if wall owner has blocked the commenter
	if userService.IsBlocked(targetUID, auid) {
		return c.Status(fiber.StatusForbidden).JSON(response.BlockedByUser())
	}

	replyTo := ""
	if req.RespondTo != nil {
		replyTo = *req.RespondTo
	}

	// Prepare media list
	var mediaList *utilsModel.MediaList
	if len(req.MediaList) > 0 {
		ml := utilsModel.MediaList(req.MediaList)
		mediaList = &ml
	}

	ndcId := getNdcId(c)
	comment, err := userService.AddWallComment(auid, targetUID, req.Content, replyTo, ndcId, req.Type, req.StickerID, req.StickerIcon, req.StickerMedia, mediaList)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"comment": comment,
	})
}

// ReplyToComment godoc
// @Summary Reply to wall comment
// @Description Reply to a comment on the user's wall
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   request body CreateCommentRequest true "Comment data"
// @Success 200 {object} map[string]interface{}
// @Router /g/s/user-profile/{userId}/comment [post]
// @Router /x{comId}/s/user-profile/{userId}/comment [post]
func ReplyToComment(c fiber.Ctx) error {
	return AddWallComment(c)
}

// DeleteWallComment godoc
// @Summary Delete wall comment
// @Description Delete a comment from the user's wall
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID"
// @Param   comId path string false "Community ID (NDC ID)"
// @Param   commentId path string true "Comment ID"
// @Success 200 {object} nil
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId}/comment/{commentId} [delete]
// @Router /x{comId}/s/user-profile/{userId}/comment/{commentId} [delete]
func DeleteWallComment(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	// userId := c.Params("userId") // Владелец профиля (можно использовать для проверки, но в service мы проверяем ParentID)
	commentId := c.Params("commentId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	userService := service.NewUserService(db)
	err := userService.DeleteWallComment(commentId, auid)
	if err != nil {
		if err.Error() == "permission denied" {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusInvalidRequest))
		}
		if err.Error() == "comment not found" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return nil
}

// GetJoinedCommunities - заглушка или список сообществ
func GetJoinedCommunities(c fiber.Ctx) error {
	// В Amino это возвращает список communityList.
	// Если у нас standalone - возвращаем пустой список или текущее 'сообщество'
	return c.JSON(fiber.Map{
		"communityList": []interface{}{},
	})
}

// ==================== Global Ban (Astranet Only) ====================

// GlobalBanUser godoc
// @Summary Globally ban a user
// @Description Globally ban a user from the entire platform (Astranet team only)
// @Tags admin-moderation
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID to ban"
// @Param   request body GlobalBanRequest true "Ban reason"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId}/global-ban [post]
func GlobalBanUser(c fiber.Ctx) error {
	targetUID := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req GlobalBanRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	db := middleware.GetDBFromContext(c)
	userSvc := service.NewUserService(db)

	if err := userSvc.GlobalBanUser(auid, targetUID, req.Reason); err != nil {
		if err == service.ErrPermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err.Error() == "cannot ban Astranet team members" {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "User globally banned",
	})
}

// GlobalUnbanUser godoc
// @Summary Globally unban a user
// @Description Remove global ban from a user (Astranet team only)
// @Tags admin-moderation
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID to unban"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId}/global-unban [post]
func GlobalUnbanUser(c fiber.Ctx) error {
	targetUID := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	db := middleware.GetDBFromContext(c)
	userSvc := service.NewUserService(db)

	if err := userSvc.GlobalUnbanUser(auid, targetUID); err != nil {
		if err == service.ErrPermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "User globally unbanned",
	})
}

// GlobalBanRequest - request body for global ban
type GlobalBanRequest struct {
	Reason string `json:"reason"`
}

// ==================== User Blocking ====================

// BlockUser godoc
// @Summary Block a user
// @Description Block another user. The blocked user cannot message you or join chats you organize.
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID to block"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId}/block [post]
func BlockUser(c fiber.Ctx) error {
	targetUID := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	db := middleware.GetDBFromContext(c)
	userSvc := service.NewUserService(db)

	if err := userSvc.BlockUser(auid, targetUID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(response.StatusInvalidRequest, err.Error()))
	}

	// Auto-unfollow in both directions (all communities + global)
	_ = userSvc.UnfollowUser(auid, targetUID, 0)
	_ = userSvc.UnfollowUser(targetUID, auid, 0)
	// Also remove community-scoped follows
	userSvc.UnfollowUserAllCommunities(auid, targetUID)

	return c.JSON(fiber.Map{
		"api:statuscode": 0,
		"message":        "User blocked",
	})
}

// UnblockUser godoc
// @Summary Unblock a user
// @Description Remove a block on another user.
// @Tags user-profile
// @Accept  json
// @Produce  json
// @Param   userId path string true "User ID to unblock"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId}/unblock [post]
func UnblockUser(c fiber.Ctx) error {
	targetUID := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	db := middleware.GetDBFromContext(c)
	userSvc := service.NewUserService(db)

	if err := userSvc.UnblockUser(auid, targetUID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(response.StatusInvalidRequest, err.Error()))
	}

	return c.JSON(fiber.Map{
		"api:statuscode": 0,
		"message":        "User unblocked",
	})
}

// GetBlockedUsers godoc
// @Summary Get blocked users list
// @Description Returns the list of users blocked by the authenticated user.
// @Tags user-profile
// @Produce  json
// @Param   start query int false "Start offset" default(0)
// @Param   size query int false "Page size" default(25)
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Router /g/s/user-profile/blocked [get]
func GetBlockedUsers(c fiber.Ctx) error {
	auid := middleware.GetAUIDFromContext(c)
	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))

	db := middleware.GetDBFromContext(c)
	userSvc := service.NewUserService(db)

	blocks, total, err := userSvc.GetBlockedUsers(auid, start, size)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Build user list from blocks
	userList := make([]map[string]interface{}, 0, len(blocks))
	for _, b := range blocks {
		userList = append(userList, map[string]interface{}{
			"uid":       b.BlockedUID,
			"nickname":  b.BlockedUser.Nickname,
			"icon":      b.BlockedUser.Icon,
			"blockedAt": b.CreatedAt,
		})
	}

	return c.JSON(fiber.Map{
		"api:statuscode": 0,
		"userList":       userList,
		"total":          total,
	})
}

// IsBlocked godoc
// @Summary Check if a user is blocked
// @Description Check if the authenticated user has blocked the specified user.
// @Tags user-profile
// @Produce  json
// @Param   userId path string true "User ID to check"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId}/blocked-status [get]
func IsBlocked(c fiber.Ctx) error {
	targetUID := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)
	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	db := middleware.GetDBFromContext(c)
	userSvc := service.NewUserService(db)

	blocked := userSvc.IsBlocked(auid, targetUID)
	blockedBy := userSvc.IsBlocked(targetUID, auid)

	return c.JSON(fiber.Map{
		"api:statuscode": 0,
		"blocked":        blocked,
		"blockedBy":      blockedBy,
	})
}
