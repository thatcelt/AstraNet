package service

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/AugustLigh/GoMino/internal/models/community"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

var (
	ErrCommunityNotFound = errors.New("community not found")
	ErrAlreadyJoined     = errors.New("already joined this community")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrAlreadyFeatured   = errors.New("community is already featured")
)

type CommunityService struct {
	db *gorm.DB
}

func NewCommunityService(db *gorm.DB) *CommunityService {
	return &CommunityService{db: db}
}

// GenerateNDCId generates a random 10-digit number
func (s *CommunityService) GenerateNDCId() int {
	rand.Seed(time.Now().UnixNano())
	min := 1000000000
	max := 9999999999
	return rand.Intn(max-min+1) + min
}

// CreateCommunity creates a new community and joins the creator as Agent
func (s *CommunityService) CreateCommunity(com *community.Community, creatorUID string) (*community.Community, error) {
	// 1. Get Global User Profile to copy data
	var globalUser user.User
	if err := s.db.Where("uid = ? AND ndc_id = 0", creatorUID).First(&globalUser).Error; err != nil {
		return nil, fmt.Errorf("creator global profile not found: %w", err)
	}

	// 2. Transaction
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Generate Unique NDCId
		for {
			com.NdcId = s.GenerateNDCId()
			var count int64
			tx.Model(&community.Community{}).Where("ndc_id = ?", com.NdcId).Count(&count)
			if count == 0 {
				break
			}
		}

		// Ensure Endpoint/Link uniqueness (simple strategy if not provided)
		if com.Endpoint == "" {
			com.Endpoint = fmt.Sprintf("%d", com.NdcId)
		}
		if com.Link == "" {
			com.Link = fmt.Sprintf("/c/%s", com.Endpoint)
		}

		now := utils.CustomTime{Time: time.Now()}
		com.CreatedTime = now
		com.UpdatedTime = now
		com.ModifiedTime = now
		com.MembersCount = 1 // Agent is the first member

		// Agent Profile Data
		agentProfile := user.User{
			UID:          creatorUID,
			NdcID:        com.NdcId,
			Nickname:     globalUser.Nickname,
			Icon:         globalUser.Icon,
			Role:         user.RoleAgent, // Agent Role (110)
			Status:       0,
			Level:        1,
			Reputation:   0,
			CreatedTime:  now,
			ModifiedTime: now,
			AminoID:      fmt.Sprintf("%s_%d", globalUser.AminoID, com.NdcId),
		}

		// Set Agent in Community
		com.Agent = user.DetailedAuthor{
			Author: user.Author{
				UID:        agentProfile.UID,
				Nickname:   agentProfile.Nickname,
				Icon:       agentProfile.Icon,
				Level:      agentProfile.Level,
				Reputation: agentProfile.Reputation,
				Role:       agentProfile.Role,
				Status:     agentProfile.Status,
			},
			AminoID:  agentProfile.AminoID,
			IsGlobal: agentProfile.IsGlobal,
		}

		// Create Community
		if err := tx.Create(com).Error; err != nil {
			return err
		}

		// Create Agent Profile
		if err := tx.Create(&agentProfile).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return com, nil
}

// TransferAgent transfers agent status to another user
func (s *CommunityService) TransferAgent(ndcId int, requesterUID, newAgentUID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var com community.Community
		if err := tx.Where("ndc_id = ?", ndcId).First(&com).Error; err != nil {
			return ErrCommunityNotFound
		}

		permSvc := NewPermissionService(tx)
		// Для передачи прав Агента нужен уровень Агента (110).
		// allowSelf=true разрешает текущему Агенту (com.Agent.UID) выполнить действие над собой (своей должностью).
		// В данном случае 'requesterUID' действует, 'targetUID' (на кого мы проверяем права для действия) - это текущий агент.
		// Логика: Может ли 'requesterUID' управлять должностью 'com.Agent.UID'?
		// Если requesterUID == com.Agent.UID -> Self manage -> OK.
		// Если requesterUID == Astranet (1000) -> 1000 > 110 (Agent) -> OK.
		if !permSvc.CanPerform(requesterUID, com.Agent.UID, ndcId, "", user.RoleAgent, true) {
			return ErrPermissionDenied
		}

		// Get new agent user
		var newAgentUser user.User
		if err := tx.Where("uid = ? AND ndc_id = ?", newAgentUID, ndcId).First(&newAgentUser).Error; err != nil {
			return fmt.Errorf("new agent user not found: %w", err)
		}

		// Downgrade old agent to Leader
		oldAgentUID := com.Agent.UID
		if oldAgentUID != "" {
			if err := tx.Model(&user.User{}).Where("uid = ? AND ndc_id = ?", oldAgentUID, ndcId).Update("role", user.RoleLeader).Error; err != nil {
				return err
			}
		}

		// Update new agent role to Agent (110)
		if err := tx.Model(&newAgentUser).Update("role", user.RoleAgent).Error; err != nil {
			return err
		}

		// Update Community Agent
		newAgentAuthor := user.DetailedAuthor{
			Author: user.Author{
				UID:        newAgentUser.UID,
				Nickname:   newAgentUser.Nickname,
				Icon:       newAgentUser.Icon,
				Level:      newAgentUser.Level,
				Reputation: newAgentUser.Reputation,
				Role:       user.RoleAgent,
				Status:     newAgentUser.Status,
			},
			AminoID:  newAgentUser.AminoID,
			IsGlobal: newAgentUser.IsGlobal,
		}
		
		com.Agent = newAgentAuthor
		return tx.Save(&com).Error
	})
}

