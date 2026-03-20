package chat_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	chatapi "github.com/AugustLigh/GoMino/internal/api/chat"
	"github.com/AugustLigh/GoMino/internal/models/chat"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&chat.Thread{}, &chat.ThreadMember{})
	require.NoError(t, err)

	return db
}

// setupApp creates a Fiber app with middleware that injects db and auid into context.
func setupApp(db *gorm.DB, auid string) *fiber.App {
	app := fiber.New()
	app.Post("/chat/thread/:threadId/member/:userId/background", func(c fiber.Ctx) error {
		c.Locals("db", db)
		c.Locals("auid", auid)
		return c.Next()
	}, chatapi.SetBackgroundImage)
	return app
}

func intPtr(v int) *int { return &v }

func createThread(t *testing.T, db *gorm.DB, threadID string, threadType int) {
	thread := chat.Thread{
		ThreadID: threadID,
		Type:     intPtr(threadType),
	}
	require.NoError(t, db.Create(&thread).Error)
}

func addMember(t *testing.T, db *gorm.DB, threadID, userUID string, role chat.MemberRole) {
	member := chat.ThreadMember{
		ThreadID: threadID,
		UserUID:  userUID,
		Role:     role,
	}
	require.NoError(t, db.Create(&member).Error)
}

func TestSetBackgroundImage_DMChatMemberAllowed(t *testing.T) {
	db := setupTestDB(t)

	createThread(t, db, "dm-thread-1", 1) // type=1 -> DM
	addMember(t, db, "dm-thread-1", "user-a", chat.MemberRoleMember)
	addMember(t, db, "dm-thread-1", "user-b", chat.MemberRoleMember)

	body, _ := json.Marshal(map[string]interface{}{
		"media": []interface{}{100, "https://example.com/bg.jpg", nil},
	})

	app := setupApp(db, "user-b")
	req := httptest.NewRequest(http.MethodPost, "/chat/thread/dm-thread-1/member/user-b/background", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify background was set
	var thread chat.Thread
	db.First(&thread, "thread_id = ?", "dm-thread-1")
	assert.NotNil(t, thread.Extensions)
	assert.Len(t, thread.Extensions.BM, 3)
	assert.Equal(t, "https://example.com/bg.jpg", thread.Extensions.BM[1])
}

func TestSetBackgroundImage_GroupChatMemberDenied(t *testing.T) {
	db := setupTestDB(t)

	createThread(t, db, "group-thread-1", 0) // type=0 -> group
	addMember(t, db, "group-thread-1", "host-user", chat.MemberRoleHost)
	addMember(t, db, "group-thread-1", "regular-user", chat.MemberRoleMember)

	body, _ := json.Marshal(map[string]interface{}{
		"media": []interface{}{100, "https://example.com/bg.jpg", nil},
	})

	app := setupApp(db, "regular-user")
	req := httptest.NewRequest(http.MethodPost, "/chat/thread/group-thread-1/member/regular-user/background", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestSetBackgroundImage_GroupChatHostAllowed(t *testing.T) {
	db := setupTestDB(t)

	createThread(t, db, "group-thread-2", 0)
	addMember(t, db, "group-thread-2", "host-user", chat.MemberRoleHost)

	body, _ := json.Marshal(map[string]interface{}{
		"media": []interface{}{100, "https://example.com/bg2.jpg", nil},
	})

	app := setupApp(db, "host-user")
	req := httptest.NewRequest(http.MethodPost, "/chat/thread/group-thread-2/member/host-user/background", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var thread chat.Thread
	db.First(&thread, "thread_id = ?", "group-thread-2")
	assert.NotNil(t, thread.Extensions)
	assert.Equal(t, "https://example.com/bg2.jpg", thread.Extensions.BM[1])
}
