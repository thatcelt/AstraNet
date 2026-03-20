package blog

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/AugustLigh/GoMino/internal/middleware"
	blogModel "github.com/AugustLigh/GoMino/internal/models/blog"
	utilsModel "github.com/AugustLigh/GoMino/internal/models/utils"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
)

// CreateGlobalBlog godoc
// @Summary Create a global blog post
// @Description Create a blog post accessible globally (only for admins/Astranet team)
// @Tags blog
// @Accept  json
// @Produce  json
// @Param   request body CreateBlogRequest true "Blog details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/blog [post]
func CreateGlobalBlog(c fiber.Ctx) error {
	var req CreateBlogRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)

	// ndcId = 0 for global
	blog, err := svc.CreateBlog(uid, 0, req.Title, req.Content, req.MediaList, req.GetBackgroundMediaList())
	if err != nil {
		if err.Error() == "permission denied: only Astranet team can create global blogs" {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"blog": blog,
	})
}

// CreateCommunityBlog godoc
// @Summary Create a community blog post
// @Description Create a blog post within a specific community
// @Tags blog
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   request body CreateBlogRequest true "Blog details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/blog [post]
func CreateCommunityBlog(c fiber.Ctx) error {
	ndcId, err := getNdcId(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	var req CreateBlogRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)

	blog, err := svc.CreateBlog(uid, ndcId, req.Title, req.Content, req.MediaList, req.GetBackgroundMediaList())
	if err != nil {
		if err.Error() == "permission denied: user is banned" {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Notify followers about new post
	go func() {
		notificationSvc := service.NewNotificationService(db, nil)
		_ = notificationSvc.NotifyFollowersAboutNewPost(uid, ndcId, blog)
	}()

	return c.JSON(fiber.Map{
		"blog": blog,
	})
}

// GetCommunityBlogFeed godoc
// @Summary Get community blog feed
// @Description Get a list of recent blogs in the community
// @Tags blog
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/feed/blog-all [get]
func GetCommunityBlogFeed(c fiber.Ctx) error {
	ndcId, err := getNdcId(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	start, _ := strconv.Atoi(c.Query("start", "0"))
	size := 25
	if s := c.Query("size"); s != "" {
		if val, err := strconv.Atoi(s); err == nil {
			size = val
		}
	}

	uid := middleware.GetAUIDFromContext(c)

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)

	blogs, err := svc.GetCommunityBlogs(ndcId, start, size, uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	truncateBlogContent(blogs, 100)

	return c.JSON(fiber.Map{
		"blogList": blogs,
	})
}

// GetCommunityUserBlogs godoc
// @Summary Get user blogs in community
// @Description Get a list of blogs created by a specific user in the community
// @Tags blog
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   q query string true "User ID"
// @Param   size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/blog [get]
func GetCommunityUserBlogs(c fiber.Ctx) error {
	ndcId, err := getNdcId(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	targetUID := c.Query("q") // q={userId}
	if targetUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))
	if size > 100 {
		size = 100
	}

	requestorUID := middleware.GetAUIDFromContext(c)

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)

	blogs, err := svc.GetUserBlogs(targetUID, ndcId, start, size, requestorUID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	truncateBlogContent(blogs, 100)

	return c.JSON(fiber.Map{
		"blogList": blogs,
	})
}

// GetSingleBlog godoc
// @Summary Get a single blog post
// @Description Get details of a specific blog post
// @Tags blog
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   blogId path string true "Blog ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} response.ErrorResponse
// @Router /x{comId}/s/blog/{blogId} [get]
func GetSingleBlog(c fiber.Ctx) error {
	blogId := c.Params("blogId")
	uid := middleware.GetAUIDFromContext(c)

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)

	var blog interface{}
	var err error

	// Если пользователь авторизован, получаем с информацией о его голосе
	if uid != "" {
		blog, err = svc.GetBlogWithVote(blogId, uid)
	} else {
		blog, err = svc.GetBlog(blogId)
	}

	if err != nil {
		// Handle not found
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
	}

	return c.JSON(fiber.Map{
		"blog": blog,
	})
}

// DeleteCommunityBlog godoc
// @Summary Delete a blog post
// @Description Delete a specific blog post
// @Tags blog
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   blogId path string true "Blog ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/blog/{blogId} [delete]
func DeleteCommunityBlog(c fiber.Ctx) error {
	blogId := c.Params("blogId")
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)

	if err := svc.DeleteBlog(blogId, uid); err != nil {
		if err.Error() == "permission denied" {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err.Error() == "blog not found" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Blog deleted",
	})
}