// PromoteToLeader promotes a user to Leader
func (s *CommunityService) PromoteToLeader(ndcId int, requesterUID, targetUID string) error {
	permSvc := NewPermissionService(s.db)
	
	// Чтобы назначить Лидера (100), нужен уровень Агента (110).
	// Целевой пользователь (targetUID) не должен быть выше или равен нам по званию.
	if !permSvc.CanPerform(requesterUID, targetUID, ndcId, "", user.RoleAgent, false) {
		return ErrPermissionDenied
	}

	return s.db.Model(&user.User{}).
		Where("uid = ? AND ndc_id = ?", targetUID, ndcId).
		Update("role", user.RoleLeader).Error
}

// PromoteToCurator promotes a user to Curator
func (s *CommunityService) PromoteToCurator(ndcId int, requesterUID, targetUID string) error {
	permSvc := NewPermissionService(s.db)

	// Чтобы назначить Куратора (50), нужен уровень Лидера (100).
	if !permSvc.CanPerform(requesterUID, targetUID, ndcId, "", user.RoleLeader, false) {
		return ErrPermissionDenied
	}

	return s.db.Model(&user.User{}).
		Where("uid = ? AND ndc_id = ?", targetUID, ndcId).
		Update("role", user.RoleCurator).Error
}

// GetCommunity returns community by ID (int) or Endpoint (string)
func (s *CommunityService) GetCommunity(id interface{}) (*community.Community, error) {
	// Handle Global Community (ID 0)
	if val, ok := id.(int); ok && val == 0 {
		return &community.Community{
			NdcId:    0,
			Name:     "Global",
			Endpoint: "global",
			Link:     "http://aminoapps.com/g",
			Status:   0,
		}, nil
	}

	var com community.Community
	query := s.db.Model(&community.Community{})

	switch v := id.(type) {
	case int:
		query = query.Where("ndc_id = ?", v)
	case string:
		query = query.Where("endpoint = ?", v) // or Link
	default:
		return nil, errors.New("invalid id type")
	}

	err := query.First(&com).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCommunityNotFound
		}
		return nil, err
	}
	return &com, nil
}

// JoinCommunity creates a user profile in the community
func (s *CommunityService) JoinCommunity(ndcId int, uid string) error {
	fmt.Printf("[JoinCommunity Service] Starting for uid=%s, ndcId=%d\n", uid, ndcId)

	// 1. Check Community exists
	var com community.Community
	if err := s.db.Where("ndc_id = ?", ndcId).First(&com).Error; err != nil {
		fmt.Printf("[JoinCommunity Service] Community not found: %v\n", err)
		return ErrCommunityNotFound
	}
	fmt.Printf("[JoinCommunity Service] Community found: %s\n", com.Name)

	// 2. Check if already joined
	var count int64
	s.db.Model(&user.User{}).Where("uid = ? AND ndc_id = ?", uid, ndcId).Count(&count)
	if count > 0 {
		fmt.Printf("[JoinCommunity Service] User already joined\n")
		return ErrAlreadyJoined
	}

	// 3. Get Global Profile
	var globalUser user.User
	if err := s.db.Where("uid = ? AND ndc_id = 0", uid).First(&globalUser).Error; err != nil {
		fmt.Printf("[JoinCommunity Service] Global profile NOT found for uid=%s: %v\n", uid, err)
		return fmt.Errorf("global profile not found: %w", err)
	}
	fmt.Printf("[JoinCommunity Service] Global profile found: %s\n", globalUser.Nickname)

	// 4. Create Profile
	now := utils.CustomTime{Time: time.Now()}
	newProfile := user.User{
		UID:          uid,
		NdcID:        ndcId,
		Nickname:     globalUser.Nickname,
		Icon:         globalUser.Icon,
		Role:         0, // Member
		Level:        1,
		CreatedTime:  now,
		ModifiedTime: now,
		// See note about AminoID above
		AminoID: fmt.Sprintf("%s_%d", globalUser.AminoID, ndcId),
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&newProfile).Error; err != nil {
			return err
		}
		// Increment Member Count
		if err := tx.Model(&community.Community{}).Where("ndc_id = ?", ndcId).Update("members_count", gorm.Expr("members_count + 1")).Error; err != nil {
			return err
		}
		return nil
	})
}

