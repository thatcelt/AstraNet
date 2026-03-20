package ws

import (
	"sync"
	"time"
)

// PresenceType represents where the user currently is
type PresenceType string

const (
	PresenceOnline    PresenceType = "online"     // Just online, no specific location
	PresenceInChat    PresenceType = "in_chat"    // Viewing a specific chat
	PresenceInBlog    PresenceType = "in_blog"    // Viewing a specific blog post
	PresenceInProfile PresenceType = "in_profile" // Viewing a user profile
)

// UserPresence represents a user's current activity state
type UserPresence struct {
	UserID      string       `json:"userId"`
	CommunityID int          `json:"communityId"`
	Type        PresenceType `json:"type"`
	TargetID    string       `json:"targetId,omitempty"` // chatId, blogId, or profileId depending on type
	UpdatedAt   time.Time    `json:"updatedAt"`
	UserIcon    string       `json:"userIcon,omitempty"`
	Nickname    string       `json:"nickname,omitempty"`
}

// CommunityActivity represents aggregated activity data for a community
type CommunityActivity struct {
	CommunityID  int                  `json:"communityId"`
	OnlineCount  int                  `json:"onlineCount"`
	TopChats     []ChatActivity       `json:"topChats"`
	TopBlogs     []BlogActivity       `json:"topBlogs"`
	OnlineUsers  []OnlineUserInfo     `json:"onlineUsers"`
}

// ChatActivity represents activity in a specific chat
type ChatActivity struct {
	ChatID       string `json:"chatId"`
	ChatTitle    string `json:"chatTitle,omitempty"`
	ChatIcon     string `json:"chatIcon,omitempty"`
	ActiveCount  int    `json:"activeCount"`
}

// BlogActivity represents activity on a specific blog post
type BlogActivity struct {
	BlogID       string `json:"blogId"`
	BlogTitle    string `json:"blogTitle,omitempty"`
	BlogIcon     string `json:"blogIcon,omitempty"`
	ActiveCount  int    `json:"activeCount"`
}

// OnlineUserInfo represents minimal user info for online display
type OnlineUserInfo struct {
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	Icon     string `json:"icon,omitempty"`
}

// PresenceManager manages user presence states
type PresenceManager struct {
	// Map of userID -> UserPresence
	presences map[string]*UserPresence

	// Map of communityID -> set of userIDs
	communityUsers map[int]map[string]bool

	// Map of chatID -> set of userIDs (for chat activity)
	chatUsers map[string]map[string]bool

	// Map of blogID -> set of userIDs (for blog activity)
	blogUsers map[string]map[string]bool

	mu sync.RWMutex
}

// NewPresenceManager creates a new PresenceManager
func NewPresenceManager() *PresenceManager {
	return &PresenceManager{
		presences:      make(map[string]*UserPresence),
		communityUsers: make(map[int]map[string]bool),
		chatUsers:      make(map[string]map[string]bool),
		blogUsers:      make(map[string]map[string]bool),
	}
}

// UpdatePresence updates a user's presence state
func (pm *PresenceManager) UpdatePresence(userID string, communityID int, presenceType PresenceType, targetID string, nickname string, icon string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Get old presence to clean up
	oldPresence := pm.presences[userID]
	if oldPresence != nil {
		// Remove from old community
		if oldPresence.CommunityID != communityID && oldPresence.CommunityID > 0 {
			if users, ok := pm.communityUsers[oldPresence.CommunityID]; ok {
				delete(users, userID)
				if len(users) == 0 {
					delete(pm.communityUsers, oldPresence.CommunityID)
				}
			}
		}
		// Remove from old chat
		if oldPresence.Type == PresenceInChat && oldPresence.TargetID != "" {
			if users, ok := pm.chatUsers[oldPresence.TargetID]; ok {
				delete(users, userID)
				if len(users) == 0 {
					delete(pm.chatUsers, oldPresence.TargetID)
				}
			}
		}
		// Remove from old blog
		if oldPresence.Type == PresenceInBlog && oldPresence.TargetID != "" {
			if users, ok := pm.blogUsers[oldPresence.TargetID]; ok {
				delete(users, userID)
				if len(users) == 0 {
					delete(pm.blogUsers, oldPresence.TargetID)
				}
			}
		}
	}

	// Create new presence
	pm.presences[userID] = &UserPresence{
		UserID:      userID,
		CommunityID: communityID,
		Type:        presenceType,
		TargetID:    targetID,
		UpdatedAt:   time.Now(),
		UserIcon:    icon,
		Nickname:    nickname,
	}

	// Add to community users
	if communityID > 0 {
		if _, ok := pm.communityUsers[communityID]; !ok {
			pm.communityUsers[communityID] = make(map[string]bool)
		}
		pm.communityUsers[communityID][userID] = true
	}

	// Add to chat users if in chat
	if presenceType == PresenceInChat && targetID != "" {
		if _, ok := pm.chatUsers[targetID]; !ok {
			pm.chatUsers[targetID] = make(map[string]bool)
		}
		pm.chatUsers[targetID][userID] = true
	}

	// Add to blog users if viewing blog
	if presenceType == PresenceInBlog && targetID != "" {
		if _, ok := pm.blogUsers[targetID]; !ok {
			pm.blogUsers[targetID] = make(map[string]bool)
		}
		pm.blogUsers[targetID][userID] = true
	}
}