// UpdateCommunityBlog godoc
// @Summary Update a blog post
// @Description Update details of a specific blog post
// @Tags blog
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   blogId path string true "Blog ID"
// @Param   request body CreateBlogRequest true "Update data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/blog/{blogId} [post]
func UpdateCommunityBlog(c fiber.Ctx) error {
	blogId := c.Params("blogId")
	uid := middleware.GetAUIDFromContext(c)
	
	var req CreateBlogRequest // Reusing for now
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.MediaList != nil {
		updates["media_list"] = req.MediaList
	}
	if bgMedia := req.GetBackgroundMediaList(); bgMedia != nil {
		updates["extensions"] = &blogModel.BlogExtensions{BackgroundMediaList: &bgMedia}
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)
	
	updatedBlog, err := svc.UpdateBlog(blogId, uid, updates)
	if err != nil {
		if err.Error() == "only author can update blog" {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	
	return c.JSON(fiber.Map{
		"blog": updatedBlog,
	})
}

// truncateBlogContent обрезает контент блогов для превью в ленте
func truncateBlogContent(blogs []blogModel.Blog, maxRunes int) {
	for i := range blogs {
		runes := []rune(blogs[i].Content)
		if len(runes) > maxRunes {
			blogs[i].Content = string(runes[:maxRunes]) + "..."
		}
	}
}

func getNdcId(c fiber.Ctx) (int, error) {
	comIdStr := c.Params("comId")
	return strconv.Atoi(comIdStr)
}

// FeatureBlog godoc
// @Summary Add blog to featured
// @Description Add a blog post to the community featured list (Curator+ only)
// @Tags blog-admin
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   blogId path string true "Blog ID"
// @Param   request body FeatureBlogRequest true "Feature duration"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/blog/{blogId}/feature [post]
func FeatureBlog(c fiber.Ctx) error {
	ndcId, err := getNdcId(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	blogId := c.Params("blogId")
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req FeatureBlogRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)

	if err := svc.FeatureBlog(blogId, ndcId, uid, req.Days); err != nil {
		if err.Error() == "permission denied" {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err.Error() == "invalid duration: must be 1, 2, 3, or 4 days" {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
		}
		if err.Error() == "blog is already featured" {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
		}
		if err.Error() == "blog not found in this community" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Blog featured successfully",
	})
}

// UnfeatureBlog godoc
// @Summary Remove blog from featured
// @Description Remove a blog post from the community featured list (Curator+ only)
// @Tags blog-admin
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   blogId path string true "Blog ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/blog/{blogId}/feature [delete]
func UnfeatureBlog(c fiber.Ctx) error {
	ndcId, err := getNdcId(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	blogId := c.Params("blogId")
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)

	if err := svc.UnfeatureBlog(blogId, ndcId, uid); err != nil {
		if err.Error() == "permission denied" {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err.Error() == "blog is not featured" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Blog unfeatured successfully",
	})
}

// GetFeaturedBlogs godoc
// @Summary Get featured blogs
// @Description Get a list of featured blogs in the community
// @Tags blog
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/feed/featured [get]
func GetFeaturedBlogs(c fiber.Ctx) error {
	ndcId, err := getNdcId(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	start, _ := strconv.Atoi(c.Query("start", "0"))
	size := 25
	if s := c.Query("size"); s != "" {
		if val, err := strconv.Atoi(s); err == nil {
			size = val
		}
	}

	uid := middleware.GetAUIDFromContext(c)

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)

	blogs, err := svc.GetFeaturedBlogs(ndcId, start, size, uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	truncateBlogContent(blogs, 100)

	return c.JSON(fiber.Map{
		"blogList": blogs,
	})
}

// VoteBlog godoc
// @Summary Like or unlike a blog
// @Description Like a blog post (1 = like, 0 = unlike)
// @Tags blog
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   blogId path string true "Blog ID"
// @Param   request body VoteBlogRequest true "Vote value (1 or 0)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/blog/{blogId}/g-vote [post]
func VoteBlog(c fiber.Ctx) error {
	blogId := c.Params("blogId")
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req VoteBlogRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)

	if err := svc.VoteBlog(blogId, uid, req.Value); err != nil {
		if err.Error() == "blog not found" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		if err.Error() == "invalid vote value" {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Send notification about blog like
	if req.Value == 1 {
		go func() {
			blogPost, err := svc.GetBlog(blogId)
			if err != nil {
				return
			}
			notificationSvc := service.NewNotificationService(db, nil)
			_ = notificationSvc.NotifyBlogAuthorAboutLike(blogPost.UID, uid, blogPost.NdcID, blogId, blogPost.Title)
		}()
	}

	return c.JSON(fiber.Map{
		"message": "Vote recorded",
	})
}

// GetBlogComments - получить комментарии к посту
func GetBlogComments(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	blogId := c.Params("blogId")
	uid := middleware.GetAUIDFromContext(c)

	sort := c.Query("sort", "newest")
	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))
	if size > 100 {
		size = 100
	}

	svc := service.NewBlogService(db)
	comments, err := svc.GetBlogComments(blogId, sort, start, size, uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"commentList": comments,
	})
}

// PostBlogComment - добавить комментарий к посту
func PostBlogComment(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	blogId := c.Params("blogId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusInvalidRequest))
	}

	var req BlogCommentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	svc := service.NewBlogService(db)

	// Check if blog author has blocked the commenter
	blogPost, err := svc.GetBlog(blogId)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
	}
	userSvc := service.NewUserService(db)
	if userSvc.IsBlocked(blogPost.UID, auid) {
		return c.Status(fiber.StatusForbidden).JSON(response.BlockedByUser())
	}

	replyTo := ""
	if req.RespondTo != nil {
		replyTo = *req.RespondTo
	}

	// Prepare media list
	var mediaList *utilsModel.MediaList
	if len(req.MediaList) > 0 {
		ml := utilsModel.MediaList(req.MediaList)
		mediaList = &ml
	}

	comment, err := svc.AddBlogComment(auid, blogId, req.Content, replyTo, req.Type, req.StickerID, req.StickerIcon, req.StickerMedia, mediaList)
	if err != nil {
		if err.Error() == "blog not found" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"comment": comment,
	})
}

