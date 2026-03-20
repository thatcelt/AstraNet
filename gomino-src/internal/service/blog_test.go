package service_test

import (
	"testing"

	"github.com/AugustLigh/GoMino/internal/models/blog"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupBlogTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	err = db.AutoMigrate(
		&user.User{},
		&blog.Blog{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestBlogService_CreateGlobalBlog(t *testing.T) {
	db := setupBlogTestDB(t)
	blogService := service.NewBlogService(db)

	// Create users
	// Note: UID needs to be unique if there are constraints, but here in memory it's fine.
	admin := &user.User{UID: "admin1", Nickname: "Admin", Role: user.RoleAstranet, NdcID: 0, AminoID: "admin_amino"}
	member := &user.User{UID: "member1", Nickname: "Member", Role: user.RoleMember, NdcID: 0, AminoID: "member_amino"}

	db.Create(admin)
	db.Create(member)

	t.Run("admin can create global blog", func(t *testing.T) {
		blog, err := blogService.CreateBlog(admin.UID, 0, "Global News", "Content", []utils.MediaItem{}, []utils.MediaItem{})
		assert.NoError(t, err)
		assert.NotNil(t, blog)
		assert.Equal(t, "Global News", blog.Title)
		assert.Equal(t, 0, blog.NdcID)
	})

	t.Run("member cannot create global blog", func(t *testing.T) {
		blog, err := blogService.CreateBlog(member.UID, 0, "Spam", "Content", []utils.MediaItem{}, []utils.MediaItem{})
		assert.Error(t, err)
		assert.Nil(t, blog)
		assert.Contains(t, err.Error(), "permission denied")
	})

	t.Run("member can create community blog", func(t *testing.T) {
		blog, err := blogService.CreateBlog(member.UID, 1, "Com Blog", "Content", []utils.MediaItem{}, []utils.MediaItem{})
		assert.NoError(t, err)
		assert.NotNil(t, blog)
		assert.Equal(t, 1, blog.NdcID)
	})

	t.Run("create blog with media", func(t *testing.T) {
		media := []utils.MediaItem{{Type: 1, URL: "http://example.com/img.png"}}
		blog, err := blogService.CreateBlog(member.UID, 1, "Media Blog", "...", media, []utils.MediaItem{})
		assert.NoError(t, err)
		assert.NotNil(t, blog.MediaList)
		assert.Len(t, *blog.MediaList, 1)
		assert.Equal(t, "http://example.com/img.png", (*blog.MediaList)[0].URL)
	})

	t.Run("get community blogs", func(t *testing.T) {
		blogs, err := blogService.GetCommunityBlogs(1, 0, 10)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(blogs), 2)
		// "Media Blog" was created last, so it should be first
		assert.Equal(t, "Media Blog", blogs[0].Title)
	})

	t.Run("get user blogs", func(t *testing.T) {
		blogs, err := blogService.GetUserBlogs(member.UID, 1, 0, 10)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(blogs), 2)
	})
	t.Run("member can update own blog", func(t *testing.T) {
		blogs, _ := blogService.GetUserBlogs(member.UID, 1, 0, 10)
		blogID := blogs[0].BlogID

		updates := map[string]interface{}{
			"title": "Updated Title",
		}
		updated, err := blogService.UpdateBlog(blogID, member.UID, updates)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Title", updated.Title)
	})

	t.Run("admin cannot update member blog", func(t *testing.T) {
		blogs, _ := blogService.GetUserBlogs(member.UID, 1, 0, 10)
		blogID := blogs[0].BlogID

		updates := map[string]interface{}{
			"title": "Hacked",
		}
		_, err := blogService.UpdateBlog(blogID, admin.UID, updates)
		assert.Error(t, err)
		assert.Equal(t, "only author can update blog", err.Error())
	})

	t.Run("member can delete own blog", func(t *testing.T) {
		// Create another blog for deletion
		b, _ := blogService.CreateBlog(member.UID, 1, "To Delete", "...", []utils.MediaItem{}, []utils.MediaItem{})

		err := blogService.DeleteBlog(b.BlogID, member.UID)
		assert.NoError(t, err)

		// Verify deleted
		_, err = blogService.GetBlog(b.BlogID)
		assert.Error(t, err)
	})
}