// RemoveUser removes a user from presence tracking (on disconnect)
func (pm *PresenceManager) RemoveUser(userID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	presence := pm.presences[userID]
	if presence == nil {
		return
	}

	// Remove from community
	if presence.CommunityID > 0 {
		if users, ok := pm.communityUsers[presence.CommunityID]; ok {
			delete(users, userID)
			if len(users) == 0 {
				delete(pm.communityUsers, presence.CommunityID)
			}
		}
	}

	// Remove from chat
	if presence.Type == PresenceInChat && presence.TargetID != "" {
		if users, ok := pm.chatUsers[presence.TargetID]; ok {
			delete(users, userID)
			if len(users) == 0 {
				delete(pm.chatUsers, presence.TargetID)
			}
		}
	}

	// Remove from blog
	if presence.Type == PresenceInBlog && presence.TargetID != "" {
		if users, ok := pm.blogUsers[presence.TargetID]; ok {
			delete(users, userID)
			if len(users) == 0 {
				delete(pm.blogUsers, presence.TargetID)
			}
		}
	}

	delete(pm.presences, userID)
}

// GetCommunityActivity returns aggregated activity for a community
func (pm *PresenceManager) GetCommunityActivity(communityID int, limit int) CommunityActivity {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	activity := CommunityActivity{
		CommunityID: communityID,
		TopChats:    make([]ChatActivity, 0),
		TopBlogs:    make([]BlogActivity, 0),
		OnlineUsers: make([]OnlineUserInfo, 0),
	}

	// Get online users in this community
	users, ok := pm.communityUsers[communityID]
	if !ok {
		return activity
	}

	activity.OnlineCount = len(users)

	// Collect online users info
	for userID := range users {
		if presence, ok := pm.presences[userID]; ok {
			activity.OnlineUsers = append(activity.OnlineUsers, OnlineUserInfo{
				UserID:   userID,
				Nickname: presence.Nickname,
				Icon:     presence.UserIcon,
			})
			if len(activity.OnlineUsers) >= limit {
				break
			}
		}
	}

	// Calculate chat activity
	chatCounts := make(map[string]int)
	for chatID, chatUsers := range pm.chatUsers {
		// Count only users from this community in this chat
		count := 0
		for userID := range chatUsers {
			if presence, ok := pm.presences[userID]; ok {
				if presence.CommunityID == communityID {
					count++
				}
			}
		}
		if count > 0 {
			chatCounts[chatID] = count
		}
	}

	// Sort chats by activity
	for chatID, count := range chatCounts {
		activity.TopChats = append(activity.TopChats, ChatActivity{
			ChatID:      chatID,
			ActiveCount: count,
		})
	}

	// Sort by active count descending
	for i := 0; i < len(activity.TopChats)-1; i++ {
		for j := i + 1; j < len(activity.TopChats); j++ {
			if activity.TopChats[j].ActiveCount > activity.TopChats[i].ActiveCount {
				activity.TopChats[i], activity.TopChats[j] = activity.TopChats[j], activity.TopChats[i]
			}
		}
	}

	// Limit top chats
	if len(activity.TopChats) > limit {
		activity.TopChats = activity.TopChats[:limit]
	}

	// Calculate blog activity
	blogCounts := make(map[string]int)
	for blogID, blogUsers := range pm.blogUsers {
		// Count only users from this community viewing this blog
		count := 0
		for userID := range blogUsers {
			if presence, ok := pm.presences[userID]; ok {
				if presence.CommunityID == communityID {
					count++
				}
			}
		}
		if count > 0 {
			blogCounts[blogID] = count
		}
	}

	// Sort blogs by activity
	for blogID, count := range blogCounts {
		activity.TopBlogs = append(activity.TopBlogs, BlogActivity{
			BlogID:      blogID,
			ActiveCount: count,
		})
	}

	// Sort by active count descending
	for i := 0; i < len(activity.TopBlogs)-1; i++ {
		for j := i + 1; j < len(activity.TopBlogs); j++ {
			if activity.TopBlogs[j].ActiveCount > activity.TopBlogs[i].ActiveCount {
				activity.TopBlogs[i], activity.TopBlogs[j] = activity.TopBlogs[j], activity.TopBlogs[i]
			}
		}
	}

	// Limit top blogs
	if len(activity.TopBlogs) > limit {
		activity.TopBlogs = activity.TopBlogs[:limit]
	}

	return activity
}

// GetChatOnlineCount returns the number of users currently viewing a chat
func (pm *PresenceManager) GetChatOnlineCount(chatID string) int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if users, ok := pm.chatUsers[chatID]; ok {
		return len(users)
	}
	return 0
}

// GetUserPresence returns a user's current presence
func (pm *PresenceManager) GetUserPresence(userID string) *UserPresence {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.presences[userID]
}

// IsUserOnline checks if a user is currently online
func (pm *PresenceManager) IsUserOnline(userID string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	_, ok := pm.presences[userID]
	return ok
}
