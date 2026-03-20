package service

import (
	"testing"

	"github.com/AugustLigh/GoMino/internal/models/chat"
	"github.com/AugustLigh/GoMino/internal/models/community"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite DB and migrates permissions-related models
func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		// Logger: logger.Default.LogMode(logger.Info), // Uncomment for debug
	})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate
	err = db.AutoMigrate(&user.User{}, &chat.ThreadMember{}, &community.Community{}, &community.AdminLog{})
	if err != nil {
		panic("failed to migrate database: " + err.Error())
	}
	return db
}

func TestGetEffectiveRole(t *testing.T) {
	db := setupTestDB()
	svc := NewPermissionService(db)

	ndcId := 100
	threadId := "thread-1"

	// Setup Data
	users := []user.User{
		{UID: "u_astranet", NdcID: 0, AminoID: "a_astranet", Role: user.RoleAstranet},
		{UID: "u_global_banned", NdcID: 0, AminoID: "a_gbanned", Role: user.RoleBanned},
		{UID: "u_leader", NdcID: ndcId, AminoID: "a_leader", Role: user.RoleLeader},
		{UID: "u_member", NdcID: ndcId, AminoID: "a_member", Role: user.RoleMember},
		{UID: "u_com_banned", NdcID: ndcId, AminoID: "a_cbanned", Role: user.RoleBanned},
		{UID: "u_host", NdcID: ndcId, AminoID: "a_host", Role: user.RoleMember}, // Member in com, Host in chat
		{UID: "u_outsider", NdcID: 0, AminoID: "a_outsider", Role: user.RoleMember}, // Not in community
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("Failed to create users: %v", err)
	}

	members := []chat.ThreadMember{
		{ThreadID: threadId, UserUID: "u_host", Role: chat.MemberRoleHost},
		{ThreadID: threadId, UserUID: "u_leader", Role: chat.MemberRoleMember}, // Leader is just member in chat
		{ThreadID: threadId, UserUID: "u_com_banned", Role: chat.MemberRoleHost}, // Banned user is Host (should be ignored)
	}
	if err := db.Create(&members).Error; err != nil {
		t.Fatalf("Failed to create members: %v", err)
	}

	tests := []struct {
		name     string
		uid      string
		ndcId    int
		threadId string
		wantRole int
	}{
		{"Astranet Global", "u_astranet", ndcId, threadId, user.RoleAstranet},
		{"Global Banned", "u_global_banned", ndcId, threadId, user.RoleBanned},
		{"Community Leader", "u_leader", ndcId, "", user.RoleLeader},
		{"Community Leader inside Chat", "u_leader", ndcId, threadId, user.RoleLeader}, // Should take max(Leader, Member) = Leader
		{"Chat Host", "u_host", ndcId, threadId, int(chat.MemberRoleHost)}, // 20
		{"Chat Host outside Chat", "u_host", ndcId, "", user.RoleMember}, // 0
		{"Community Banned overrides Chat Role", "u_com_banned", ndcId, threadId, user.RoleBanned}, // -1 wins over 20
		{"Outsider", "u_outsider", ndcId, threadId, user.RoleMember}, // 0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.GetEffectiveRole(tt.uid, tt.ndcId, tt.threadId)
			assert.Equal(t, tt.wantRole, got)
		})
	}
}

func TestCanPerform(t *testing.T) {
	db := setupTestDB()
	svc := NewPermissionService(db)

	ndcId := 100
	threadId := "thread-1"

	// Create Users
	// u_admin: Leader (100)
	// u_mod: Curator (50)
	// u_host: Host (20)
	// u_cohost: CoHost (10)
	// u_user: Member (0)
	// u_banned: Banned (-1)

	setupUsers := []user.User{
		{UID: "u_admin", NdcID: ndcId, AminoID: "a_admin", Role: user.RoleLeader},
		{UID: "u_mod", NdcID: ndcId, AminoID: "a_mod", Role: user.RoleCurator},
		{UID: "u_host", NdcID: ndcId, AminoID: "a_thost", Role: user.RoleMember},
		{UID: "u_cohost", NdcID: ndcId, AminoID: "a_cohost", Role: user.RoleMember},
		{UID: "u_user", NdcID: ndcId, AminoID: "a_user", Role: user.RoleMember},
		{UID: "u_banned", NdcID: ndcId, AminoID: "a_banned", Role: user.RoleBanned},
	}
	if err := db.Create(&setupUsers).Error; err != nil {
		t.Fatalf("Failed to create users: %v", err)
	}

	setupMembers := []chat.ThreadMember{
		{ThreadID: threadId, UserUID: "u_host", Role: chat.MemberRoleHost},
		{ThreadID: threadId, UserUID: "u_cohost", Role: chat.MemberRoleCoHost},
		{ThreadID: threadId, UserUID: "u_user", Role: chat.MemberRoleMember},
		// u_admin and u_mod are implicit members (or not members, but have higher com role)
	}
	if err := db.Create(&setupMembers).Error; err != nil {
		t.Fatalf("Failed to create members: %v", err)
	}

	// Action: Delete Message (Requires CoHost/10)
	minRoleDelete := int(chat.MemberRoleCoHost)

	tests := []struct {
		name      string
		actor     string
		target    string
		allowSelf bool
		want      bool
	}{
		// Self Management
		{"User delete self", "u_user", "u_user", true, true},
		{"User delete self forbidden", "u_user", "u_user", false, false}, // Role 0 < 10

		// Banned User
		{"Banned user try self", "u_banned", "u_banned", true, false},
		{"Banned user try other", "u_banned", "u_user", true, false},

		// Regular Role Checks
		{"User delete other", "u_user", "u_host", true, false}, // 0 < 10
		{"CoHost delete User", "u_cohost", "u_user", true, true}, // 10 >= 10, 10 > 0
		{"Host delete CoHost", "u_host", "u_cohost", true, true}, // 20 >= 10, 20 > 10

		// Hierarchy Checks
		{"CoHost delete Host", "u_cohost", "u_host", true, false}, // 10 >= 10 BUT 10 < 20
		{"CoHost delete Admin", "u_cohost", "u_admin", true, false}, // 10 < 100
		
		// Cross-Scope (Community Role vs Chat Role)
		{"Admin delete Host", "u_admin", "u_host", true, true}, // 100 > 20
		{"Mod delete CoHost", "u_mod", "u_cohost", true, true}, // 50 > 10
		{"Mod delete Admin", "u_mod", "u_admin", true, false}, // 50 < 100
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.CanPerform(tt.actor, tt.target, ndcId, threadId, minRoleDelete, tt.allowSelf)
			assert.Equal(t, tt.want, got)
		})
	}
}
