package push

import (
	"context"
	"fmt"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
	"gorm.io/gorm"

	"github.com/AugustLigh/GoMino/internal/api/device"
	"github.com/AugustLigh/GoMino/internal/models"
	"github.com/AugustLigh/GoMino/internal/models/user"
)

// PushNotificationService handles sending push notifications via Firebase Cloud Messaging
type PushNotificationService struct {
	db            *gorm.DB
	messagingClient *messaging.Client
	ctx           context.Context
}

// NewPushNotificationService creates a new push notification service
// serviceAccountPath - path to Firebase service account JSON file
func NewPushNotificationService(db *gorm.DB, serviceAccountPath string) (*PushNotificationService, error) {
	ctx := context.Background()

	// Initialize Firebase App with service account
	opt := option.WithCredentialsFile(serviceAccountPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("error initializing Firebase app: %v", err)
	}

	// Get messaging client
	messagingClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting Messaging client: %v", err)
	}

	log.Println("Push Notification Service initialized successfully")

	return &PushNotificationService{
		db:              db,
		messagingClient: messagingClient,
		ctx:             ctx,
	}, nil
}

// NotificationData contains data for push notification
type NotificationData struct {
	Title            string
	Body             string
	NotificationID   string
	Type             int    // NotificationType value
	ObjectType       int    // ObjectType value
	ObjectID         string
	NdcID            int
	ImageURL         string // Optional notification icon/image
}

// SendPushToUser sends a push notification to all devices of a specific user
func (s *PushNotificationService) SendPushToUser(userID string, data NotificationData) error {
	// Check if user has push notifications enabled
	var u user.User
	if err := s.db.Select("push_enabled").Where("uid = ? AND ndc_id = 0", userID).First(&u).Error; err != nil {
		// User not found or error - skip push
		log.Printf("Error checking push settings for user %s: %v", userID, err)
		return nil
	}

	// Skip if push notifications are disabled
	if !u.PushEnabled {
		log.Printf("Push notifications disabled for user %s, skipping", userID)
		return nil
	}

	// Get all device tokens for this user
	tokens, err := device.GetUserDeviceTokens(s.db, userID)
	if err != nil {
		return fmt.Errorf("error getting device tokens: %v", err)
	}

	if len(tokens) == 0 {
		log.Printf("No device tokens found for user %s", userID)
		return nil // Not an error - user just doesn't have devices registered
	}

	// Extract token strings
	tokenStrings := make([]string, len(tokens))
	for i, token := range tokens {
		tokenStrings[i] = token.Token
	}

	// Send multicast message
	return s.SendMulticast(tokenStrings, data)
}

// SendPushToToken sends a push notification to a specific device token
func (s *PushNotificationService) SendPushToToken(token string, data NotificationData) error {
	message := s.buildMessage(token, data)

	// Send the message
	response, err := s.messagingClient.Send(s.ctx, message)
	if err != nil {
		// Check if token is invalid
		if messaging.IsRegistrationTokenNotRegistered(err) ||
			messaging.IsInvalidArgument(err) {
			log.Printf("Invalid or unregistered token: %s - %v", token, err)
			// Delete invalid token from database
			s.deleteInvalidToken(token)
			return nil // Don't return error for invalid tokens
		}
		return fmt.Errorf("error sending push notification: %v", err)
	}

	log.Printf("Successfully sent push notification: %s", response)
	return nil
}

// SendMulticast sends a push notification to multiple device tokens
func (s *PushNotificationService) SendMulticast(tokens []string, data NotificationData) error {
	if len(tokens) == 0 {
		return nil
	}

	// Build multicast message
	message := &messaging.MulticastMessage{
		Notification: &messaging.Notification{
			Title: data.Title,
			Body:  data.Body,
		},
		Data: map[string]string{
			"notificationId": data.NotificationID,
			"type":           fmt.Sprintf("%d", data.Type),
			"objectType":     fmt.Sprintf("%d", data.ObjectType),
			"objectId":       data.ObjectID,
			"ndcId":          fmt.Sprintf("%d", data.NdcID),
		},
		Tokens: tokens,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "high_importance_channel",
				Priority:  messaging.PriorityHigh,
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: data.Title,
						Body:  data.Body,
					},
					Badge: nil, // You can set badge count here if needed
					Sound: "default",
				},
			},
		},
		Webpush: &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title: data.Title,
				Body:  data.Body,
				Icon:  data.ImageURL,
			},
		},
	}

	// Add image if provided
	if data.ImageURL != "" {
		message.Notification.ImageURL = data.ImageURL
	}

	// Send multicast
	batchResponse, err := s.messagingClient.SendEachForMulticast(s.ctx, message)
	if err != nil {
		return fmt.Errorf("error sending multicast: %v", err)
	}

	// Check for failures and remove invalid tokens
	if batchResponse.FailureCount > 0 {
		var failedTokens []string
		for idx, resp := range batchResponse.Responses {
			if !resp.Success {
				log.Printf("Failed to send to token %s: %v", tokens[idx], resp.Error)
				if messaging.IsRegistrationTokenNotRegistered(resp.Error) ||
					messaging.IsInvalidArgument(resp.Error) {
					failedTokens = append(failedTokens, tokens[idx])
				}
			}
		}

		// Delete invalid tokens
		for _, token := range failedTokens {
			s.deleteInvalidToken(token)
		}
	}

	log.Printf("Successfully sent %d/%d push notifications", batchResponse.SuccessCount, len(tokens))
	return nil
}

// buildMessage builds a Firebase message for a single token
func (s *PushNotificationService) buildMessage(token string, data NotificationData) *messaging.Message {
	return &messaging.Message{
		Notification: &messaging.Notification{
			Title:    data.Title,
			Body:     data.Body,
			ImageURL: data.ImageURL,
		},
		Data: map[string]string{
			"notificationId": data.NotificationID,
			"type":           fmt.Sprintf("%d", data.Type),
			"objectType":     fmt.Sprintf("%d", data.ObjectType),
			"objectId":       data.ObjectID,
			"ndcId":          fmt.Sprintf("%d", data.NdcID),
		},
		Token: token,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "high_importance_channel",
				Priority:  messaging.PriorityHigh,
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: data.Title,
						Body:  data.Body,
					},
					Sound: "default",
				},
			},
		},
		Webpush: &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title: data.Title,
				Body:  data.Body,
				Icon:  data.ImageURL,
			},
		},
	}
}

// deleteInvalidToken removes an invalid token from the database
func (s *PushNotificationService) deleteInvalidToken(token string) {
	if err := s.db.Where("token = ?", token).Delete(&models.DeviceToken{}).Error; err != nil {
		log.Printf("Error deleting invalid token: %v", err)
	} else {
		log.Printf("Deleted invalid token: %s", token)
	}
}
