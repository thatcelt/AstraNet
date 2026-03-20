package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/AugustLigh/GoMino/internal/models/blog"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

type BlogService struct {
	db *gorm.DB
}

func NewBlogService(db *gorm.DB) *BlogService {
	return &BlogService{db: db}
}

func (s *BlogService) CreateBlog(uid string, ndcId int, title, content string, mediaList []utils.MediaItem, backgroundMediaList []utils.MediaItem) (*blog.Blog, error) {
	permSvc := NewPermissionService(s.db)

	// Permission check for global blogs
	if ndcId == 0 {
		if !isTeamAstranet(s.db, uid) {
			return nil, fmt.Errorf("permission denied: only Astranet team can create global blogs")
		}
	} else {
		// Check if user is banned in the community
		role := permSvc.GetEffectiveRole(uid, ndcId, "")
		if role == user.RoleBanned {
			return nil, fmt.Errorf("permission denied: user is banned")
		}
	}

	var mList *[]utils.MediaItem
	if len(mediaList) > 0 {
		mList = &mediaList
	}

	var extensions *blog.BlogExtensions
	if len(backgroundMediaList) > 0 {
		extensions = &blog.BlogExtensions{
			BackgroundMediaList: &backgroundMediaList,
		}
	}

	newBlog := &blog.Blog{
		BlogID:     generateUID(),
		Title:      title,
		Content:    content,
		MediaList:  mList,
		Extensions: extensions,
		UID:        uid,
		NdcID:      ndcId,
	}

	if err := s.db.Create(newBlog).Error; err != nil {
		return nil, fmt.Errorf("failed to create blog: %w", err)
	}

	// Load Author
	if err := s.db.Preload("Author").First(newBlog, newBlog.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to load author: %w", err)
	}

	return newBlog, nil
}

func (s *BlogService) GetBlog(blogID string) (*blog.Blog, error) {
	var b blog.Blog
	if err := s.db.Preload("Author").First(&b, "blog_id = ?", blogID).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *BlogService) GetBlogWithVote(blogID string, uid string) (*blog.Blog, error) {
	var b blog.Blog
	if err := s.db.Preload("Author").First(&b, "blog_id = ?", blogID).Error; err != nil {
		return nil, err
	}

	// Получить голос пользователя
	if uid != "" {
		voteValue, err := s.GetUserVote(blogID, uid)
		if err == nil {
			b.VotedValue = &voteValue
		}
	}

	return &b, nil
}

func (s *BlogService) GetCommunityBlogs(ndcId int, start, size int, requestorUID string) ([]blog.Blog, error) {
	var blogs []blog.Blog

	permSvc := NewPermissionService(s.db)
	role := permSvc.GetEffectiveRole(requestorUID, ndcId, "")

	query := s.db.Preload("Author").Where("blogs.ndc_id = ?", ndcId)

	if role >= user.RoleCurator {
		// Moderators see all posts (including hidden)
		query = query.Where("blogs.status >= 0") // Exclude deleted (-1)
	} else {
		// Regular users see only active posts (status = 0)
		// and don't see posts from banned users
		query = query.Where("blogs.status = 0").
			Joins("LEFT JOIN users ON blogs.uid = users.uid AND users.ndc_id = ?", ndcId).
			Where("(users.role >= 0 OR users.role IS NULL OR blogs.uid = ?)", requestorUID)
	}

	err := query.
		Order("blogs.created_time DESC").
		Limit(size).
		Offset(start).
		Find(&blogs).Error

	if err != nil {
		return nil, err
	}

	// Заполнить VotedValue для каждого блога
	if requestorUID != "" {
		s.fillVotedValues(blogs, requestorUID)
	}

	return blogs, nil
}

func (s *BlogService) GetUserBlogs(uid string, ndcId int, start, size int, requestorUID string) ([]blog.Blog, error) {
	var blogs []blog.Blog
	query := s.db.Preload("Author").Where("uid = ? AND status = 0", uid)

	if ndcId != 0 {
		query = query.Where("ndc_id = ?", ndcId)
	}

	err := query.
		Order("created_time DESC").
		Limit(size).
		Offset(start).
		Find(&blogs).Error

	if err != nil {
		return nil, err
	}

	// Заполнить VotedValue для каждого блога
	if requestorUID != "" {
		s.fillVotedValues(blogs, requestorUID)
	}

	return blogs, nil
}

func (s *BlogService) DeleteBlog(blogID string, uid string) error {
	var b blog.Blog
	if err := s.db.First(&b, "blog_id = ?", blogID).Error; err != nil {
		return fmt.Errorf("blog not found")
	}

	permSvc := NewPermissionService(s.db)
	// Require Curator (50) to delete others' posts, or be the author (allowSelf=true)
	if !permSvc.CanPerform(uid, b.UID, b.NdcID, "", user.RoleCurator, true) {
		return fmt.Errorf("permission denied")
	}

	return s.db.Delete(&b).Error
}

