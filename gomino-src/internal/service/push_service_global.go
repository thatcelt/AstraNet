package service

import (
	pushService "github.com/AugustLigh/GoMino/internal/service/push"
)

// Global push notification service instance
var globalPushService *pushService.PushNotificationService

// SetPushService sets the global push notification service instance
func SetPushService(svc *pushService.PushNotificationService) {
	globalPushService = svc
}

// GetPushService returns the global push notification service instance
func GetPushService() *pushService.PushNotificationService {
	return globalPushService
}

// IsPushEnabled returns true if push notifications are enabled
func IsPushEnabled() bool {
	return globalPushService != nil
}
