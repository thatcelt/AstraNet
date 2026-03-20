package admin

import (
	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/models/apikey"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/gofiber/fiber/v3"
)

type SetRoleRequest struct {
	UID  string `json:"uid"`
	Role int    `json:"role"`
}

// SetUserRole godoc
// @Summary Set user role (Bootstrap API Key only)
// @Description Set the role for a user. Only accessible with bootstrap API key (server owner).
// @Tags admin
// @Accept json
// @Produce json
// @Param request body SetRoleRequest true "User UID and role"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Router /g/s/admin/set-role [post]
func SetUserRole(c fiber.Ctx) error {
	// Check if request has a valid bootstrap API key
	apiKeyInterface := middleware.GetAPIKeyFromContext(c)
	if apiKeyInterface == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"API key required",
		))
	}

	apiKey, ok := apiKeyInterface.(*apikey.APIKey)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"Invalid API key",
		))
	}

	// Bootstrap key has no UserID (it's not tied to any user)
	if apiKey.UserID != nil {
		return c.Status(fiber.StatusForbidden).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"Only bootstrap API key can set roles. This key is tied to a user.",
		))
	}

	// Parse request
	var req SetRoleRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if req.UID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest,
			"uid is required",
		))
	}

	// Validate role
	validRoles := map[int]string{
		user.RoleBanned:   "Banned",
		user.RoleMember:   "Member",
		user.RoleCurator:  "Curator",
		user.RoleLeader:   "Leader",
		user.RoleAgent:    "Agent",
		user.RoleAstranet: "Astranet Team",
	}

	roleName, isValid := validRoles[req.Role]
	if !isValid {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest,
			"Invalid role. Valid roles: -1 (Banned), 0 (Member), 50 (Curator), 100 (Leader), 110 (Agent), 1000 (Astranet)",
		))
	}

	db := middleware.GetDBFromContext(c)

	// Find the global user (ndc_id = 0)
	var globalUser user.User
	if err := db.Where("uid = ? AND ndc_id = 0", req.UID).First(&globalUser).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewErrorWithMessage(
			response.StatusAccountNotExist,
			"User not found (global profile)",
		))
	}

	oldRole := globalUser.Role

	// Update role
	if err := db.Model(&globalUser).Update("role", req.Role).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message":  "Role updated successfully",
		"uid":      req.UID,
		"nickname": globalUser.Nickname,
		"oldRole":  oldRole,
		"newRole":  req.Role,
		"roleName": roleName,
	})
}

// GetUserRole godoc
// @Summary Get user role (Bootstrap API Key only)
// @Description Get the current role for a user.
// @Tags admin
// @Accept json
// @Produce json
// @Param uid query string true "User UID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /g/s/admin/get-role [get]
func GetUserRole(c fiber.Ctx) error {
	// Check if request has a valid bootstrap API key
	apiKeyInterface := middleware.GetAPIKeyFromContext(c)
	if apiKeyInterface == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"API key required",
		))
	}

	apiKey, ok := apiKeyInterface.(*apikey.APIKey)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"Invalid API key",
		))
	}

	// Bootstrap key has no UserID
	if apiKey.UserID != nil {
		return c.Status(fiber.StatusForbidden).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"Only bootstrap API key can view roles",
		))
	}

	uid := c.Query("uid")
	if uid == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest,
			"uid query parameter is required",
		))
	}

	db := middleware.GetDBFromContext(c)

	var globalUser user.User
	if err := db.Where("uid = ? AND ndc_id = 0", uid).First(&globalUser).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewErrorWithMessage(
			response.StatusAccountNotExist,
			"User not found",
		))
	}

	roleNames := map[int]string{
		user.RoleBanned:   "Banned",
		user.RoleMember:   "Member",
		user.RoleCurator:  "Curator",
		user.RoleLeader:   "Leader",
		user.RoleAgent:    "Agent",
		user.RoleAstranet: "Astranet Team",
	}

	return c.JSON(fiber.Map{
		"uid":      globalUser.UID,
		"nickname": globalUser.Nickname,
		"role":     globalUser.Role,
		"roleName": roleNames[globalUser.Role],
	})
}
