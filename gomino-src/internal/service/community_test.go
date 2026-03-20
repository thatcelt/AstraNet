package service

import (
	"testing"

	"github.com/AugustLigh/GoMino/internal/models/community"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/stretchr/testify/assert"
)

func TestCommunityModerationFlow(t *testing.T) {
	db := setupTestDB()
	svc := NewCommunityService(db)

	ndcId := 100

	// 1. Setup Data
	// Create Community
	com := community.Community{
		NdcId: ndcId,
		Name:  "Test Community",
		Link:  "/c/test",
		Agent: user.DetailedAuthor{
			Author: user.Author{UID: "u_agent"},
		},
	}
	db.Create(&com)

	// Create Users
	users := []user.User{
		{UID: "u_agent", NdcID: ndcId, AminoID: "a_agent", Role: user.RoleAgent},
		{UID: "u_leader", NdcID: ndcId, AminoID: "a_leader", Role: user.RoleLeader},
		{UID: "u_curator", NdcID: ndcId, AminoID: "a_curator", Role: user.RoleCurator},
		{UID: "u_member", NdcID: ndcId, AminoID: "a_member", Role: user.RoleMember},
		{UID: "u_victim", NdcID: ndcId, AminoID: "a_victim", Role: user.RoleMember},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("Failed to create users: %v", err)
	}

	// 2. Test Ban Scenarios

	// Case A: Curator tries to ban Member (Should Fail)
	err := svc.BanUser(ndcId, "u_curator", "u_victim", "Spam")
	assert.Error(t, err)
	assert.Equal(t, ErrPermissionDenied, err)

	// Case B: Leader bans Victim (Should Success)
	err = svc.BanUser(ndcId, "u_leader", "u_victim", "Spam & Violations")
	assert.NoError(t, err)

	// Verify Victim is Banned
	var victim user.User
	db.Where("uid = ? AND ndc_id = ?", "u_victim", ndcId).First(&victim)
	assert.Equal(t, user.RoleBanned, victim.Role)

	// Case C: Leader tries to ban Agent (Should Fail)
	err = svc.BanUser(ndcId, "u_leader", "u_agent", "Mutiny")
	assert.Error(t, err) // Permission Denied

	// 3. Check Logs
	// We expect 1 log entry (from Case B)
	logs, err := svc.GetModerationHistory(ndcId, nil, nil, 0, 10)
	assert.NoError(t, err)
	assert.Len(t, logs, 1)
	
	if len(logs) > 0 {
		assert.Equal(t, "u_leader", logs[0].OperatorUID)
		assert.Equal(t, "u_victim", logs[0].TargetUID)
		assert.Equal(t, "Spam & Violations", logs[0].Note)
		assert.Equal(t, community.OpTypeBan, logs[0].OperationType)
	}

	// 4. Test Unban Scenarios

	// Case D: Curator tries to unban (Should Fail)
	err = svc.UnbanUser(ndcId, "u_curator", "u_victim", "Please")
	assert.Error(t, err)

	// Case E: Leader unbans Victim (Should Success)
	err = svc.UnbanUser(ndcId, "u_leader", "u_victim", "Apologized")
	assert.NoError(t, err)

	// Verify Victim is Member again
	db.Where("uid = ? AND ndc_id = ?", "u_victim", ndcId).First(&victim)
	assert.Equal(t, user.RoleMember, victim.Role)

	// 5. Check Logs Again
	// Expect 2 logs (Ban + Unban)
	logs, err = svc.GetModerationHistory(ndcId, nil, nil, 0, 10)
	assert.NoError(t, err)
	assert.Len(t, logs, 2)
	
	// Logs are ordered DESC (newest first)
	if len(logs) >= 2 {
		assert.Equal(t, community.OpTypeUnban, logs[0].OperationType)
		assert.Equal(t, "Apologized", logs[0].Note)
		
		assert.Equal(t, community.OpTypeBan, logs[1].OperationType)
	}
}
