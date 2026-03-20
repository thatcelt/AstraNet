package community

import (
	"fmt"
	"strconv"

	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/models/community"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/gofiber/fiber/v3"
)

// CreateCommunity godoc
// @Summary Create a new community
// @Description Create a new community with the given details
// @Tags community
// @Accept  json
// @Produce  json
// @Param   request body CreateCommunityRequest true "Community details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/community [post]
func CreateCommunity(c fiber.Ctx) error {
	var req CreateCommunityRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	newCom := &community.Community{
		Name:            req.Name,
		Tagline:         &req.Tagline,
		Icon:            req.Icon,
		PrimaryLanguage: req.PrimaryLanguage,
		JoinType:        req.PrivacyMode,

		// Map other fields as needed
	}

	createdCom, err := svc.CreateCommunity(newCom, uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"community": createdCom,
	})
}

// GetCommunity godoc
// @Summary Get community details
// @Description Get details of a specific community
// @Tags community
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID or NDC ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s-x{comId}/community [get]
func GetCommunity(c fiber.Ctx) error {
	comIdStr := c.Params("comId")
	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	var id interface{}
	// Try to parse as int (NdcId)
	if ndcId, err := strconv.Atoi(comIdStr); err == nil {
		id = ndcId
	} else {
		// Treat as endpoint
		id = comIdStr
	}

	com, err := svc.GetCommunity(id)
	if err != nil {
		if err == service.ErrCommunityNotFound {
			// Using StatusAccountNotExist as a generic "Not Found" for now
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"community": com,
	})
}

// JoinCommunity godoc
// @Summary Join a community
// @Description Join a specific community
// @Tags community
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/community/join [post]
func JoinCommunity(c fiber.Ctx) error {
	comIdStr := c.Params("comId")
	fmt.Printf("[JoinCommunity] Raw comId param: '%s', Full path: %s\n", comIdStr, c.Path())

	ndcId, err := strconv.Atoi(comIdStr)
	if err != nil {
		fmt.Printf("[JoinCommunity] Failed to parse comId '%s': %v\n", comIdStr, err)
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		fmt.Printf("[JoinCommunity] No UID in context\n")
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	fmt.Printf("[JoinCommunity] uid=%s, ndcId=%d\n", uid, ndcId)

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	err = svc.JoinCommunity(ndcId, uid)
	if err != nil {
		fmt.Printf("[JoinCommunity] Error: %v\n", err)
		if err == service.ErrAlreadyJoined {
			// Already joined - return success to allow navigation
			fmt.Printf("[JoinCommunity] User already joined, returning success\n")
			return c.JSON(fiber.Map{
				"membershipStatus": 1,
			})
		}
		if err == service.ErrCommunityNotFound {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	fmt.Printf("[JoinCommunity] Success for uid=%s in ndcId=%d\n", uid, ndcId)
	return c.JSON(fiber.Map{
		"membershipStatus": 1, // Joined
	})
}

// LeaveCommunity godoc
// @Summary Leave a community
// @Description Leave a specific community
// @Tags community
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Success 200 {object} map[string]interface{}
// @Router /x{comId}/s/community/leave [post]
func LeaveCommunity(c fiber.Ctx) error {
	// TODO: Implement leave logic
	return c.JSON(fiber.Map{})
}

// GetJoinedCommunities godoc
// @Summary Get joined communities
// @Description Get a list of communities the user has joined
// @Tags community
// @Accept  json
// @Produce  json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/community/joined [get]
func GetJoinedCommunities(c fiber.Ctx) error {
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	start := 0
	size := 25 // Default size

	// TODO: Parse query params for start/size if needed

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	list, err := svc.GetJoinedCommunities(uid, start, size)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{

		"communityList": list,
	})

}

// Helper to get NDC ID from params (reused from logic in JoinCommunity/GetCommunity or centralized)

func getNdcId(c fiber.Ctx) (int, error) {

	comIdStr := c.Params("comId")

	return strconv.Atoi(comIdStr)

}

// TransferAgent godoc
// @Summary Transfer agent status
// @Description Transfer agent status to another user
// @Tags community-admin
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   userId path string true "Target User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/user-profile/{userId}/transfer-agent [post]
func TransferAgent(c fiber.Ctx) error {

	ndcId, err := getNdcId(c)

	if err != nil {

		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))

	}

	targetUID := c.Params("userId")

	requesterUID := middleware.GetAUIDFromContext(c)

	if requesterUID == "" {

		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))

	}

	db := middleware.GetDBFromContext(c)

	svc := service.NewCommunityService(db)

	if err := svc.TransferAgent(ndcId, requesterUID, targetUID); err != nil {

		if err == service.ErrPermissionDenied {

			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))

		}

		if err == service.ErrCommunityNotFound || err.Error() == "new agent user not found" {

			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))

		}

		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))

	}

	return c.JSON(fiber.Map{

		"message": "Agent transferred",
	})

}