func (s *BlogService) UpdateBlog(blogID string, uid string, updates map[string]interface{}) (*blog.Blog, error) {
	var b blog.Blog
	if err := s.db.First(&b, "blog_id = ?", blogID).Error; err != nil {
		return nil, fmt.Errorf("blog not found")
	}

	if b.UID != uid {
		return nil, fmt.Errorf("only author can update blog")
	}

	updates["modified_time"] = utils.CustomTime{Time: time.Now()}

	if err := s.db.Model(&b).Updates(updates).Error; err != nil {
		return nil, err
	}

	return s.GetBlog(blogID)
}

// FeatureBlog - добавить пост в подборку
func (s *BlogService) FeatureBlog(blogID string, ndcId int, uid string, days int) error {
	// Проверка прав: требуется минимум Куратор (50)
	permSvc := NewPermissionService(s.db)
	if !permSvc.CanPerform(uid, "", ndcId, "", user.RoleCurator, false) {
		return fmt.Errorf("permission denied")
	}

	// Проверить существование блога
	var b blog.Blog
	if err := s.db.First(&b, "blog_id = ? AND ndc_id = ?", blogID, ndcId).Error; err != nil {
		return fmt.Errorf("blog not found in this community")
	}

	// Проверить валидность срока (1, 2, 3, 4 дня)
	if days != 1 && days != 2 && days != 3 && days != 4 {
		return fmt.Errorf("invalid duration: must be 1, 2, 3, or 4 days")
	}

	// Проверить, не в подборке ли уже
	var existing blog.FeaturedPost
	if err := s.db.Where("blog_id = ? AND ndc_id = ?", blogID, ndcId).First(&existing).Error; err == nil {
		return fmt.Errorf("blog is already featured")
	}

	// Создать запись в подборке
	featuredUntil := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	featured := &blog.FeaturedPost{
		BlogID:        blogID,
		NdcID:         ndcId,
		FeaturedByUID: uid,
		FeaturedUntil: utils.CustomTime{Time: featuredUntil},
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(featured).Error; err != nil {
			return fmt.Errorf("failed to feature blog: %w", err)
		}

		// Логировать действие
		return s.logFeaturedAction(tx, ndcId, uid, b.UID, blogID, true, fmt.Sprintf("Featured for %d days", days))
	})
}

// UnfeatureBlog - удалить пост из подборки
func (s *BlogService) UnfeatureBlog(blogID string, ndcId int, uid string) error {
	// Проверка прав: требуется минимум Куратор (50)
	permSvc := NewPermissionService(s.db)
	if !permSvc.CanPerform(uid, "", ndcId, "", user.RoleCurator, false) {
		return fmt.Errorf("permission denied")
	}

	var featured blog.FeaturedPost
	if err := s.db.Where("blog_id = ? AND ndc_id = ?", blogID, ndcId).First(&featured).Error; err != nil {
		return fmt.Errorf("blog is not featured")
	}

	// Получить информацию о блоге для логирования
	var b blog.Blog
	if err := s.db.First(&b, "blog_id = ?", blogID).Error; err != nil {
		return fmt.Errorf("blog not found")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&featured).Error; err != nil {
			return fmt.Errorf("failed to unfeature blog: %w", err)
		}

		// Логировать действие
		return s.logFeaturedAction(tx, ndcId, uid, b.UID, blogID, false, "Removed from featured")
	})
}

// GetFeaturedBlogs - получить список постов из подборки
func (s *BlogService) GetFeaturedBlogs(ndcId int, start, size int, requestorUID string) ([]blog.Blog, error) {
	var blogs []blog.Blog

	// Сначала удалить просроченные посты
	s.RemoveExpiredFeaturedBlogs(ndcId)

	// Получить список featured blog_id
	var featuredIDs []string
	if err := s.db.Model(&blog.FeaturedPost{}).
		Where("ndc_id = ? AND featured_until > ?", ndcId, utils.CustomTime{Time: time.Now()}).
		Order("created_time DESC").
		Pluck("blog_id", &featuredIDs).Error; err != nil {
		return nil, err
	}

	if len(featuredIDs) == 0 {
		return []blog.Blog{}, nil
	}

	permSvc := NewPermissionService(s.db)
	role := permSvc.GetEffectiveRole(requestorUID, ndcId, "")

	query := s.db.Preload("Author").Where("blogs.blog_id IN ?", featuredIDs)

	if role >= user.RoleCurator {
		// Moderators see all featured posts
		query = query.Where("blogs.status >= 0")
	} else {
		// Regular users see only active featured posts from non-banned users
		query = query.Where("blogs.status = 0").
			Joins("LEFT JOIN users ON blogs.uid = users.uid AND users.ndc_id = ?", ndcId).
			Where("(users.role >= 0 OR users.role IS NULL OR blogs.uid = ?)", requestorUID)
	}

	err := query.
		Limit(size).
		Offset(start).
		Find(&blogs).Error

	if err != nil {
		return nil, err
	}

	// Заполнить VotedValue для каждого блога
	if requestorUID != "" {
		s.fillVotedValues(blogs, requestorUID)
	}

	return blogs, nil
}

