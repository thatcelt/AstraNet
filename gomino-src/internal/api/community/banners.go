package community

import (
	communityModel "github.com/AugustLigh/GoMino/internal/models/community"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/gofiber/fiber/v3"
)

// GetBanners returns banners for a language segment
func GetBanners(c fiber.Ctx) error {
	segment := c.Query("segment", communityModel.SegmentEnglish)

	if !communityModel.IsValidSegment(segment) {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest,
			"Invalid segment. Valid segments: ru, en, es, ar",
		))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	banners, err := svc.GetBanners(segment)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"segment":    segment,
		"bannerList": banners,
	})
}

// SetBanners replaces all banners for a language segment (Astranet only)
func SetBanners(c fiber.Ctx) error {
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req SetBannersRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if !communityModel.IsValidSegment(req.Segment) {
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
			"Only Astranet Team members can manage banners",
		))
	}

	// Convert request items to service format
	banners := make([]struct {
		ImageURL string
		LinkURL  string
	}, len(req.Banners))
	for i, b := range req.Banners {
		banners[i].ImageURL = b.ImageURL
		banners[i].LinkURL = b.LinkURL
	}

	svc := service.NewCommunityService(db)

	if err := svc.SetBanners(req.Segment, banners, uid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Banners updated",
		"segment": req.Segment,
		"count":   len(req.Banners),
	})
}
