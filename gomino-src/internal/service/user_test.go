package service_test

import (
	"testing"

	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB создаёт временную in-memory БД для тестов
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Автомиграция
	err = db.AutoMigrate(
		&user.User{},
		&user.UserFollow{},
		&user.AvatarFrame{},
		&user.CustomTitle{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestUserService_Following(t *testing.T) {
	db := setupTestDB(t)
	userService := service.NewUserService(db)

	// Создаём двух пользователей: follower и target
	follower := &user.User{UID: "follower1", Nickname: "Follower", AminoID: "f1", Icon: "f.png"}
	target := &user.User{UID: "target1", Nickname: "Target", AminoID: "t1", Icon: "t.png"}
	assert.NoError(t, userService.CreateUser(follower))
	assert.NoError(t, userService.CreateUser(target))

	// Подписка
	err := userService.FollowUser(follower.UID, target.UID)
	assert.NoError(t, err)

	// Проверяем, что GetFollowing возвращает target
	following, err := userService.GetFollowing(follower.UID, 0, 10)
	assert.NoError(t, err)
	assert.Len(t, following, 1)
	assert.Equal(t, target.UID, following[0].UID)
	assert.Equal(t, 1, following[0].FollowingStatus)

	// Отписка и проверка пустого списка
	err = userService.UnfollowUser(follower.UID, target.UID)
	assert.NoError(t, err)
	following, err = userService.GetFollowing(follower.UID, 0, 10)
	assert.NoError(t, err)
	assert.Len(t, following, 0)
}

func TestUserService_CreateUser(t *testing.T) {
	db := setupTestDB(t)
	userService := service.NewUserService(db)

	t.Run("successful creation", func(t *testing.T) {
		user := &user.User{
			UID:      "test123",
			Nickname: "TestUser",
			AminoID:  "amino123",
			Icon:     "default.png",
		}

		err := userService.CreateUser(user)
		assert.NoError(t, err)
		assert.NotZero(t, user.ID)
	})

	t.Run("duplicate UID", func(t *testing.T) {
		user1 := &user.User{
			UID:      "duplicate123",
			Nickname: "User1",
			AminoID:  "amino1",
			Icon:     "icon.png",
		}
		err := userService.CreateUser(user1)
		assert.NoError(t, err)

		user2 := &user.User{
			UID:      "duplicate123",
			Nickname: "User2",
			AminoID:  "amino2",
			Icon:     "icon.png",
		}
		err = userService.CreateUser(user2)
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrUserAlreadyExists)
	})

	t.Run("missing required fields", func(t *testing.T) {
		user := &user.User{
			UID: "", // Empty UID
		}
		err := userService.CreateUser(user)
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrInvalidUserData)
	})
}

func TestUserService_GetUserByID(t *testing.T) {
	db := setupTestDB(t)
	userService := service.NewUserService(db)

	// Создаём тестового пользователя
	user := &user.User{
		UID:      "gettest123",
		Nickname: "GetTestUser",
		AminoID:  "amino_get",
		Icon:     "default.png",
	}
	err := userService.CreateUser(user)
	assert.NoError(t, err)

	t.Run("existing user", func(t *testing.T) {
		foundUser, err := userService.GetUserByID("gettest123", 0, false)
		assert.NoError(t, err)
		assert.NotNil(t, foundUser)
		assert.Equal(t, "GetTestUser", foundUser.Nickname)
	})

	t.Run("non-existing user", func(t *testing.T) {
		foundUser, err := userService.GetUserByID("nonexistent", 0, false)
		assert.Error(t, err)
		assert.Nil(t, foundUser)
		assert.ErrorIs(t, err, service.ErrUserNotFound)
	})
}

func TestUserService_UpdateUser(t *testing.T) {
	db := setupTestDB(t)
	userService := service.NewUserService(db)

	// Создаём пользователя
	user := &user.User{
		UID:      "updatetest123",
		Nickname: "OldNickname",
		AminoID:  "amino_update",
		Icon:     "default.png",
	}
	err := userService.CreateUser(user)
	assert.NoError(t, err)

	t.Run("successful update", func(t *testing.T) {
		newNickname := "NewNickname"
		updates := map[string]interface{}{
			"nickname": newNickname,
		}

		updatedUser, err := userService.UpdateUser("updatetest123", 0, updates)
		assert.NoError(t, err)
		assert.NotNil(t, updatedUser)
		assert.Equal(t, newNickname, updatedUser.Nickname)
	})

	t.Run("update non-existing user", func(t *testing.T) {
		updates := map[string]interface{}{
			"nickname": "SomeNickname",
		}

		updatedUser, err := userService.UpdateUser("nonexistent", 0, updates)
		assert.Error(t, err)
		assert.Nil(t, updatedUser)
		assert.ErrorIs(t, err, service.ErrUserNotFound)
	})

	t.Run("empty updates", func(t *testing.T) {
		updates := map[string]interface{}{}

		updatedUser, err := userService.UpdateUser("updatetest123", 0, updates)
		assert.Error(t, err)
		assert.Nil(t, updatedUser)
		assert.ErrorIs(t, err, service.ErrInvalidUserData)
	})
}

func TestUserService_DeleteUser(t *testing.T) {
	db := setupTestDB(t)
	userService := service.NewUserService(db)

	// Создаём пользователя
	user := &user.User{
		UID:      "deletetest123",
		Nickname: "DeleteMe",
		AminoID:  "amino_delete",
		Icon:     "default.png",
	}
	err := userService.CreateUser(user)
	assert.NoError(t, err)

	t.Run("successful deletion", func(t *testing.T) {
		err := userService.DeleteUser("deletetest123")
		assert.NoError(t, err)

		// Проверяем что пользователь удалён
		deletedUser, err := userService.GetUserByID("deletetest123", 0, false)
		assert.Error(t, err)
		assert.Nil(t, deletedUser)
	})

	t.Run("delete non-existing user", func(t *testing.T) {
		err := userService.DeleteUser("nonexistent")
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrUserNotFound)
	})
}