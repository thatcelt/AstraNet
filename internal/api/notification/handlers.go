package notification

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

// Helper to get NDC ID from context or params
func getNdcId(c fiber.Ctx) int {
	// Try parsing from URL param (e.g. /x:comId/...)
	comIdStr := c.Params("comId")
	if comIdStr != "" {
		if id, err := strconv.Atoi(comIdStr); err == nil {
			return id
		}
	}
	return 0 // Global
}

// GetNotifications godoc
// @Summary Get notifications
// @Description Get notifications for the authenticated user
// @Tags notifications
// @Accept json
// @Produce json
// @Param comId path string false "Community ID (NDC ID)"
// @Param start query int false "Start offset"
// @Param size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/notification [get]
// @Router /x{comId}/s/notification [get]
func GetNotifications(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	auid := middleware.GetAUIDFromContext(c)
	ndcId := getNdcId(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))

	if size > 100 {
		size = 100
	}

	svc := service.NewNotificationService(db, Hub)
	notifications, err := svc.GetNotifications(auid, ndcId, start, size)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"notificationList": notifications,
	})
}

// GetUnreadCount godoc
// @Summary Get unread notification count
// @Description Get the count of unread notifications for the authenticated user
// @Tags notifications
// @Accept json
// @Produce json
// @Param comId path string false "Community ID (NDC ID)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/notification/count [get]
// @Router /x{comId}/s/notification/count [get]
func GetUnreadCount(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	auid := middleware.GetAUIDFromContext(c)
	ndcId := getNdcId(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	svc := service.NewNotificationService(db, Hub)
	count, err := svc.GetUnreadCount(auid, ndcId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"unreadCount": count,
	})
}

// MarkAsRead godoc
// @Summary Mark notification as read
// @Description Mark a single notification as read
// @Tags notifications
// @Accept json
// @Produce json
// @Param comId path string false "Community ID (NDC ID)"
// @Param id path string true "Notification ID"
// @Success 200 {object} nil
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/notification/{id}/read [post]
// @Router /x{comId}/s/notification/{id}/read [post]
func MarkAsRead(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	auid := middleware.GetAUIDFromContext(c)
	notificationId := c.Params("id")

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	svc := service.NewNotificationService(db, Hub)
	err := svc.MarkAsRead(notificationId, auid)
	if err != nil {
		if err == service.ErrNotificationNotFound {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Notification marked as read",
	})
}

// MarkAllAsRead godoc
// @Summary Mark all notifications as read
// @Description Mark all notifications as read for the authenticated user
// @Tags notifications
// @Accept json
// @Produce json
// @Param comId path string false "Community ID (NDC ID)"
// @Success 200 {object} nil
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/notification/checked [post]
// @Router /x{comId}/s/notification/checked [post]
func MarkAllAsRead(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	auid := middleware.GetAUIDFromContext(c)
	ndcId := getNdcId(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	svc := service.NewNotificationService(db, Hub)
	err := svc.MarkAllAsRead(auid, ndcId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "All notifications marked as read",
	})
}

// DeleteNotification godoc
// @Summary Delete a notification
// @Description Delete a single notification
// @Tags notifications
// @Accept json
// @Produce json
// @Param comId path string false "Community ID (NDC ID)"
// @Param id path string true "Notification ID"
// @Success 200 {object} nil
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/notification/{id} [delete]
// @Router /x{comId}/s/notification/{id} [delete]
func DeleteNotification(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	auid := middleware.GetAUIDFromContext(c)
	notificationId := c.Params("id")

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	svc := service.NewNotificationService(db, Hub)
	err := svc.DeleteNotification(notificationId, auid)
	if err != nil {
		if err == service.ErrNotificationNotFound {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Notification deleted",
	})
}

// AcknowledgeNotification godoc
// @Summary Acknowledge an important notification
// @Description Mark an important notification as acknowledged (stops bottom toast)
// @Tags notifications
// @Accept json
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} response.ErrorResponse
// @Router /g/s/notification/{id}/acknowledge [post]
func AcknowledgeNotification(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	auid := middleware.GetAUIDFromContext(c)
	notificationId := c.Params("id")

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	svc := service.NewNotificationService(db, Hub)
	err := svc.AcknowledgeNotification(notificationId, auid)
	if err != nil {
		if err == service.ErrNotificationNotFound {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Notification acknowledged",
	})
}

// GetImportantNotifications godoc
// @Summary Get unacknowledged important notifications
// @Description Get unacknowledged important notifications for the toast system
// @Tags notifications
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /g/s/notification/important [get]
func GetImportantNotifications(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	auid := middleware.GetAUIDFromContext(c)
	ndcId := getNdcId(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	svc := service.NewNotificationService(db, Hub)
	notifications, err := svc.GetUnacknowledgedImportant(auid, ndcId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"notificationList": notifications,
	})
}

// DeleteAllNotifications godoc
// @Summary Delete all notifications
// @Description Delete all notifications for the authenticated user
// @Tags notifications
// @Accept json
// @Produce json
// @Param comId path string false "Community ID (NDC ID)"
// @Success 200 {object} nil
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/notification [delete]
// @Router /x{comId}/s/notification [delete]
func DeleteAllNotifications(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	auid := middleware.GetAUIDFromContext(c)
	ndcId := getNdcId(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.InvalidRequest().StatusCode))
	}

	svc := service.NewNotificationService(db, Hub)
	err := svc.DeleteAllNotifications(auid, ndcId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "All notifications deleted",
	})
}
