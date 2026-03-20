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

type SetAstranetRequest struct {
	UID        string `json:"uid"`
	IsAstranet bool   `json:"isAstranet"`
}

// requireBootstrapKey validates that the request uses a bootstrap API key (no UserID).
func requireBootstrapKey(c fiber.Ctx) (*apikey.APIKey, error) {
	apiKeyInterface := middleware.GetAPIKeyFromContext(c)
	if apiKeyInterface == nil {
		return nil, c.Status(fiber.StatusUnauthorized).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"API key required",
		))
	}

	apiKey, ok := apiKeyInterface.(*apikey.APIKey)
	if !ok {
		return nil, c.Status(fiber.StatusUnauthorized).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"Invalid API key",
		))
	}

	if apiKey.UserID != nil {
		return nil, c.Status(fiber.StatusForbidden).JSON(response.NewErrorWithMessage(
			response.StatusNoPermission,
			"Only bootstrap API key can perform this action. This key is tied to a user.",
		))
	}

	return apiKey, nil
}

// SetUserRole godoc
// @Summary Set user role (Bootstrap API Key only)
// @Description Set the community/global role for a user. Use set-astranet for Astranet Team status.
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
	if _, err := requireBootstrapKey(c); err != nil {
		return err
	}

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

	// Validate role — role 1000 is no longer valid, use set-astranet instead
	validRoles := map[int]string{
		user.RoleBanned:  "Banned",
		user.RoleMember:  "Member",
		user.RoleCurator: "Curator",
		user.RoleLeader:  "Leader",
		user.RoleAgent:   "Agent",
	}

	roleName, isValid := validRoles[req.Role]
	if !isValid {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest,
			"Invalid role. Valid roles: -1 (Banned), 0 (Member), 50 (Curator), 100 (Leader), 110 (Agent). Use /admin/set-astranet for Astranet Team status.",
		))
	}

	db := middleware.GetDBFromContext(c)

	var globalUser user.User
	if err := db.Where("uid = ? AND ndc_id = 0", req.UID).First(&globalUser).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewErrorWithMessage(
			response.StatusAccountNotExist,
			"User not found (global profile)",
		))
	}

	oldRole := globalUser.Role

	if err := db.Model(&globalUser).Update("role", req.Role).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message":    "Role updated successfully",
		"uid":        req.UID,
		"nickname":   globalUser.Nickname,
		"oldRole":    oldRole,
		"newRole":    req.Role,
		"roleName":   roleName,
		"isAstranet": globalUser.IsAstranet,
	})
}

// SetAstranetStatus godoc
// @Summary Set Astranet Team status (Bootstrap API Key only)
// @Description Grant or revoke Astranet Team membership. This is separate from community roles.
// @Tags admin
// @Accept json
// @Produce json
// @Param request body SetAstranetRequest true "User UID and Astranet status"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Router /g/s/admin/set-astranet [post]
func SetAstranetStatus(c fiber.Ctx) error {
	if _, err := requireBootstrapKey(c); err != nil {
		return err
	}

	var req SetAstranetRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if req.UID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest,
			"uid is required",
		))
	}

	db := middleware.GetDBFromContext(c)

	var globalUser user.User
	if err := db.Where("uid = ? AND ndc_id = 0", req.UID).First(&globalUser).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewErrorWithMessage(
			response.StatusAccountNotExist,
			"User not found (global profile)",
		))
	}

	oldStatus := globalUser.IsAstranet

	// Update is_astranet on ALL profiles for this user (global + community)
	if err := db.Model(&user.User{}).Where("uid = ?", req.UID).Update("is_astranet", req.IsAstranet).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message":       "Astranet Team status updated",
		"uid":           req.UID,
		"nickname":      globalUser.Nickname,
		"oldIsAstranet": oldStatus,
		"newIsAstranet": req.IsAstranet,
	})
}

// GetUserRole godoc
// @Summary Get user role (Bootstrap API Key only)
// @Description Get the current role and Astranet Team status for a user.
// @Tags admin
// @Accept json
// @Produce json
// @Param uid query string true "User UID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /g/s/admin/get-role [get]
func GetUserRole(c fiber.Ctx) error {
	if _, err := requireBootstrapKey(c); err != nil {
		return err
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
		user.RoleBanned:  "Banned",
		user.RoleMember:  "Member",
		user.RoleCurator: "Curator",
		user.RoleLeader:  "Leader",
		user.RoleAgent:   "Agent",
	}

	return c.JSON(fiber.Map{
		"uid":        globalUser.UID,
		"nickname":   globalUser.Nickname,
		"role":       globalUser.Role,
		"roleName":   roleNames[globalUser.Role],
		"isAstranet": globalUser.IsAstranet,
	})
}