// GetJoinedCommunities returns list of communities a user has joined
func (s *CommunityService) GetJoinedCommunities(uid string, start, size int) ([]community.Community, error) {
	var profiles []user.User
	// Get all profiles for this user where NdcID != 0
	err := s.db.Where("uid = ? AND ndc_id != 0", uid).
		Offset(start).Limit(size).
		Find(&profiles).Error
	
	if err != nil {
		return nil, err
	}

	if len(profiles) == 0 {
		return []community.Community{}, nil
	}

	var ndcIds []int
	for _, p := range profiles {
		ndcIds = append(ndcIds, p.NdcID)
	}

	var communities []community.Community
	if err := s.db.Where("ndc_id IN ?", ndcIds).Find(&communities).Error; err != nil {
		return nil, err
	}

	return communities, nil
}

// Admin / Moderation Methods

// BanUser bans a user from the community (sets Role to -1)
func (s *CommunityService) BanUser(ndcId int, requesterUID, targetUID, reason string) error {
	permSvc := NewPermissionService(s.db)
	
	// Ban requires Leader (100) or higher.
	// CanPerform checks if requester > target.
	if !permSvc.CanPerform(requesterUID, targetUID, ndcId, "", user.RoleLeader, false) {
		return ErrPermissionDenied
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Update Role to Banned (-1)
		if err := tx.Model(&user.User{}).
			Where("uid = ? AND ndc_id = ?", targetUID, ndcId).
			Update("role", user.RoleBanned).Error; err != nil {
			return err
		}

		// Log Action
		return s.logAction(tx, ndcId, requesterUID, targetUID, targetUID, community.ObjectTypeUser, community.OpTypeBan, reason)
	})
}

// UnbanUser unbans a user (sets Role to 0 - Member)
func (s *CommunityService) UnbanUser(ndcId int, requesterUID, targetUID, reason string) error {
	permSvc := NewPermissionService(s.db)

	// Unban requires Leader (100) or higher.
	// CanPerform checks if requester > target (banned user is -1, so Leader(100) > -1 is True).
	if !permSvc.CanPerform(requesterUID, targetUID, ndcId, "", user.RoleLeader, false) {
		return ErrPermissionDenied
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Update Role to Member (0)
		if err := tx.Model(&user.User{}).
			Where("uid = ? AND ndc_id = ?", targetUID, ndcId).
			Update("role", user.RoleMember).Error; err != nil {
			return err
		}

		// Log Action
		return s.logAction(tx, ndcId, requesterUID, targetUID, targetUID, community.ObjectTypeUser, community.OpTypeUnban, reason)
	})
}

