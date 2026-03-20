package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Определяем кастомные ошибки для бизнес-логики
var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidUserData    = errors.New("invalid user data")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailAlreadyExists = errors.New("email already exists")
)

// UserService инкапсулирует бизнес-логику работы с пользователями
type UserService struct {
	db *gorm.DB
}

// NewUserService создаёт новый экземпляр UserService
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// CreateUser создаёт нового пользователя
// Проверяет уникальность UID и AminoID перед созданием
func (s *UserService) CreateUser(u *user.User) error {
	// Проверка обязательных полей
	if u.UID == "" || u.Nickname == "" {
		return fmt.Errorf("%w: UID and Nickname are required", ErrInvalidUserData)
	}

	// Проверка уникальности
	var count int64
	s.db.Model(&user.User{}).Where("uid = ? OR amino_id = ?", u.UID, u.AminoID).Count(&count)
	if count > 0 {
		return ErrUserAlreadyExists
	}

	return s.db.Create(u).Error
}

// GetUserByID получает пользователя по UID и NDC ID
// Опционально загружает связанные данные (AvatarFrame, CustomTitles)
func (s *UserService) GetUserByID(uid string, ndcId int, withRelations bool) (*user.User, error) {
	var u user.User

	query := s.db.Model(&user.User{})

	// Если нужны связанные данные - подгружаем их
	if withRelations {
		// Joins для has-one, Preload для has-many - хороший компромисс
		query = query.Joins("AvatarFrame").Preload("CustomTitles")
	}

	// Указываем таблицу 'users' явно, чтобы избежать неоднозначности после JOIN
	// Ищем по UID и NDC ID
	err := query.First(&u, "users.uid = ? AND users.ndc_id = ?", uid, ndcId).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &u, nil
}

// UpdateUser обновляет данные пользователя
// Принимает map с полями для обновления
// Возвращает обновлённого пользователя
func (s *UserService) UpdateUser(uid string, ndcId int, updates map[string]interface{}) (*user.User, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("%w: no fields to update", ErrInvalidUserData)
	}

	// Проверяем существование пользователя
	u, err := s.GetUserByID(uid, ndcId, false)
	if err != nil {
		return nil, err
	}

	// Обновляем
	err = s.db.Model(u).Updates(updates).Error
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Возвращаем обновлённые данные
	return s.GetUserByID(uid, ndcId, true)
}

