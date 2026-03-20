package user

import (
	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/gofiber/fiber/v3"
)

// ==================== Read Mode ====================

type EnableReadModeRequest struct {
	Reason          string `json:"reason"`
	DurationMinutes int    `json:"durationMinutes,omitempty"` // only for global
}

// EnableGlobalReadMode godoc
// @Summary Enable global read mode for a user
// @Description Put a user into global read mode (Astranet only). Duration: 30, 60, 120, 1440, 4320 minutes.
// @Tags admin-moderation
// @Accept json
// @Produce json
// @Param userId path string true "Target User ID"
// @Param body body EnableReadModeRequest true "Reason and duration"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId}/read-mode/enable [post]
func EnableGlobalReadMode(c fiber.Ctx) error {
	targetUID := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)
	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req EnableReadModeRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewReadModeService(db, Hub)

	if err := svc.EnableGlobalReadMode(auid, targetUID, req.DurationMinutes, req.Reason); err != nil {
		if err == service.ErrPermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err == service.ErrInvalidDuration {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(response.StatusInvalidRequest, "Invalid duration. Allowed: 30, 60, 120, 1440, 4320 minutes."))
		}
		if err == service.ErrAlreadyInReadMode {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(response.StatusInvalidRequest, "User is already in read mode."))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{"message": "Global read mode enabled"})
}

// DisableGlobalReadMode godoc
// @Summary Disable global read mode for a user
// @Description Lift global read mode from a user (Astranet only)
// @Tags admin-moderation
// @Accept json
// @Produce json
// @Param userId path string true "Target User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} response.ErrorResponse
// @Router /g/s/user-profile/{userId}/read-mode/disable [post]
func DisableGlobalReadMode(c fiber.Ctx) error {
	targetUID := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)
	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewReadModeService(db, Hub)

	if err := svc.DisableGlobalReadMode(auid, targetUID); err != nil {
		if err == service.ErrPermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err == service.ErrReadModeNotFound {
			return c.Status(fiber.StatusNotFound).JSON(response.NewErrorWithMessage(response.StatusInvalidRequest, "User is not in read mode."))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{"message": "Global read mode disabled"})
}

// EnableCommunityReadMode godoc
// @Summary Enable community read mode for a user
// @Description Put a user into read mode within a community (Curator+ only)
// @Tags admin-moderation
// @Accept json
// @Produce json
// @Param comId path string true "Community NDC ID"
// @Param userId path string true "Target User ID"
// @Param body body EnableReadModeRequest true "Reason"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} response.ErrorResponse
// @Router /x{comId}/s/user-profile/{userId}/read-mode/enable [post]
func EnableCommunityReadMode(c fiber.Ctx) error {
	targetUID := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)
	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	ndcID := getNdcId(c)
	if ndcID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	var req EnableReadModeRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewReadModeService(db, Hub)

	if err := svc.EnableCommunityReadMode(auid, targetUID, ndcID, req.Reason, req.DurationMinutes); err != nil {
		if err == service.ErrPermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err == service.ErrAlreadyInReadMode {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(response.StatusInvalidRequest, "User is already in read mode."))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{"message": "Community read mode enabled"})
}

// DisableCommunityReadMode godoc
// @Summary Disable community read mode for a user
// @Description Lift read mode from a user in a community (Curator+ only)
// @Tags admin-moderation
// @Accept json
// @Produce json
// @Param comId path string true "Community NDC ID"
// @Param userId path string true "Target User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} response.ErrorResponse
// @Router /x{comId}/s/user-profile/{userId}/read-mode/disable [post]
func DisableCommunityReadMode(c fiber.Ctx) error {
	targetUID := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)
	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	ndcID := getNdcId(c)
	if ndcID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewReadModeService(db, Hub)

	if err := svc.DisableCommunityReadMode(auid, targetUID, ndcID); err != nil {
		if err == service.ErrPermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err == service.ErrReadModeNotFound {
			return c.Status(fiber.StatusNotFound).JSON(response.NewErrorWithMessage(response.StatusInvalidRequest, "User is not in read mode."))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{"message": "Community read mode disabled"})
}

// GetReadModeStatus godoc
// @Summary Get read mode status for a user
// @Description Returns all active read modes for a user
// @Tags admin-moderation
// @Accept json
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /g/s/user-profile/{userId}/read-mode [get]
func GetReadModeStatus(c fiber.Ctx) error {
	targetUID := c.Params("userId")
	auid := middleware.GetAUIDFromContext(c)
	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewReadModeService(db, Hub)

	modes, err := svc.GetReadModeStatus(targetUID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"readModeList": modes,
	})
}