// GetModerationHistory returns admin logs with filtering
func (s *CommunityService) GetModerationHistory(ndcId int, objectID *string, objectType *int, start, size int) ([]community.AdminLog, error) {
	var logs []community.AdminLog
	
	query := s.db.Preload("Operator").Preload("Target").
		Where("ndc_id = ?", ndcId).
		Order("created_time DESC, id DESC").
		Offset(start).Limit(size)

	if objectID != nil && *objectID != "" {
		query = query.Where("object_id = ?", *objectID)
	}
	if objectType != nil {
		query = query.Where("object_type = ?", *objectType)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	
	return logs, nil
}

// UpdateCommunitySettings updates community settings
func (s *CommunityService) UpdateCommunitySettings(ndcId int, requesterUID string, updates map[string]interface{}) error {
	permSvc := NewPermissionService(s.db)

	// Settings update requires Leader (100) or higher.
	// targetUID is empty because we check against the community role threshold.
	if !permSvc.CanPerform(requesterUID, "", ndcId, "", user.RoleLeader, false) {
		return ErrPermissionDenied
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var com community.Community
		if err := tx.Where("ndc_id = ?", ndcId).First(&com).Error; err != nil {
			return ErrCommunityNotFound
		}

		// Handle ThemePack update with merge
		if themePackUpdate, ok := updates["theme_pack_update"].(map[string]*string); ok {
			// Initialize ThemePack if nil
			if com.ThemePack == nil {
				com.ThemePack = &community.ThemePack{}
			}
			// Merge fields
			if v, exists := themePackUpdate["themeColor"]; exists && v != nil {
				com.ThemePack.ThemeColor = *v
			}
			if v, exists := themePackUpdate["themeSideImage"]; exists && v != nil {
				com.ThemePack.ThemeSideImage = *v
			}
			if v, exists := themePackUpdate["themeUpperImage"]; exists && v != nil {
				com.ThemePack.ThemeUpperImage = *v
			}
			if v, exists := themePackUpdate["cover"]; exists && v != nil {
				com.ThemePack.Cover = *v
			}
			// Don't use Updates() for serialized fields, apply directly to model
			delete(updates, "theme_pack_update")
		}

		// Apply simple field updates
		if name, ok := updates["name"].(string); ok {
			com.Name = name
		}
		if icon, ok := updates["icon"].(string); ok {
			com.Icon = icon
		}
		if content, ok := updates["content"].(string); ok {
			com.Content = &content
		}
		if endpoint, ok := updates["endpoint"].(string); ok {
			com.Endpoint = endpoint
		}
		if lang, ok := updates["primary_language"].(string); ok {
			com.PrimaryLanguage = lang
		}
		// Handle searchable - update the com object so it's saved with Updates below
		if searchable, ok := updates["searchable"].(bool); ok {
			com.Searchable = searchable
			fmt.Printf("[DEBUG] Searchable update: ndcId=%d, value=%v\n", ndcId, searchable)
		}

		// Update timestamps
		now := utils.CustomTime{Time: time.Now()}
		com.ModifiedTime = now
		com.UpdatedTime = now

		// Use Updates() with Select("*") to update all fields including serialized ones
		// We need to specify WHERE condition since Community model doesn't have ID field
		if err := tx.Model(&community.Community{}).Where("ndc_id = ?", ndcId).Select("*").Updates(&com).Error; err != nil {
			return err
		}

		// Log Action (Optional, but good for transparency)
		return s.logAction(tx, ndcId, requesterUID, "", fmt.Sprintf("%d", ndcId), community.ObjectTypeCommunity, community.OpTypeEdit, "Update settings")
	})
}

// Internal helper to log admin actions
func (s *CommunityService) logAction(tx *gorm.DB, ndcId int, opUID, targetUID, objID string, objType, opType int, note string) error {
	logEntry := community.AdminLog{
		NdcID:         ndcId,
		OperatorUID:   opUID,
		TargetUID:     targetUID,
		ObjectID:      &objID,
		ObjectType:    objType,
		OperationType: opType,
		Note:          note,
		CreatedTime:   utils.CustomTime{Time: time.Now()},
	}
	return tx.Create(&logEntry).Error
}

// ==================== Featured Communities ====================

// GetFeaturedCommunities returns featured communities for a language segment
func (s *CommunityService) GetFeaturedCommunities(segment string) ([]community.Community, error) {
	var featured []community.FeaturedCommunity
	if err := s.db.Where("segment = ?", segment).
		Order("position ASC").
		Find(&featured).Error; err != nil {
		return nil, err
	}

	if len(featured) == 0 {
		return []community.Community{}, nil
	}

	// Collect NDC IDs
	var ndcIds []int
	for _, f := range featured {
		ndcIds = append(ndcIds, f.NdcID)
	}

	// Get communities by IDs
	var communities []community.Community
	if err := s.db.Where("ndc_id IN ?", ndcIds).Find(&communities).Error; err != nil {
		return nil, err
	}

	// Create map for ordering
	comMap := make(map[int]community.Community)
	for _, c := range communities {
		comMap[c.NdcId] = c
	}

	// Return in featured order
	result := make([]community.Community, 0, len(featured))
	for _, f := range featured {
		if c, ok := comMap[f.NdcID]; ok {
			result = append(result, c)
		}
	}

	return result, nil
}