// PromoteToLeader godoc
// @Summary Promote user to leader
// @Description Promote a user to leader role
// @Tags community-admin
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   userId path string true "Target User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/user-profile/{userId}/leader [post]
func PromoteToLeader(c fiber.Ctx) error {

	ndcId, err := getNdcId(c)

	if err != nil {

		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))

	}

	targetUID := c.Params("userId")

	requesterUID := middleware.GetAUIDFromContext(c)

	if requesterUID == "" {

		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))

	}

	db := middleware.GetDBFromContext(c)

	svc := service.NewCommunityService(db)

	if err := svc.PromoteToLeader(ndcId, requesterUID, targetUID); err != nil {

		if err == service.ErrPermissionDenied {

			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))

		}

		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))

	}

	return c.JSON(fiber.Map{

		"message": "Promoted to leader",
	})

}

// PromoteToCurator godoc
// @Summary Promote user to curator
// @Description Promote a user to curator role
// @Tags community-admin
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   userId path string true "Target User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/user-profile/{userId}/curator [post]
func PromoteToCurator(c fiber.Ctx) error {

	ndcId, err := getNdcId(c)

	if err != nil {

		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))

	}

	targetUID := c.Params("userId")

	requesterUID := middleware.GetAUIDFromContext(c)

	if requesterUID == "" {

		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))

	}

	db := middleware.GetDBFromContext(c)

	svc := service.NewCommunityService(db)

	if err := svc.PromoteToCurator(ndcId, requesterUID, targetUID); err != nil {

		if err == service.ErrPermissionDenied {

			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))

		}

		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))

	}

		return c.JSON(fiber.Map{

			"message": "Promoted to curator",

		})

	}

	
// BanUser godoc
// @Summary Ban user
// @Description Ban a user from the community
// @Tags community-admin
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   userId path string true "Target User ID"
// @Param   request body BanRequest true "Ban reason"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/user-profile/{userId}/ban [post]
	func BanUser(c fiber.Ctx) error {

		ndcId, err := getNdcId(c)

		if err != nil {

			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))

		}

	

		targetUID := c.Params("userId")

		requesterUID := middleware.GetAUIDFromContext(c)

	

		var req BanRequest

		if err := c.Bind().JSON(&req); err != nil {

			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))

		}

	

		db := middleware.GetDBFromContext(c)

		svc := service.NewCommunityService(db)

	

		if err := svc.BanUser(ndcId, requesterUID, targetUID, req.Note.Content); err != nil {

			if err == service.ErrPermissionDenied {

				return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))

			}

			return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))

		}

	

		return c.JSON(fiber.Map{

			"message": "User banned",

		})

	}

	
// UnbanUser godoc
// @Summary Unban user
// @Description Unban a user from the community
// @Tags community-admin
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   userId path string true "Target User ID"
// @Param   request body BanRequest true "Unban reason"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/user-profile/{userId}/unban [post]
	func UnbanUser(c fiber.Ctx) error {

		ndcId, err := getNdcId(c)

		if err != nil {

			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))

		}

	

		targetUID := c.Params("userId")

		requesterUID := middleware.GetAUIDFromContext(c)

	

		var req BanRequest

		if err := c.Bind().JSON(&req); err != nil {

			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))

		}

	

		db := middleware.GetDBFromContext(c)

		svc := service.NewCommunityService(db)

	

		if err := svc.UnbanUser(ndcId, requesterUID, targetUID, req.Note.Content); err != nil {

			if err == service.ErrPermissionDenied {

				return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))

			}

			return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))

		}

	

		return c.JSON(fiber.Map{

			"message": "User unbanned",

		})

	}

	