// RemoveExpiredFeaturedBlogs - удалить просроченные посты из подборки
func (s *BlogService) RemoveExpiredFeaturedBlogs(ndcId int) error {
	now := utils.CustomTime{Time: time.Now()}
	return s.db.Where("ndc_id = ? AND featured_until <= ?", ndcId, now).
		Delete(&blog.FeaturedPost{}).Error
}

// logFeaturedAction - логировать действие с подборкой
func (s *BlogService) logFeaturedAction(tx *gorm.DB, ndcId int, operatorUID, targetUID, blogID string, featured bool, note string) error {
	opType := 104 // OpTypeFeature
	if !featured {
		opType = 105 // OpTypeUnfeature
	}

	log := struct {
		NdcID         int              `gorm:"index"`
		OperatorUID   string           `gorm:"index"`
		TargetUID     string           `gorm:"index"`
		ObjectID      *string          `gorm:"index"`
		ObjectType    int
		OperationType int
		Note          string           `gorm:"type:text"`
		CreatedTime   utils.CustomTime
	}{
		NdcID:         ndcId,
		OperatorUID:   operatorUID,
		TargetUID:     targetUID,
		ObjectID:      &blogID,
		ObjectType:    1, // ObjectTypeBlog
		OperationType: opType,
		Note:          note,
		CreatedTime:   utils.CustomTime{Time: time.Now()},
	}

	return tx.Table("admin_logs").Create(&log).Error
}