// DeleteUser мягко удаляет пользователя (soft delete)
func (s *UserService) DeleteUser(uid string) error {
	result := s.db.Where("uid = ?", uid).Delete(&user.User{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// ListUsers возвращает список пользователей с пагинацией
func (s *UserService) ListUsers(page, pageSize int) ([]user.User, int64, error) {
	var users []user.User
	var total int64

	// Подсчёт total
	if err := s.db.Model(&user.User{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Получение страницы
	offset := (page - 1) * pageSize
	if err := s.db.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	return users, total, nil
}

// GetUserAuthByEmail получает данные аутентификации по email
func (s *UserService) GetUserAuthByEmail(email string) (*user.UserAuth, error) {
	var userAuth user.UserAuth

	err := s.db.Where("email = ?", email).First(&userAuth).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user auth: %w", err)
	}

	return &userAuth, nil
}

// GetUserAuthByUID returns user auth data by user UID
func (s *UserService) GetUserAuthByUID(uid string) (*user.UserAuth, error) {
	var userAuth user.UserAuth

	err := s.db.Where("user_id = ?", uid).First(&userAuth).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user auth: %w", err)
	}

	return &userAuth, nil
}

// UpdateContentRegion updates the content region preference for a user
func (s *UserService) UpdateContentRegion(uid, region string) error {
	result := s.db.Model(&user.UserAuth{}).Where("user_id = ?", uid).Update("content_region", region)
	if result.Error != nil {
		return fmt.Errorf("failed to update content region: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// VerifyPassword проверяет соответствие пароля хешу
func (s *UserService) VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// AuthenticateUser проверяет email и пароль, возвращает пользователя
func (s *UserService) AuthenticateUser(email, password string) (*user.User, error) {
	// Получаем данные аутентификации
	userAuth, err := s.GetUserAuthByEmail(email)
	if err != nil {
		// Если юзер не найден - возвращаем ошибку "юзер не найден"
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Проверяем пароль
	if err := s.VerifyPassword(userAuth.PasswordHash, password); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Получаем полные данные пользователя (Global Profile)
	u, err := s.GetUserByID(userAuth.UserID, 0, true)
	if err != nil {
		return nil, err
	}

	return u, nil
}

// CreateUserWithAuth создаёт нового пользователя с паролем
// Проверяет уникальность email, создаёт User и UserAuth в одной транзакции
func (s *UserService) CreateUserWithAuth(email, password, nickname string) (*user.User, error) {
	// Проверка email на уникальность
	var count int64
	s.db.Model(&user.UserAuth{}).Where("email = ?", email).Count(&count)
	if count > 0 {
		return nil, ErrEmailAlreadyExists
	}

	// Генерируем UID и AminoID
	uid := generateUID()
	aminoID := generateAminoID(nickname)

	// Проверяем уникальность AminoID
	s.db.Model(&user.User{}).Where("amino_id = ?", aminoID).Count(&count)
	if count > 0 {
		// Если занят, добавляем случайные цифры
		aminoID = fmt.Sprintf("%s%d", aminoID, time.Now().Unix()%10000)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	var u user.User
	err = s.db.Transaction(func(tx *gorm.DB) error {
		now := utils.CustomTime{Time: time.Now()}

		u = user.User{
			UID:                uid,
			Nickname:           nickname,
			AminoID:            aminoID,
			Status:             0,
			Level:              1,
			Role:               0,
			Reputation:         0,
			IsGlobal:           true,
			Icon:               "",
			CreatedTime:        now,
			ModifiedTime:       now,
			OnlineStatus:       1,
			MembershipStatus:   0,
			FollowingStatus:    0,
			IsNicknameVerified: false,
			PushEnabled:        true,
		}

		if err := tx.Create(&u).Error; err != nil {
			return err
		}

		// Создаём UserAuth
		userAuth := user.UserAuth{
			UserID:       uid,
			Email:        email,
			PasswordHash: string(hashedPassword),
		}

		if err := tx.Create(&userAuth).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &u, nil
}

func generateAminoID(nickname string) string {
	aminoID := strings.ToLower(strings.ReplaceAll(nickname, " ", ""))
	var result strings.Builder
	for _, r := range aminoID {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		}
	}

	randomSuffix := time.Now().UnixNano() % 10000
	result.WriteString(fmt.Sprintf("%04d", randomSuffix))
	return result.String()
}

// FollowUser подписывает followerUID на targetUID в контексте ndcId
func (s *UserService) FollowUser(followerUID, targetUID string, ndcId int) error {
	if followerUID == targetUID {
		return fmt.Errorf("cannot follow yourself")
	}

	// Проверяем существование целевого пользователя
	var count int64
	s.db.Model(&user.User{}).Where("uid = ?", targetUID).Count(&count)
	if count == 0 {
		return ErrUserNotFound
	}

	// Проверяем, не подписан ли уже в этом контексте
	var existing user.UserFollow
	err := s.db.Where("follower_uid = ? AND target_uid = ? AND ndc_id = ?", followerUID, targetUID, ndcId).First(&existing).Error
	if err == nil {
		return nil // Уже подписан
	}

	// Создаем подписку
	follow := user.UserFollow{
		FollowerUID: followerUID,
		TargetUID:   targetUID,
		NdcID:       ndcId,
	}

	if err := s.db.Create(&follow).Error; err != nil {
		return err
	}

	// Обновляем membersCount у целевого пользователя и joinedCount у подписчика
	s.db.Model(&user.User{}).Where("uid = ? AND ndc_id = ?", targetUID, ndcId).UpdateColumn("members_count", gorm.Expr("members_count + 1"))
	s.db.Model(&user.User{}).Where("uid = ? AND ndc_id = ?", followerUID, ndcId).UpdateColumn("joined_count", gorm.Expr("joined_count + 1"))

	return nil
}

// UnfollowUser отписывает followerUID от targetUID в контексте ndcId
func (s *UserService) UnfollowUser(followerUID, targetUID string, ndcId int) error {
	result := s.db.Where("follower_uid = ? AND target_uid = ? AND ndc_id = ?", followerUID, targetUID, ndcId).Delete(&user.UserFollow{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		s.db.Model(&user.User{}).Where("uid = ? AND ndc_id = ? AND members_count > 0", targetUID, ndcId).UpdateColumn("members_count", gorm.Expr("members_count - 1"))
		s.db.Model(&user.User{}).Where("uid = ? AND ndc_id = ? AND joined_count > 0", followerUID, ndcId).UpdateColumn("joined_count", gorm.Expr("joined_count - 1"))
	}
	return nil
}

// IsFollowing проверяет, подписан ли followerUID на targetUID в контексте ndcId
func (s *UserService) IsFollowing(followerUID, targetUID string, ndcId int) bool {
	var count int64
	s.db.Model(&user.UserFollow{}).Where("follower_uid = ? AND target_uid = ? AND ndc_id = ?", followerUID, targetUID, ndcId).Count(&count)
	return count > 0
}

// GetFollowers возвращает список подписчиков пользователя в контексте ndcId
func (s *UserService) GetFollowers(uid string, ndcId int, start, size int) ([]user.DetailedAuthor, error) {
	var follows []user.UserFollow
	var followers []user.DetailedAuthor

	// Получаем UID подписчиков в конкретном сообществе
	err := s.db.Where("target_uid = ? AND ndc_id = ?", uid, ndcId).
		Order("created_at DESC").
		Offset(start).
		Limit(size).
		Find(&follows).Error
	if err != nil {
		return nil, err
	}

	if len(follows) == 0 {
		return []user.DetailedAuthor{}, nil
	}

	followerUIDs := make([]string, len(follows))
	for i, f := range follows {
		followerUIDs[i] = f.FollowerUID
	}

	// Загружаем профили пользователей для нужного сообщества
	var users []user.User
	if err := s.db.Where("uid IN ? AND ndc_id = ?", followerUIDs, ndcId).Find(&users).Error; err != nil {
		return nil, err
	}

	userMap := make(map[string]user.User)
	for _, u := range users {
		userMap[u.UID] = u
	}

	for _, uid := range followerUIDs {
		if u, ok := userMap[uid]; ok {
			author := user.DetailedAuthor{
				Author: user.Author{
					UID:        u.UID,
					Nickname:   u.Nickname,
					Icon:       u.Icon,
					Level:      u.Level,
					Reputation: u.Reputation,
					Role:       u.Role,
					Status:     u.Status,
				},
				AminoID:  u.AminoID,
				IsGlobal: u.IsGlobal,
			}
			followers = append(followers, author)
		}
	}

	return followers, nil
}

// GetFollowing возвращает список пользователей, на которых подписан uid в контексте ndcId
func (s *UserService) GetFollowing(uid string, ndcId int, start, size int) ([]user.DetailedAuthor, error) {
	var follows []user.UserFollow
	var following []user.DetailedAuthor

	// Получаем UID тех, на кого подписан пользователь в конкретном сообществе
	err := s.db.Where("follower_uid = ? AND ndc_id = ?", uid, ndcId).
		Order("created_at DESC").
		Offset(start).
		Limit(size).
		Find(&follows).Error
	if err != nil {
		return nil, err
	}

	if len(follows) == 0 {
		return []user.DetailedAuthor{}, nil
	}

	targetUIDs := make([]string, len(follows))
	for i, f := range follows {
		targetUIDs[i] = f.TargetUID
	}

	// Загружаем профили пользователей для нужного сообщества
	var users []user.User
	if err := s.db.Where("uid IN ? AND ndc_id = ?", targetUIDs, ndcId).Find(&users).Error; err != nil {
		return nil, err
	}

	userMap := make(map[string]user.User)
	for _, u := range users {
		userMap[u.UID] = u
	}

	for _, uid := range targetUIDs {
		if u, ok := userMap[uid]; ok {
			author := user.DetailedAuthor{
				Author: user.Author{
					UID:        u.UID,
					Nickname:   u.Nickname,
					Icon:       u.Icon,
					Level:      u.Level,
					Reputation: u.Reputation,
					Role:       u.Role,
					Status:     u.Status,
				},
				AminoID:  u.AminoID,
				IsGlobal: u.IsGlobal,
				FollowingStatus: 1,
			}
			following = append(following, author)
		}
	}

	return following, nil
}

// AddWallComment добавляет комментарий на стену пользователя
func (s *UserService) AddWallComment(authorUID, targetUID, content, replyTo string, ndcId int) (*user.Comment, error) {
	// Проверка существования получателя
	var target user.User
	if err := s.db.First(&target, "uid = ?", targetUID).Error; err != nil {
		return nil, ErrUserNotFound
	}

	comment := &user.Comment{
		ID:         generateUID(),
		ParentID:   targetUID,
		ParentType: 0, // 0 - User Profile
		NdcID:      ndcId,
		Content:    content,
		AuthorUID:  authorUID,
	}

	if replyTo != "" {
		// Проверяем существование родительского комментария
		var parent user.Comment
		if err := s.db.First(&parent, "id = ?", replyTo).Error; err != nil {
			return nil, fmt.Errorf("parent comment not found")
		}

		// Логика вложенности: в Amino обычно 1 уровень вложенности (или плоская структура с реплаем)
		// Если parent уже имеет RootCommentID, наследуем его, иначе parent сам становится Root
		if parent.RootCommentID != nil {
			comment.RootCommentID = parent.RootCommentID
		} else {
			comment.RootCommentID = &replyTo
		}

		// Увеличиваем счетчик саб-комментариев у родителя/корня (опционально)
		// s.db.Model(&user.Comment{}).Where("id = ?", *comment.RootCommentID).Update("subcomments_count", gorm.Expr("subcomments_count + 1"))
	}

	if err := s.db.Create(comment).Error; err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	// Обновляем commentsCount у автора комментария
	s.db.Model(&user.User{}).Where("uid = ? AND ndc_id = ?", authorUID, ndcId).UpdateColumn("comments_count", gorm.Expr("comments_count + 1"))

	// Загружаем автора для ответа
	s.db.Preload("Author").First(comment, "id = ?", comment.ID)

	return comment, nil
}

// GetWallComments получает комментарии со стены пользователя
func (s *UserService) GetWallComments(targetUID string, ndcId int, sort string, start, size int) ([]user.Comment, error) {
	var comments []user.Comment

	query := s.db.Preload("Author").
		Where("parent_id = ? AND parent_type = ? AND ndc_id = ?", targetUID, 0, ndcId)

	if sort == "newest" {
		query = query.Order("created_time DESC")
	} else {
		query = query.Order("like_count DESC") // vote?
	}

	err := query.Offset(start).Limit(size).Find(&comments).Error
	return comments, err
}

// DeleteWallComment удаляет комментарий со стены
// Удалить может: Автор, Владелец стены, Куратор+ (в контексте сообщества), Астранет.
func (s *UserService) DeleteWallComment(commentId, requestorId string) error {
	var comment user.Comment
	if err := s.db.First(&comment, "id = ?", commentId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("comment not found")
		}
		return err
	}

	// Получаем NdcID контекста (по владельцу стены)
	var wallOwner user.User
	if err := s.db.Select("ndc_id").First(&wallOwner, "uid = ?", comment.ParentID).Error; err != nil {
		// Если владельца нет, считаем глобальным (0) или ошибкой. Пусть будет 0.
	}
	ndcId := wallOwner.NdcID

	permSvc := NewPermissionService(s.db)
	role := permSvc.GetEffectiveRole(requestorId, ndcId, "")

	if role == user.RoleBanned {
		return errors.New("permission denied: account banned")
	}

	// 1. Staff Check (Curator+)
	if role >= user.RoleCurator {
		return s.db.Delete(&comment).Error
	}

	// 2. Author Check
	if comment.AuthorUID == requestorId {
		return s.db.Delete(&comment).Error
	}

	// 3. Wall Owner Check
	if comment.ParentID == requestorId {
		return s.db.Delete(&comment).Error
	}

	return errors.New("permission denied")
}

// ==================== Global Ban (Astranet only) ====================

// GlobalBanUser globally bans a user (only Astranet team can do this)
// This removes the user from all communities and prevents any actions
func (s *UserService) GlobalBanUser(requesterUID, targetUID, reason string) error {
	permSvc := NewPermissionService(s.db)

	// Only Astranet (role 1000) can global ban
	requesterRole := permSvc.GetEffectiveRole(requesterUID, 0, "")
	if requesterRole != user.RoleAstranet {
		return ErrPermissionDenied
	}

	// Check target is not Astranet
	targetRole := permSvc.GetEffectiveRole(targetUID, 0, "")
	if targetRole >= user.RoleAstranet {
		return errors.New("cannot ban Astranet team members")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Set role = -1 for global profile
		if err := tx.Model(&user.User{}).
			Where("uid = ? AND ndc_id = 0", targetUID).
			Update("role", user.RoleBanned).Error; err != nil {
			return err
		}

		// 2. Create global ban record
		ban := &user.GlobalBan{
			UID:         targetUID,
			BannedByUID: requesterUID,
			Reason:      reason,
			IsActive:    true,
		}
		if err := tx.Create(ban).Error; err != nil {
			return err
		}

		// 3. Remove user from all communities (delete community profiles)
		if err := tx.Where("uid = ? AND ndc_id != 0", targetUID).
			Delete(&user.User{}).Error; err != nil {
			return err
		}

		return nil
	})
}

// GlobalUnbanUser removes global ban (only Astranet team can do this)
func (s *UserService) GlobalUnbanUser(requesterUID, targetUID string) error {
	permSvc := NewPermissionService(s.db)

	// Only Astranet can unban
	requesterRole := permSvc.GetEffectiveRole(requesterUID, 0, "")
	if requesterRole != user.RoleAstranet {
		return ErrPermissionDenied
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Deactivate global ban
		if err := tx.Model(&user.GlobalBan{}).
			Where("uid = ? AND is_active = true", targetUID).
			Update("is_active", false).Error; err != nil {
			return err
		}

		// 2. Restore role to Member (0) for global profile
		if err := tx.Model(&user.User{}).
			Where("uid = ? AND ndc_id = 0", targetUID).
			Update("role", user.RoleMember).Error; err != nil {
			return err
		}

		return nil
	})
}

// IsGloballyBanned checks if a user is globally banned
func (s *UserService) IsGloballyBanned(uid string) bool {
	var ban user.GlobalBan
	err := s.db.Where("uid = ? AND is_active = true", uid).First(&ban).Error
	return err == nil
}

// ==================== User Blocking ====================

// BlockUser blocks targetUID from the perspective of blockerUID.
// blockerUID is the user performing the block.
func (s *UserService) BlockUser(blockerUID, targetUID string) error {
	if blockerUID == targetUID {
		return errors.New("cannot block yourself")
	}

	// Check if already blocked
	var existing user.UserBlock
	err := s.db.Where("uid = ? AND blocked_uid = ?", blockerUID, targetUID).First(&existing).Error
	if err == nil {
		return errors.New("user already blocked")
	}

	block := &user.UserBlock{
		UID:        blockerUID,
		BlockedUID: targetUID,
	}
	return s.db.Create(block).Error
}

// UnblockUser removes a block
func (s *UserService) UnblockUser(blockerUID, targetUID string) error {
	result := s.db.Where("uid = ? AND blocked_uid = ?", blockerUID, targetUID).Delete(&user.UserBlock{})
	if result.RowsAffected == 0 {
		return errors.New("user is not blocked")
	}
	return result.Error
}

// IsBlocked checks if blockerUID has blocked targetUID
func (s *UserService) IsBlocked(blockerUID, targetUID string) bool {
	var block user.UserBlock
	err := s.db.Where("uid = ? AND blocked_uid = ?", blockerUID, targetUID).First(&block).Error
	return err == nil
}

// IsBlockedEither checks if either user has blocked the other
func (s *UserService) IsBlockedEither(uid1, uid2 string) bool {
	var count int64
	s.db.Model(&user.UserBlock{}).
		Where("(uid = ? AND blocked_uid = ?) OR (uid = ? AND blocked_uid = ?)", uid1, uid2, uid2, uid1).
		Count(&count)
	return count > 0
}

// GetBlockedUsers returns the list of users blocked by blockerUID
func (s *UserService) GetBlockedUsers(blockerUID string, start, size int) ([]user.UserBlock, int64, error) {
	var blocks []user.UserBlock
	var total int64

	s.db.Model(&user.UserBlock{}).Where("uid = ?", blockerUID).Count(&total)

	err := s.db.Where("uid = ?", blockerUID).
		Preload("BlockedUser").
		Order("created_at DESC").
		Offset(start).Limit(size).
		Find(&blocks).Error

	return blocks, total, err
}