// GetCommunitiesByIds returns communities by their NDC IDs (preserving order)
func (s *CommunityService) GetCommunitiesByIds(ndcIds []int) ([]community.Community, error) {
	if len(ndcIds) == 0 {
		return []community.Community{}, nil
	}

	var communities []community.Community
	if err := s.db.Where("ndc_id IN ?", ndcIds).Find(&communities).Error; err != nil {
		return nil, err
	}

	// Create map for ordering
	comMap := make(map[int]community.Community)
	for _, c := range communities {
		comMap[c.NdcId] = c
	}

	// Return in requested order
	result := make([]community.Community, 0, len(ndcIds))
	for _, id := range ndcIds {
		if c, ok := comMap[id]; ok {
			result = append(result, c)
		}
	}

	return result, nil
}

// SetFeaturedCommunities replaces the featured communities for a segment
func (s *CommunityService) SetFeaturedCommunities(segment string, ndcIds []int, addedBy string) error {
	// Verify all communities exist
	var count int64
	s.db.Model(&community.Community{}).Where("ndc_id IN ?", ndcIds).Count(&count)
	if int(count) != len(ndcIds) {
		return ErrCommunityNotFound
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete existing featured for this segment
		if err := tx.Where("segment = ?", segment).Delete(&community.FeaturedCommunity{}).Error; err != nil {
			return err
		}

		// Insert new featured communities
		for i, ndcId := range ndcIds {
			featured := community.FeaturedCommunity{
				NdcID:    ndcId,
				Segment:  segment,
				Position: i,
				AddedBy:  addedBy,
			}
			if err := tx.Create(&featured).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// AddFeaturedCommunity adds a single community to the featured list
func (s *CommunityService) AddFeaturedCommunity(segment string, ndcId int, addedBy string) error {
	// Verify community exists
	var count int64
	s.db.Model(&community.Community{}).Where("ndc_id = ?", ndcId).Count(&count)
	if count == 0 {
		return ErrCommunityNotFound
	}

	// Check if already featured
	s.db.Model(&community.FeaturedCommunity{}).Where("segment = ? AND ndc_id = ?", segment, ndcId).Count(&count)
	if count > 0 {
		return ErrAlreadyFeatured
	}

	// Get max position
	var maxPos int
	s.db.Model(&community.FeaturedCommunity{}).
		Where("segment = ?", segment).
		Select("COALESCE(MAX(position), -1)").
		Scan(&maxPos)

	featured := community.FeaturedCommunity{
		NdcID:    ndcId,
		Segment:  segment,
		Position: maxPos + 1,
		AddedBy:  addedBy,
	}

	return s.db.Create(&featured).Error
}

// RemoveFeaturedCommunity removes a community from the featured list
func (s *CommunityService) RemoveFeaturedCommunity(segment string, ndcId int) error {
	return s.db.Where("segment = ? AND ndc_id = ?", segment, ndcId).
		Delete(&community.FeaturedCommunity{}).Error
}

// ==================== Banners ====================

// SetBanners replaces all banners for a segment atomically
func (s *CommunityService) SetBanners(segment string, banners []struct {
	ImageURL string
	LinkURL  string
}, addedBy string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete existing banners for this segment
		if err := tx.Where("segment = ?", segment).Delete(&community.Banner{}).Error; err != nil {
			return err
		}

		// Insert new banners
		for i, b := range banners {
			banner := community.Banner{
				Segment:  segment,
				ImageURL: b.ImageURL,
				LinkURL:  b.LinkURL,
				Position: i,
				AddedBy:  addedBy,
			}
			if err := tx.Create(&banner).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// GetBanners returns banners for a segment ordered by position
func (s *CommunityService) GetBanners(segment string) ([]community.Banner, error) {
	var banners []community.Banner
	if err := s.db.Where("segment = ?", segment).
		Order("position ASC").
		Find(&banners).Error; err != nil {
		return nil, err
	}
	return banners, nil
}

// GetCommunityMembers returns community members filtered by type
func (s *CommunityService) GetCommunityMembers(ndcId int, memberType string, start, size int) ([]user.User, error) {
	var members []user.User

	query := s.db.Where("ndc_id = ? AND role >= 0", ndcId)

	switch memberType {
	case "leaders":
		query = query.Where("role >= ?", user.RoleLeader)
	case "curators":
		query = query.Where("role = ?", user.RoleCurator)
	case "members":
		query = query.Where("role = ?", user.RoleMember)
	}

	err := query.
		Order("role DESC, created_time ASC").
		Preload("AvatarFrame").
		Preload("CustomTitles").
		Offset(start).Limit(size).
		Find(&members).Error

	if err != nil {
		return nil, err
	}

	return members, nil
}

// ==================== Content Moderation ====================

// HideBlog hides a blog post (Curator+ required)
func (s *CommunityService) HideBlog(ndcId int, requesterUID, blogId, reason string) error {
	permSvc := NewPermissionService(s.db)

	// Hide requires Curator (50) or higher
	if !permSvc.CanPerform(requesterUID, "", ndcId, "", user.RoleCurator, false) {
		return ErrPermissionDenied
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Get blog to log the action
		var blogUID string
		if err := tx.Table("blogs").Select("uid").Where("blog_id = ? AND ndc_id = ?", blogId, ndcId).Scan(&blogUID).Error; err != nil {
			return fmt.Errorf("blog not found: %w", err)
		}

		// Set status = 1 (hidden)
		if err := tx.Table("blogs").Where("blog_id = ? AND ndc_id = ?", blogId, ndcId).
			Update("status", 1).Error; err != nil {
			return err
		}

		// Log action
		return s.logAction(tx, ndcId, requesterUID, blogUID, blogId, community.ObjectTypeBlog, community.OpTypeHide, reason)
	})
}

// UnhideBlog unhides a blog post (Curator+ required)
func (s *CommunityService) UnhideBlog(ndcId int, requesterUID, blogId, reason string) error {
	permSvc := NewPermissionService(s.db)

	if !permSvc.CanPerform(requesterUID, "", ndcId, "", user.RoleCurator, false) {
		return ErrPermissionDenied
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var blogUID string
		if err := tx.Table("blogs").Select("uid").Where("blog_id = ? AND ndc_id = ?", blogId, ndcId).Scan(&blogUID).Error; err != nil {
			return fmt.Errorf("blog not found: %w", err)
		}

		// Set status = 0 (visible)
		if err := tx.Table("blogs").Where("blog_id = ? AND ndc_id = ?", blogId, ndcId).
			Update("status", 0).Error; err != nil {
			return err
		}

		return s.logAction(tx, ndcId, requesterUID, blogUID, blogId, community.ObjectTypeBlog, community.OpTypeUnhide, reason)
	})
}

// HideThread hides a chat thread (Curator+ required)
func (s *CommunityService) HideThread(ndcId int, requesterUID, threadId, reason string) error {
	permSvc := NewPermissionService(s.db)

	if !permSvc.CanPerform(requesterUID, "", ndcId, "", user.RoleCurator, false) {
		return ErrPermissionDenied
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var threadUID string
		if err := tx.Table("threads").Select("uid").Where("thread_id = ? AND ndc_id = ?", threadId, ndcId).Scan(&threadUID).Error; err != nil {
			return fmt.Errorf("thread not found: %w", err)
		}

		// Set status = 1 (hidden)
		if err := tx.Table("threads").Where("thread_id = ? AND ndc_id = ?", threadId, ndcId).
			Update("status", 1).Error; err != nil {
			return err
		}

		return s.logAction(tx, ndcId, requesterUID, threadUID, threadId, community.ObjectTypeThread, community.OpTypeHide, reason)
	})
}

// UnhideThread unhides a chat thread (Curator+ required)
func (s *CommunityService) UnhideThread(ndcId int, requesterUID, threadId, reason string) error {
	permSvc := NewPermissionService(s.db)

	if !permSvc.CanPerform(requesterUID, "", ndcId, "", user.RoleCurator, false) {
		return ErrPermissionDenied
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var threadUID string
		if err := tx.Table("threads").Select("uid").Where("thread_id = ? AND ndc_id = ?", threadId, ndcId).Scan(&threadUID).Error; err != nil {
			return fmt.Errorf("thread not found: %w", err)
		}

		// Set status = 0 (visible)
		if err := tx.Table("threads").Where("thread_id = ? AND ndc_id = ?", threadId, ndcId).
			Update("status", 0).Error; err != nil {
			return err
		}

		return s.logAction(tx, ndcId, requesterUID, threadUID, threadId, community.ObjectTypeThread, community.OpTypeUnhide, reason)
	})
}