// VoteBlog - поставить или убрать лайк на пост
func (s *BlogService) VoteBlog(blogID string, uid string, voteValue int) error {
	// Проверить существование блога
	var b blog.Blog
	if err := s.db.First(&b, "blog_id = ?", blogID).Error; err != nil {
		return fmt.Errorf("blog not found")
	}

	// Валидация voteValue: должен быть только 1 (лайк) или 0 (убрать лайк)
	if voteValue != 1 && voteValue != 0 {
		return fmt.Errorf("invalid vote value: must be 1 (like) or 0 (unlike)")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var existingVote blog.Vote
		err := tx.Where("blog_id = ? AND user_uid = ?", blogID, uid).First(&existingVote).Error

		if err == nil {
			// Голос уже существует
			if voteValue == 0 {
				// Убираем лайк
				if err := tx.Delete(&existingVote).Error; err != nil {
					return err
				}
				// Уменьшаем счетчик на 1
				if err := tx.Model(&b).Update("votes_count", gorm.Expr("votes_count - 1")).Error; err != nil {
					return err
				}
			}
			// Если voteValue == 1, а лайк уже есть - ничего не делаем (защита от дублирования)
		} else {
			// Лайка нет - создаем новый
			if voteValue == 1 {
				newVote := &blog.Vote{
					BlogID:   blogID,
					UserUID:  uid,
					VoteType: 1,
				}
				if err := tx.Create(newVote).Error; err != nil {
					return err
				}

				// Увеличиваем счетчик на 1
				if err := tx.Model(&b).Update("votes_count", gorm.Expr("votes_count + 1")).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// GetUserVote - получить голос пользователя за пост
func (s *BlogService) GetUserVote(blogID string, uid string) (int, error) {
	var vote blog.Vote
	err := s.db.Where("blog_id = ? AND user_uid = ?", blogID, uid).First(&vote).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil // Нет голоса
		}
		return 0, err
	}
	return vote.VoteType, nil
}

// fillVotedValues - заполнить VotedValue для списка блогов
func (s *BlogService) fillVotedValues(blogs []blog.Blog, uid string) {
	if len(blogs) == 0 || uid == "" {
		return
	}

	// Собрать все blogID
	blogIDs := make([]string, len(blogs))
	for i, b := range blogs {
		blogIDs[i] = b.BlogID
	}

	// Получить все голоса пользователя за эти блоги одним запросом
	var votes []blog.Vote
	s.db.Where("blog_id IN ? AND user_uid = ?", blogIDs, uid).Find(&votes)

	// Создать map для быстрого поиска
	voteMap := make(map[string]int)
	for _, v := range votes {
		voteMap[v.BlogID] = v.VoteType
	}

	// Заполнить VotedValue
	for i := range blogs {
		if voteType, ok := voteMap[blogs[i].BlogID]; ok {
			blogs[i].VotedValue = &voteType
		}
	}
}

// AddBlogComment добавляет комментарий к посту
func (s *BlogService) AddBlogComment(authorUID, blogID, content, replyTo string) (*user.Comment, error) {
	var b blog.Blog
	if err := s.db.Where("blog_id = ?", blogID).First(&b).Error; err != nil {
		return nil, fmt.Errorf("blog not found")
	}

	comment := &user.Comment{
		ID:         generateUID(),
		ParentID:   blogID,
		ParentType: 1, // 1 = Blog
		Content:    content,
		AuthorUID:  authorUID,
		NdcID:      b.NdcID,
	}

	if replyTo != "" {
		var parent user.Comment
		if err := s.db.First(&parent, "id = ?", replyTo).Error; err != nil {
			return nil, fmt.Errorf("parent comment not found")
		}
		// Всегда ссылаемся на корневой комментарий
		if parent.RootCommentID != nil {
			comment.RootCommentID = parent.RootCommentID
		} else {
			comment.RootCommentID = &replyTo
		}
	}

	if err := s.db.Create(comment).Error; err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	// Обновить счётчик комментариев блога
	s.db.Model(&b).Update("comments_count", gorm.Expr("comments_count + 1"))

	// Обновить счётчик ответов корневого комментария
	if comment.RootCommentID != nil {
		s.db.Model(&user.Comment{}).Where("id = ?", *comment.RootCommentID).
			Update("subcomments_count", gorm.Expr("subcomments_count + 1"))
	}

	s.db.Preload("Author").First(comment, "id = ?", comment.ID)

	return comment, nil
}

// GetBlogComments получает корневые комментарии к посту (без ответов)
func (s *BlogService) GetBlogComments(blogID, sort string, start, size int) ([]user.Comment, error) {
	var comments []user.Comment

	query := s.db.Preload("Author").
		Where("parent_id = ? AND parent_type = ? AND root_comment_id IS NULL", blogID, 1)

	switch sort {
	case "oldest":
		query = query.Order("created_time ASC")
	default:
		query = query.Order("created_time DESC")
	}

	err := query.Offset(start).Limit(size).Find(&comments).Error
	return comments, err
}

// GetCommentReplies получает ответы на конкретный комментарий
func (s *BlogService) GetCommentReplies(commentID, sort string, start, size int) ([]user.Comment, error) {
	var replies []user.Comment

	query := s.db.Preload("Author").
		Where("root_comment_id = ?", commentID)

	switch sort {
	case "oldest":
		query = query.Order("created_time ASC")
	default:
		query = query.Order("created_time DESC")
	}

	err := query.Offset(start).Limit(size).Find(&replies).Error
	return replies, err
}

// DeleteBlogComment удаляет комментарий к посту
func (s *BlogService) DeleteBlogComment(commentID, requestorUID string, ndcId int) error {
	var comment user.Comment
	if err := s.db.First(&comment, "id = ?", commentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("comment not found")
		}
		return err
	}

	if comment.ParentType != 1 {
		return errors.New("comment not found")
	}

	permSvc := NewPermissionService(s.db)
	role := permSvc.GetEffectiveRole(requestorUID, ndcId, "")

	if role == user.RoleBanned {
		return errors.New("permission denied")
	}

	// Staff (Curator+) can delete
	if role >= user.RoleCurator {
		return s.deleteBlogCommentAndUpdate(comment)
	}

	// Author can delete own comment
	if comment.AuthorUID == requestorUID {
		return s.deleteBlogCommentAndUpdate(comment)
	}

	// Blog author can delete comments on their blog
	var b blog.Blog
	if err := s.db.Where("blog_id = ?", comment.ParentID).First(&b).Error; err == nil {
		if b.UID == requestorUID {
			return s.deleteBlogCommentAndUpdate(comment)
		}
	}

	return errors.New("permission denied")
}

func (s *BlogService) deleteBlogCommentAndUpdate(comment user.Comment) error {
	if err := s.db.Delete(&comment).Error; err != nil {
		return err
	}
	s.db.Model(&blog.Blog{}).Where("blog_id = ?", comment.ParentID).
		Update("comments_count", gorm.Expr("CASE WHEN comments_count > 0 THEN comments_count - 1 ELSE 0 END"))

	// Уменьшить счётчик ответов у корневого комментария
	if comment.RootCommentID != nil {
		s.db.Model(&user.Comment{}).Where("id = ?", *comment.RootCommentID).
			Update("subcomments_count", gorm.Expr("CASE WHEN subcomments_count > 0 THEN subcomments_count - 1 ELSE 0 END"))
	}
	return nil
}