// GetModerationHistory godoc
// @Summary Get moderation history
// @Description Get moderation logs for the community
// @Tags community-admin
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   size query int false "Page size (max 100)"
// @Param   objectId query string false "Filter by Object ID"
// @Param   objectType query int false "Filter by Object Type"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/admin/operation [get]
	func GetModerationHistory(c fiber.Ctx) error {

		ndcId, err := getNdcId(c)

		if err != nil {

			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))

		}

	

		start := 0 // TODO: parse start/size logic if needed, user said "Use size, max 100".

		size := 25

		if s := c.Query("size"); s != "" {

			if val, err := strconv.Atoi(s); err == nil {

				size = val

			}

		}

		// Cap size

		if size > 100 {

			size = 100

		}

		

		// Parse objectId/objectType filters

		var objectId *string

		if oid := c.Query("objectId"); oid != "" {

			objectId = &oid

		}

		

		var objectType *int

		if otype := c.Query("objectType"); otype != "" {

			if val, err := strconv.Atoi(otype); err == nil {

				objectType = &val

			}

		}

	

		db := middleware.GetDBFromContext(c)

		svc := service.NewCommunityService(db)

	

		logs, err := svc.GetModerationHistory(ndcId, objectId, objectType, start, size)

		if err != nil {

			return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))

		}

	

		return c.JSON(fiber.Map{

			"adminLogList": logs,

		})

	}

// UpdateCommunitySettings godoc
// @Summary Update community settings
// @Description Update community settings (Agent or Leader only)
// @Tags community-admin
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   request body UpdateCommunitySettingsRequest true "Updated settings"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/community/settings [post]
func UpdateCommunitySettings(c fiber.Ctx) error {
	ndcId, err := getNdcId(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req UpdateCommunitySettingsRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.Endpoint != nil {
		updates["endpoint"] = *req.Endpoint
	}
	if req.PrimaryLanguage != nil {
		updates["primary_language"] = *req.PrimaryLanguage
	}
	if req.ThemePack != nil {
		themePackUpdate := make(map[string]*string)
		if req.ThemePack.ThemeColor != nil {
			themePackUpdate["themeColor"] = req.ThemePack.ThemeColor
		}
		if req.ThemePack.ThemeSideImage != nil {
			themePackUpdate["themeSideImage"] = req.ThemePack.ThemeSideImage
		}
		if req.ThemePack.ThemeUpperImage != nil {
			themePackUpdate["themeUpperImage"] = req.ThemePack.ThemeUpperImage
		}
		if req.ThemePack.Cover != nil {
			themePackUpdate["cover"] = req.ThemePack.Cover
		}
		if len(themePackUpdate) > 0 {
			updates["theme_pack_update"] = themePackUpdate
		}
	}
	if req.Searchable != nil {
		updates["searchable"] = *req.Searchable
		fmt.Printf("[DEBUG] Handler: searchable=%v\n", *req.Searchable)
	} else {
		fmt.Printf("[DEBUG] Handler: searchable is nil\n")
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	if err := svc.UpdateCommunitySettings(ndcId, uid, updates); err != nil {
		if err == service.ErrPermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err == service.ErrCommunityNotFound {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Settings updated",
	})
}

// GetCommunityMembers godoc
// @Summary Get community members
// @Description Get members of a community, optionally filtered by type
// @Tags community
// @Produce json
// @Param comId path string true "Community ID (NDC ID)"
// @Param type query string false "Member type filter: leaders, curators, members"
// @Param start query int false "Offset"
// @Param size query int false "Page size (max 100)"
// @Success 200 {object} map[string]interface{}
// @Router /x{comId}/s/community/member [get]
func GetCommunityMembers(c fiber.Ctx) error {
	ndcId, err := getNdcId(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	start := 0
	size := 25
	memberType := c.Query("type")

	if s := c.Query("start"); s != "" {
		if val, err := strconv.Atoi(s); err == nil {
			start = val
		}
	}
	if s := c.Query("size"); s != "" {
		if val, err := strconv.Atoi(s); err == nil {
			size = val
		}
	}
	if size > 100 {
		size = 100
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	members, err := svc.GetCommunityMembers(ndcId, memberType, start, size)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"userProfileList": members,
	})
}
