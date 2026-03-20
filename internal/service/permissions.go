package service

import (
	"github.com/AugustLigh/GoMino/internal/models/chat"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"gorm.io/gorm"
)

// PermissionService handles role-based access control
type PermissionService struct {
	db *gorm.DB
}

func NewPermissionService(db *gorm.DB) *PermissionService {
	return &PermissionService{db: db}
}

// IsAstranetTeam checks if a user is an Astranet Team member (by is_astranet flag on global profile)
func (s *PermissionService) IsAstranetTeam(userUID string) bool {
	var globalUser user.User
	if err := s.db.Where("uid = ? AND ndc_id = 0", userUID).First(&globalUser).Error; err != nil {
		return false
	}
	return globalUser.IsAstranet
}

// GetEffectiveRole returns the highest role a user has in a given context (Community + Thread)
// If threadID is empty, chat roles are ignored.
// If ndcID is 0, only global roles are considered.
// Note: Astranet Team status is NOT reflected here — use IsAstranetTeam() for global permissions.
func (s *PermissionService) GetEffectiveRole(userUID string, ndcID int, threadID string) int {
	maxRole := user.RoleMember

	// 0. Check Global Ban Table FIRST (takes priority over everything)
	var globalBan user.GlobalBan
	if err := s.db.Where("uid = ? AND is_active = true", userUID).First(&globalBan).Error; err == nil {
		return user.RoleBanned // Globally banned
	}

	// 1. Check Global Ban via role
	var globalUser user.User
	if err := s.db.Where("uid = ? AND ndc_id = 0", userUID).First(&globalUser).Error; err == nil {
		if globalUser.Role == user.RoleBanned {
			return user.RoleBanned
		}
	}

	// 2. Check Community Role
	if ndcID > 0 {
		var comUser user.User
		if err := s.db.Where("uid = ? AND ndc_id = ?", userUID, ndcID).First(&comUser).Error; err == nil {
			if comUser.Role == user.RoleBanned {
				return user.RoleBanned // Banned in community overrides any chat role
			}
			if comUser.Role > maxRole {
				maxRole = comUser.Role
			}
		}
	}

	// 3. Check Chat Role
	if threadID != "" {
		var member chat.ThreadMember
		if err := s.db.Where("thread_id = ? AND user_uid = ?", threadID, userUID).First(&member).Error; err == nil {
			// Convert Chat Role to int and compare
			chatRoleInt := int(member.Role)
			if chatRoleInt > maxRole {
				maxRole = chatRoleInt
			}
		}
	}

	return maxRole
}

// IsBanned checks whether a user is banned globally or in a specific community.
// More efficient than GetEffectiveRole when you only need the ban status.
func (s *PermissionService) IsBanned(userUID string, ndcID int) bool {
	// 1. Check Global Ban Table (highest priority)
	var count int64
	s.db.Model(&user.GlobalBan{}).Where("uid = ? AND is_active = true", userUID).Count(&count)
	if count > 0 {
		return true
	}

	// 2. Check global role
	var globalUser user.User
	if err := s.db.Where("uid = ? AND ndc_id = 0", userUID).Select("role").First(&globalUser).Error; err == nil {
		if globalUser.Role == user.RoleBanned {
			return true
		}
	}

	// 3. Check community role
	if ndcID > 0 {
		var comUser user.User
		if err := s.db.Where("uid = ? AND ndc_id = ?", userUID, ndcID).Select("role").First(&comUser).Error; err == nil {
			if comUser.Role == user.RoleBanned {
				return true
			}
		}
	}

	return false
}

// CanPerform returns true if actor has permissions to perform action on target
// requireRole: minimum role required (e.g. RoleCurator to delete message of others)
// allowSelf: if true, actor can manage their own content
func (s *PermissionService) CanPerform(actorUID string, targetUID string, ndcID int, threadID string, requiredRole int, allowSelf bool) bool {
	// 1. Get Actor Effective Role
	actorRole := s.GetEffectiveRole(actorUID, ndcID, threadID)

	// Banned users can do nothing
	if actorRole == user.RoleBanned {
		return false
	}

	// 2. Self-Management Check
	if allowSelf && actorUID == targetUID {
		return true
	}

	// 3. Check against Required Role (Minimum Threshold)
	if actorRole < requiredRole {
		return false
	}

	// 4. Check against Target Role (Hierarchy Check)
	// You cannot manage someone with a higher or equal role
	if targetUID != "" && targetUID != actorUID {
		targetRole := s.GetEffectiveRole(targetUID, ndcID, threadID)
		if actorRole <= targetRole {
			return false
		}
	}

	return true
}