// GetCommentReplies - получить ответы на комментарий
func GetCommentReplies(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	commentId := c.Params("commentId")
	uid := middleware.GetAUIDFromContext(c)

	sort := c.Query("sort", "oldest")
	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))
	if size > 100 {
		size = 100
	}

	svc := service.NewBlogService(db)
	replies, err := svc.GetCommentReplies(commentId, sort, start, size, uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"commentList": replies,
	})
}

// VoteComment - лайк/убрать лайк с комментария
func VoteComment(c fiber.Ctx) error {
	commentId := c.Params("commentId")
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req VoteCommentRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	db := middleware.GetDBFromContext(c)
	svc := service.NewBlogService(db)

	if err := svc.VoteComment(commentId, uid, req.Value); err != nil {
		if err.Error() == "comment not found" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		if err.Error() == "invalid vote value: must be 1 (like) or 0 (unlike)" {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Send notification about comment like
	if req.Value == 1 {
		go func() {
			comment, err := svc.GetCommentByID(commentId)
			if err != nil {
				return
			}
			notificationSvc := service.NewNotificationService(db, nil)
			_ = notificationSvc.NotifyCommentAuthorAboutLike(comment.AuthorUID, uid, comment.NdcID, comment.ParentID, commentId, comment.Content)
		}()
	}

	return c.JSON(fiber.Map{
		"message": "Vote recorded",
	})
}

// DeleteBlogComment - удалить комментарий к посту
func DeleteBlogComment(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	commentId := c.Params("commentId")
	auid := middleware.GetAUIDFromContext(c)

	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusInvalidRequest))
	}

	comIdStr := c.Params("comId")
	ndcId, _ := strconv.Atoi(comIdStr)

	svc := service.NewBlogService(db)
	err := svc.DeleteBlogComment(commentId, auid, ndcId)
	if err != nil {
		if err.Error() == "comment not found" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		if err.Error() == "permission denied" {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusInvalidRequest))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Comment deleted",
	})
}

// ==================== Moderation ====================

// HideBlog godoc
// @Summary Hide a blog post
// @Description Hide a blog post from regular users (Curator+ required)
// @Tags blog-moderation
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   blogId path string true "Blog ID"
// @Param   request body ModerationRequest false "Reason for hiding"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/blog/{blogId}/hide [post]
func HideBlog(c fiber.Ctx) error {
	ndcId, err := getNdcId(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	blogId := c.Params("blogId")
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req ModerationRequest
	c.Bind().Body(&req)

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	if err := svc.HideBlog(ndcId, uid, blogId, req.Reason); err != nil {
		if err == service.ErrPermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err.Error() == "blog not found" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Blog hidden",
	})
}

// UnhideBlog godoc
// @Summary Unhide a blog post
// @Description Make a hidden blog post visible again (Curator+ required)
// @Tags blog-moderation
// @Accept  json
// @Produce  json
// @Param   comId path string true "Community ID (NDC ID)"
// @Param   blogId path string true "Blog ID"
// @Param   request body ModerationRequest false "Reason for unhiding"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/blog/{blogId}/unhide [post]
func UnhideBlog(c fiber.Ctx) error {
	ndcId, err := getNdcId(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	blogId := c.Params("blogId")
	uid := middleware.GetAUIDFromContext(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	var req ModerationRequest
	c.Bind().Body(&req)

	db := middleware.GetDBFromContext(c)
	svc := service.NewCommunityService(db)

	if err := svc.UnhideBlog(ndcId, uid, blogId, req.Reason); err != nil {
		if err == service.ErrPermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		if err.Error() == "blog not found" {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"message": "Blog unhidden",
	})
}
