package community

import (
	"strconv"

	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/models/community"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/gofiber/fiber/v3"
)

// GetFeaturedCommunities godoc
// @Summary Get featured communities for a language segment
// @Description Get the list of featured communities for a specific language segment
// @Tags community
// @Accept json
// @Produce json
// @Param segment query string false "Language segment (ru, en, es, ar). Defaults to 'en'"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/community/featured [get]
func GetFeaturedCommunities(c fiber.Ctx) error {
	segment := c.Query("segment", community.SegmentEnglish)

	if !community.IsValidSegment(segment) {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest,
			"Invalid segment. Valid segments: ru, en, es, ar",
		))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	communities, err := svc.GetFeaturedCommunities(segment)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"segment":       segment,
		"communityList": communities,
	})
}

// GetFeaturedCommunitiesByIds godoc
// @Summary Get featured communities by IDs
// @Description Get communities by their NDC IDs for featured display
// @Tags community
// @Accept json
// @Produce json
// @Param request body GetFeaturedByIdsRequest true "List of community IDs"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/community/featured/by-ids [post]
func GetFeaturedCommunitiesByIds(c fiber.Ctx) error {
	var req GetFeaturedByIdsRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if len(req.NdcIds) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest,
			"ndcIds array is required",
		))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	communities, err := svc.GetCommunitiesByIds(req.NdcIds)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"communityList": communities,
	})
}

// SetFeaturedCommunities godoc
// @Summary Set featured communities for a language segment (Astranet Team only)
// @Description Replace the featured communities list for a specific language segment. Requires Astranet role (1000).
// @Tags community-admin
// @Accept json
// @Produce json
// @Param request body SetFeaturedRequest true "Featured communities configuration"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/community/featured [post]
func SetFeaturedCommunities(c fiber.Ctx) error {
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req SetFeaturedRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if !community.IsValidSegment(req.Segment) {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest,
			"Invalid segment. Valid segments: ru, en, es, ar",
		))
	}

	db := middleware.GetDBFromContext(c)

	// Check if user has Astranet role (1000)
	var globalUser user.User
	if err := db.Where("uid = ? AND ndc_id = 0", uid).First(&globalUser).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	if globalUser.Role < user.RoleAstranet {
		return c.Status(fiber.StatusForbidden).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"Only Astranet Team members can manage featured communities",
		))
	}

	svc := service.NewCommunityService(db)

	if err := svc.SetFeaturedCommunities(req.Segment, req.NdcIds, uid); err != nil {
		if err == service.ErrCommunityNotFound {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
				response.StatusInvalidRequest,
				"One or more communities not found",
			))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Featured communities updated",
		"segment": req.Segment,
		"count":   len(req.NdcIds),
	})
}

// AddFeaturedCommunity godoc
// @Summary Add a community to featured list (Astranet Team only)
// @Description Add a single community to the featured list for a segment
// @Tags community-admin
// @Accept json
// @Produce json
// @Param request body AddFeaturedRequest true "Community to add"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/community/featured/add [post]
func AddFeaturedCommunity(c fiber.Ctx) error {
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req AddFeaturedRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if !community.IsValidSegment(req.Segment) {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest,
			"Invalid segment. Valid segments: ru, en, es, ar",
		))
	}

	db := middleware.GetDBFromContext(c)

	// Check if user has Astranet role (1000)
	var globalUser user.User
	if err := db.Where("uid = ? AND ndc_id = 0", uid).First(&globalUser).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	if globalUser.Role < user.RoleAstranet {
		return c.Status(fiber.StatusForbidden).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"Only Astranet Team members can manage featured communities",
		))
	}

	svc := service.NewCommunityService(db)

	if err := svc.AddFeaturedCommunity(req.Segment, req.NdcId, uid); err != nil {
		if err == service.ErrCommunityNotFound {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
				response.StatusInvalidRequest,
				"Community not found",
			))
		}
		if err == service.ErrAlreadyFeatured {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
				response.StatusInvalidRequest,
				"Community is already featured in this segment",
			))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Community added to featured",
		"segment": req.Segment,
		"ndcId":   req.NdcId,
	})
}

// RemoveFeaturedCommunity godoc
// @Summary Remove a community from featured list (Astranet Team only)
// @Description Remove a single community from the featured list for a segment
// @Tags community-admin
// @Accept json
// @Produce json
// @Param segment query string true "Language segment"
// @Param ndcId path int true "Community NDC ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/community/featured/{ndcId} [delete]
func RemoveFeaturedCommunity(c fiber.Ctx) error {
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	segment := c.Query("segment")
	if !community.IsValidSegment(segment) {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest,
			"Invalid segment. Valid segments: ru, en, es, ar",
		))
	}

	ndcId, err := strconv.Atoi(c.Params("ndcId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	db := middleware.GetDBFromContext(c)

	// Check if user has Astranet role (1000)
	var globalUser user.User
	if err := db.Where("uid = ? AND ndc_id = 0", uid).First(&globalUser).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	if globalUser.Role < user.RoleAstranet {
		return c.Status(fiber.StatusForbidden).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"Only Astranet Team members can manage featured communities",
		))
	}

	svc := service.NewCommunityService(db)

	if err := svc.RemoveFeaturedCommunity(segment, ndcId); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Community removed from featured",
		"segment": segment,
		"ndcId":   ndcId,
	})
}

// GetAvailableSegments godoc
// @Summary Get available language segments
// @Description Get list of all available language segments for featured communities
// @Tags community
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /g/s/community/featured/segments [get]
func GetAvailableSegments(c fiber.Ctx) error {
	segments := []map[string]string{
		{"code": community.SegmentRussian, "name": "Русский"},
		{"code": community.SegmentEnglish, "name": "English"},
		{"code": community.SegmentSpanish, "name": "Español"},
		{"code": community.SegmentArabic, "name": "العربية"},
	}

	return c.JSON(fiber.Map{
		"segments": segments,
	})
}
