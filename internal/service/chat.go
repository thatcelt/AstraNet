package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/AugustLigh/GoMino/internal/models/chat"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	pushService "github.com/AugustLigh/GoMino/internal/service/push"
	"gorm.io/gorm"
)

type ChatService struct {
	db *gorm.DB
}

func NewChatService(db *gorm.DB) *ChatService {
	return &ChatService{db: db}
}

// authorScope возвращает scope для Preload Author/User с фильтрацией по ndc_id
func authorScope(ndcId int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("ndc_id = ?", ndcId)
	}
}

// getNdcIdFromThread получает ndc_id из треда
func getNdcIdFromThread(thread *chat.Thread) int {
	if thread.NdcID != nil {
		return *thread.NdcID
	}
	return 0
}

// FindExistingDM - поиск существующего DM чата между двумя пользователями
func (s *ChatService) FindExistingDM(userUID, otherUID string, ndcId int) (*chat.Thread, error) {
	var thread chat.Thread
	err := s.db.
		Joins("JOIN thread_members m1 ON m1.thread_id = threads.thread_id AND m1.user_uid = ? AND m1.deleted_at IS NULL", userUID).
		Joins("JOIN thread_members m2 ON m2.thread_id = threads.thread_id AND m2.user_uid = ? AND m2.deleted_at IS NULL", otherUID).
		Where("threads.type = ? AND threads.ndc_id = ?", 1, ndcId).
		First(&thread).Error
	if err != nil {
		return nil, err
	}

	// Загрузить автора и DM info
	s.db.Preload("Author", authorScope(ndcId)).First(&thread, thread.ID)
	s.populateDMInfo(&thread, userUID)

	return &thread, nil
}

// CreateThread - создание нового чата
func (s *ChatService) CreateThread(userUID, title, content, icon string, threadType, ndcId int, inviteeUids []string) (*chat.Thread, error) {
	// Инициализация extensions с дефолтными значениями
	defaultLanguage := "ru"
	defaultMembersCanInvite := true

	// Для DM (type=1) не сохраняем UID организатора — у DM нет владельца
	threadUID := userUID
	if threadType == 1 {
		threadUID = ""
	}

	thread := &chat.Thread{
		ThreadID:           generateUID(),
		Title:              &title,
		Content:            &content,
		Type:               &threadType,
		UID:                threadUID,
		NdcID:              &ndcId, // Save NDC ID
		MembersSummary:     []interface{}{},
		UserAddedTopicList: []string{},
		Extensions: &chat.ThreadExtensions{
			Language:         &defaultLanguage,
			MembersCanInvite: &defaultMembersCanInvite,
			CoHost:           []string{},
		},
	}

	if icon != "" {
		thread.Icon = &icon
	}

	// Создать чат и добавить участников в транзакции
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Создать чат
		if err := tx.Create(thread).Error; err != nil {
			return fmt.Errorf("failed to create thread: %w", err)
		}

		membersCount := 0

		// Для DM (type=1) — оба участника равноправные (MemberRoleMember)
		// Для остальных — создатель становится Host
		creatorRole := chat.MemberRoleHost
		if threadType == 1 {
			creatorRole = chat.MemberRoleMember
		}

		// Добавить создателя
		creatorMember := &chat.ThreadMember{
			ThreadID: thread.ThreadID,
			UserUID:  userUID,
			Role:     creatorRole,
		}
		if err := tx.Create(creatorMember).Error; err != nil {
			return fmt.Errorf("failed to add creator as member: %w", err)
		}
		membersCount++

		// Добавить приглашённых участников
		for _, inviteeUID := range inviteeUids {
			if inviteeUID == userUID {
				continue // Пропускаем создателя, он уже добавлен
			}
			inviteeMember := &chat.ThreadMember{
				ThreadID: thread.ThreadID,
				UserUID:  inviteeUID,
				Role:     chat.MemberRoleMember,
			}
			if err := tx.Create(inviteeMember).Error; err != nil {
				return fmt.Errorf("failed to add invitee as member: %w", err)
			}
			membersCount++
		}

		// Обновить счетчик участников
		thread.MembersCount = &membersCount
		if err := tx.Model(thread).Update("members_count", membersCount).Error; err != nil {
			return fmt.Errorf("failed to update members count: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Загрузить автора для ответа
	s.db.Preload("Author", authorScope(ndcId)).First(thread, thread.ID)

	return thread, nil
}

// GetThread - получение чата по ID с опциональной загрузкой сообщений
// currentUserUID используется для DM чатов, чтобы показать данные собеседника
func (s *ChatService) GetThread(threadID string, loadMessages bool, currentUserUID ...string) (*chat.Thread, error) {
	var thread chat.Thread

	if err := s.db.First(&thread, "thread_id = ?", threadID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("thread not found")
		}
		return nil, err
	}

	ndcId := getNdcIdFromThread(&thread)

	// Загрузить автора с правильным ndc_id
	s.db.Preload("Author", authorScope(ndcId)).First(&thread, thread.ID)

	var lastMsg chat.Message
	if err := s.db.Preload("Author", authorScope(ndcId)).
		Where("thread_id = ?", threadID).
		Order("created_time DESC").
		Limit(1).
		Find(&lastMsg).Error; err == nil && lastMsg.ID != 0 {
		thread.LastMessageSummary = &lastMsg
	}

	// Для DM (type=1) установить название и иконку по собеседнику
	if thread.Type != nil && *thread.Type == 1 && len(currentUserUID) > 0 {
		s.populateDMInfo(&thread, currentUserUID[0])
	}

	return &thread, nil
}

// ListUserThreads - список чатов пользователя (где он участник)
func (s *ChatService) ListUserThreads(userUID string, ndcID int, start, size int) ([]chat.Thread, error) {
	var threads []chat.Thread

	// Найти чаты через JOIN с thread_members (исключая soft-deleted)
	query := s.db.
		Preload("Author", authorScope(ndcID)).
		Joins("JOIN thread_members ON thread_members.thread_id = threads.thread_id AND thread_members.deleted_at IS NULL").
		Where("thread_members.user_uid = ?", userUID)

	// Фильтрация по ndc_id
	query = query.Where("threads.ndc_id = ?", ndcID)

	err := query.
		Order("threads.latest_activity_time DESC").
		Limit(size).
		Offset(start).
		Find(&threads).Error

	if err != nil {
		return nil, err
	}

	// Для каждого чата получить последнее сообщение и обработать DM
	for i := range threads {
		var lastMsg chat.Message
		if err := s.db.Preload("Author", authorScope(ndcID)).
			Where("thread_id = ?", threads[i].ThreadID).
			Order("created_time DESC").
			Limit(1).
			Find(&lastMsg).Error; err == nil && lastMsg.ID != 0 {
			threads[i].LastMessageSummary = &lastMsg
		}

		// Для DM (type=1) установить название и иконку по собеседнику
		if threads[i].Type != nil && *threads[i].Type == 1 {
			s.populateDMInfo(&threads[i], userUID)
		}
	}

	return threads, nil
}

// populateDMInfo заполняет title и icon для DM чата данными собеседника
func (s *ChatService) populateDMInfo(thread *chat.Thread, currentUserUID string) {
	ndcId := getNdcIdFromThread(thread)

	// Найти другого участника DM
	var otherMember chat.ThreadMember
	err := s.db.Preload("User", authorScope(ndcId)).
		Where("thread_id = ? AND user_uid != ?", thread.ThreadID, currentUserUID).
		First(&otherMember).Error

	if err != nil {
		return
	}

	// Установить title как никнейм собеседника
	if otherMember.User.Nickname != "" {
		thread.Title = &otherMember.User.Nickname
	}

	// Для DM всегда использовать актуальный аватар собеседника
	if otherMember.User.Icon != "" {
		thread.Icon = &otherMember.User.Icon
	}
}

// GetPublicThreads - получение списка публичных чатов сообщества
func (s *ChatService) GetPublicThreads(ndcID int, filterType string, start, size int, requestorUID string) ([]chat.Thread, error) {
	var threads []chat.Thread

	permSvc := NewPermissionService(s.db)
	role := permSvc.GetEffectiveRole(requestorUID, ndcID, "")

	query := s.db.Preload("Author", authorScope(ndcID)).Where("threads.type = ?", 0)

	if ndcID > 0 {
		query = query.Where("threads.ndc_id = ?", ndcID)
	} else {
		query = query.Where("(threads.ndc_id = 0 OR threads.ndc_id IS NULL)")
	}

	if role >= user.RoleCurator {
		// Moderators see all threads (including hidden, status=1)
		query = query.Where("(threads.status = 0 OR threads.status = 1 OR threads.status IS NULL)")
	} else {
		// Regular users see only active threads (status = 0 or NULL)
		// and don't see threads from banned users
		query = query.Where("(threads.status = 0 OR threads.status IS NULL)").
			Joins("LEFT JOIN users ON threads.uid = users.uid AND users.ndc_id = ?", ndcID).
			Where("(users.role >= 0 OR users.role IS NULL OR threads.uid = '' OR threads.uid = ?)", requestorUID)
	}

	switch filterType {
	case "recent":
		query = query.Order("threads.latest_activity_time DESC")
	default:
		query = query.Order("threads.created_time DESC")
	}

	if err := query.Offset(start).Limit(size).Find(&threads).Error; err != nil {
		return nil, err
	}

	// Загружаем последнее сообщение для каждого чата
	for i := range threads {
		var lastMsg chat.Message
		if err := s.db.Preload("Author", authorScope(ndcID)).
			Where("thread_id = ?", threads[i].ThreadID).
			Order("created_time DESC").
			Limit(1).
			Find(&lastMsg).Error; err == nil && lastMsg.ID != 0 {
			threads[i].LastMessageSummary = &lastMsg
		}
	}

	return threads, nil
}

// SendMessageInput - параметры для отправки сообщения
type SendMessageInput struct {
	Content        string
	Type           int
	MediaType      int
	MediaValue     string
	StickerID      string
	ReplyMessageID string
	ClientRefID    int
	MentionedUIDs  []string
	Extensions     map[string]interface{}
}

// SendMessage - отправка сообщения в чат
func (s *ChatService) SendMessage(threadID, userUID string, input SendMessageInput) (*chat.Message, error) {
	// Проверить существование чата
	var thread chat.Thread
	if err := s.db.First(&thread, "thread_id = ?", threadID).Error; err != nil {
		return nil, fmt.Errorf("thread not found")
	}

	// Проверить, является ли пользователь участником чата
	var member chat.ThreadMember
	if err := s.db.Where("thread_id = ? AND user_uid = ?", threadID, userUID).First(&member).Error; err != nil {
		return nil, fmt.Errorf("user is not a member of this thread")
	}

	message := &chat.Message{
		MessageID:   generateUID(),
		ThreadID:    threadID,
		UID:         userUID,
		Content:     input.Content,
		Type:        input.Type,
		ClientRefID: input.ClientRefID,
		MediaType:   input.MediaType,
	}

	if input.MediaValue != "" {
		message.MediaValue = &input.MediaValue
	}

	if input.StickerID != "" {
		message.StickerID = &input.StickerID
	}

	if input.ReplyMessageID != "" {
		message.ReplyTo = &input.ReplyMessageID
	}

	// Handle Extensions: начинаем с переданных extensions (может содержать duration, waveform для голосовых)
	extensions := input.Extensions
	if extensions == nil {
		extensions = map[string]interface{}{}
	}

	// Мержим mentions в extensions
	if len(input.MentionedUIDs) > 0 {
		mentionedArray := make([]map[string]string, len(input.MentionedUIDs))
		for i, uid := range input.MentionedUIDs {
			mentionedArray[i] = map[string]string{"uid": uid}
		}
		extensions["mentionedArray"] = mentionedArray
	}

	if len(extensions) > 0 {
		message.Extensions = extensions
	}

	if err := s.db.Create(message).Error; err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Обновить время последней активности в чате
	now := utils.CustomTime{Time: time.Now()}
	s.db.Model(&thread).Updates(map[string]interface{}{
		"latest_activity_time": now,
		"modified_time":        now,
	})

	// Загрузить автора для ответа
	ndcId := getNdcIdFromThread(&thread)
	s.db.Preload("Author", authorScope(ndcId)).First(message, message.ID)

	return message, nil
}

// DeleteMessage - удаление сообщения
func (s *ChatService) DeleteMessage(threadID, messageID, userUID string) error {
	var message chat.Message
	if err := s.db.Where("thread_id = ? AND message_id = ?", threadID, messageID).First(&message).Error; err != nil {
		return fmt.Errorf("message not found")
	}

	// Получаем информацию о чате для NDC ID
	var thread chat.Thread
	if err := s.db.Select("ndc_id").First(&thread, "thread_id = ?", threadID).Error; err != nil {
		return fmt.Errorf("thread not found")
	}
	ndcId := 0
	if thread.NdcID != nil {
		ndcId = *thread.NdcID
	}

	permSvc := NewPermissionService(s.db)
	// Требуется минимум роль CoHost (10), чтобы удалять чужие сообщения
	// allowSelf=true разрешает удалять свои
	if !permSvc.CanPerform(userUID, message.UID, ndcId, threadID, int(chat.MemberRoleCoHost), true) {
		return fmt.Errorf("permission denied")
	}

	return s.db.Delete(&message).Error
}

// EditMessage - редактирование сообщения (только текстового контента)
func (s *ChatService) EditMessage(threadID, messageID, userUID, newContent string) (*chat.Message, error) {
	var message chat.Message
	if err := s.db.Where("thread_id = ? AND message_id = ?", threadID, messageID).First(&message).Error; err != nil {
		return nil, fmt.Errorf("message not found")
	}

	// Только автор может редактировать сообщение
	if message.UID != userUID {
		return nil, fmt.Errorf("permission denied: only the author can edit")
	}

	// Нельзя редактировать системные сообщения
	if message.Type >= 101 {
		return nil, fmt.Errorf("cannot edit system messages")
	}

	message.Content = newContent
	message.IsEdited = true

	if err := s.db.Save(&message).Error; err != nil {
		return nil, fmt.Errorf("failed to edit message: %w", err)
	}

	// Загружаем автора для ответа
	var thread chat.Thread
	if err := s.db.Select("ndc_id").First(&thread, "thread_id = ?", threadID).Error; err == nil {
		s.db.Preload("Author", authorScope(getNdcIdFromThread(&thread))).First(&message, message.ID)
	} else {
		s.db.Preload("Author").First(&message, message.ID)
	}

	return &message, nil
}

// MarkThreadAsRead - пометка чата как прочитанного
func (s *ChatService) MarkThreadAsRead(threadID, userUID string) error {
	// В Amino это обычно обновляет lastReadTime для конкретного участника
	// Но в нашей упрощенной модели мы можем просто вернуть успех или обновить поле в ThreadMember
	return s.db.Model(&chat.ThreadMember{}).
		Where("thread_id = ? AND user_uid = ?", threadID, userUID).
		Update("last_read_time", utils.CustomTime{Time: time.Now()}).Error
}

// GetMessagesOptions - опции для получения сообщений
type GetMessagesOptions struct {
	Start           int
	Size            int
	NdcID           int
	BeforeTime      *time.Time
	AroundMessageID *string
}

// GetMessages - получение сообщений чата с поддержкой разных стратегий пагинации
func (s *ChatService) GetMessages(threadID string, opts GetMessagesOptions) ([]chat.Message, error) {
	// Стратегия 1: Контекст вокруг сообщения (Jump to message)
	if opts.AroundMessageID != nil && *opts.AroundMessageID != "" {
		return s.getMessagesAround(threadID, *opts.AroundMessageID, opts.Size, opts.NdcID)
	}

	var messages []chat.Message
	scope := authorScope(opts.NdcID)
	query := s.db.
		Preload("Author", scope).
		Preload("ReplyMessage").
		Preload("ReplyMessage.Author", scope).
		Where("thread_id = ?", threadID)

	// Стратегия 2: Курсорная пагинация (Time-based, infinite scroll)
	if opts.BeforeTime != nil {
		query = query.Where("created_time < ?", opts.BeforeTime)
	}

	// Стратегия 3: Обычная пагинация (Offset-based) - применяется, если нет курсора
	// Если есть курсор, offset обычно не нужен (или равен 0)
	if opts.Start > 0 {
		query = query.Offset(opts.Start)
	}

	err := query.
		Order("created_time DESC").
		Limit(opts.Size).
		Find(&messages).Error

	return messages, err
}

// getMessagesAround - вспомогательный метод для получения контекста сообщения
func (s *ChatService) getMessagesAround(threadID, messageID string, limit, ndcId int) ([]chat.Message, error) {
	var targetMsg chat.Message
	if err := s.db.Where("thread_id = ? AND message_id = ?", threadID, messageID).First(&targetMsg).Error; err != nil {
		return nil, fmt.Errorf("target message not found")
	}

	halfLimit := limit / 2
	if halfLimit < 1 {
		halfLimit = 1
	}

	scope := authorScope(ndcId)
	var olderMessages []chat.Message
	var newerMessages []chat.Message

	// Получаем сообщения ДО (старые) - идем в прошлое
	if err := s.db.
		Preload("Author", scope).
		Preload("ReplyMessage").
		Preload("ReplyMessage.Author", scope).
		Where("thread_id = ? AND created_time < ?", threadID, targetMsg.CreatedTime).
		Order("created_time DESC").
		Limit(halfLimit).
		Find(&olderMessages).Error; err != nil {
		return nil, err
	}

	// Получаем сообщения ПОСЛЕ (новые) + само сообщение - идем в будущее
	// Берем limit - len(olderMessages), чтобы заполнить "страницу", если старых сообщений мало
	remainingLimit := limit - len(olderMessages)
	if err := s.db.
		Preload("Author", scope).
		Preload("ReplyMessage").
		Preload("ReplyMessage.Author", scope).
		Where("thread_id = ? AND created_time >= ?", threadID, targetMsg.CreatedTime).
		Order("created_time ASC").
		Limit(remainingLimit).
		Find(&newerMessages).Error; err != nil {
		return nil, err
	}

	// Объединяем и сортируем DESC (от новых к старым), как ожидает клиент
	var result []chat.Message

	// newerMessages сейчас ASC (10:00, 10:01, 10:02...)
	// Нам нужно DESC (10:02, 10:01, 10:00...)
	for i := len(newerMessages) - 1; i >= 0; i-- {
		result = append(result, newerMessages[i])
	}

	// olderMessages уже DESC (09:59, 09:58...)
	result = append(result, olderMessages...)

	return result, nil
}

// DeleteThread - удаление чата (soft delete)
func (s *ChatService) DeleteThread(threadID, userUID string) error {
	var thread chat.Thread
	if err := s.db.First(&thread, "thread_id = ?", threadID).Error; err != nil {
		return fmt.Errorf("thread not found")
	}

	ndcId := 0
	if thread.NdcID != nil {
		ndcId = *thread.NdcID
	}

	permSvc := NewPermissionService(s.db)
	// Требуется роль Host (20) или выше (Leader/Astranet)
	if !permSvc.CanPerform(userUID, thread.UID, ndcId, threadID, int(chat.MemberRoleHost), true) {
		return fmt.Errorf("permission denied")
	}

	return s.db.Delete(&thread).Error
}

// UpdateThread - обновление информации о чате
func (s *ChatService) UpdateThread(threadID, userUID string, updates map[string]interface{}) error {
	var thread chat.Thread
	if err := s.db.First(&thread, "thread_id = ?", threadID).Error; err != nil {
		return fmt.Errorf("thread not found")
	}

	// Проверить права
	if thread.UID != userUID {
		return fmt.Errorf("only author can update thread")
	}

	return s.db.Model(&thread).Updates(updates).Error
}

// AddMember - добавление участника в чат
func (s *ChatService) AddMember(threadID, userUID string, role chat.MemberRole) error {
	// Проверить, не является ли пользователь уже участником
	var existing chat.ThreadMember
	if err := s.db.Where("thread_id = ? AND user_uid = ?", threadID, userUID).First(&existing).Error; err == nil {
		return nil // Уже участник, ничего не делаем
	}

	// Проверить лимит участников чата
	var count int64
	s.db.Model(&chat.ThreadMember{}).Where("thread_id = ?", threadID).Count(&count)
	if count >= 1000 {
		return fmt.Errorf("thread member limit reached (1000)")
	}

	// Проверить лимит чатов для пользователя
	var userThreadCount int64
	s.db.Model(&chat.ThreadMember{}).Where("user_uid = ?", userUID).Count(&userThreadCount)
	if userThreadCount >= 1000 {
		return fmt.Errorf("user thread limit reached (1000)")
	}

	// Добавить участника
	member := &chat.ThreadMember{
		ThreadID: threadID,
		UserUID:  userUID,
		Role:     role,
	}

	if err := s.db.Create(member).Error; err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	// Обновить счетчик участников
	s.db.Exec("UPDATE threads SET members_count = members_count + 1 WHERE thread_id = ?", threadID)

	return nil
}

// RemoveMember - удаление участника из чата
func (s *ChatService) RemoveMember(threadID, userUID string) error {
	// Проверить, является ли пользователь участником
	var member chat.ThreadMember
	if err := s.db.Where("thread_id = ? AND user_uid = ?", threadID, userUID).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // Уже не участник
		}
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&member).Error; err != nil {
			return err
		}

		// Обновить счетчик участников
		if err := tx.Exec("UPDATE threads SET members_count = members_count - 1 WHERE thread_id = ? AND members_count > 0", threadID).Error; err != nil {
			return err
		}

		return nil
	})
}

// GetMembers - получение списка участников чата с пагинацией
type MemberAuthor struct {
	user.Author
	Role chat.MemberRole `json:"role"`
}

func (s *ChatService) GetMembers(threadID string, start, size, ndcId int) ([]MemberAuthor, error) {
	var members []chat.ThreadMember

	err := s.db.
		Preload("User", authorScope(ndcId)).
		Where("thread_id = ?", threadID).
		Order("role DESC, joined_time ASC").
		Limit(size).
		Offset(start).
		Find(&members).Error

	if err != nil {
		return nil, err
	}

	// Преобразуем ThreadMember в MemberAuthor
	result := make([]MemberAuthor, len(members))
	for i, m := range members {
		result[i] = MemberAuthor{
			Author: m.User,
			Role:   m.Role,
		}
	}

	return result, nil
}

// TransferOwnership - передача роли владельца другому участнику
func (s *ChatService) TransferOwnership(threadID, currentOwnerUID, newOwnerUID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Обновить роль текущего владельца на обычного участника
		if err := tx.Model(&chat.ThreadMember{}).
			Where("thread_id = ? AND user_uid = ?", threadID, currentOwnerUID).
			Update("role", chat.MemberRoleMember).Error; err != nil {
			return fmt.Errorf("failed to update current owner role: %w", err)
		}

		// Обновить роль нового владельца
		if err := tx.Model(&chat.ThreadMember{}).
			Where("thread_id = ? AND user_uid = ?", threadID, newOwnerUID).
			Update("role", chat.MemberRoleHost).Error; err != nil {
			return fmt.Errorf("failed to update new owner role: %w", err)
		}

		// Обновить UID в thread
		if err := tx.Model(&chat.Thread{}).
			Where("thread_id = ?", threadID).
			Update("uid", newOwnerUID).Error; err != nil {
			return fmt.Errorf("failed to update thread author: %w", err)
		}

		return nil
	})
}

// SetMemberRole - изменение роли участника (универсальный метод)
func (s *ChatService) SetMemberRole(threadID, userUID string, role chat.MemberRole) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Получить текущую роль участника
		var member chat.ThreadMember
		if err := tx.First(&member, "thread_id = ? AND user_uid = ?", threadID, userUID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("member not found")
			}
			return fmt.Errorf("failed to get member: %w", err)
		}

		oldRole := member.Role

		// Обновить роль в thread_members
		if err := tx.Model(&member).Update("role", role).Error; err != nil {
			return fmt.Errorf("failed to update member role: %w", err)
		}

		// Синхронизировать Extensions.CoHost
		var thread chat.Thread
		if err := tx.First(&thread, "thread_id = ?", threadID).Error; err != nil {
			return fmt.Errorf("failed to get thread: %w", err)
		}

		// Инициализировать Extensions если nil
		if thread.Extensions == nil {
			thread.Extensions = &chat.ThreadExtensions{}
		}

		coHosts := thread.Extensions.CoHost
		if coHosts == nil {
			coHosts = []string{}
		}

		// Если новая роль - CoHost, добавить в массив
		if role == chat.MemberRoleCoHost && oldRole != chat.MemberRoleCoHost {
			// Проверить что UID еще не в массиве
			found := false
			for _, uid := range coHosts {
				if uid == userUID {
					found = true
					break
				}
			}
			if !found {
				coHosts = append(coHosts, userUID)
			}
		}

		// Если убираем роль CoHost, удалить из массива
		if oldRole == chat.MemberRoleCoHost && role != chat.MemberRoleCoHost {
			newCoHosts := []string{}
			for _, uid := range coHosts {
				if uid != userUID {
					newCoHosts = append(newCoHosts, uid)
				}
			}
			coHosts = newCoHosts
		}

		thread.Extensions.CoHost = coHosts

		// Обновить Extensions в базе
		if err := tx.Model(&thread).Update("extensions", thread.Extensions).Error; err != nil {
			return fmt.Errorf("failed to update thread extensions: %w", err)
		}

		return nil
	})
}

// GetAllMemberUIDs - получение UID всех участников чата
func (s *ChatService) GetAllMemberUIDs(threadID string) ([]string, error) {
	var uids []string
	err := s.db.Model(&chat.ThreadMember{}).
		Where("thread_id = ?", threadID).
		Pluck("user_uid", &uids).Error
	return uids, err
}

// MarkMessageAsDeleted - помечает сообщение как удалённое (меняет type и content, сохраняет ID)
func (s *ChatService) MarkMessageAsDeleted(threadID, messageID, userUID, content string) (*chat.Message, error) {
	var message chat.Message
	if err := s.db.Where("thread_id = ? AND message_id = ?", threadID, messageID).First(&message).Error; err != nil {
		return nil, fmt.Errorf("message not found")
	}

	// Обновляем сообщение: меняем тип на "удалено" и очищаем контент
	message.Type = chat.MessageTypeDeleted
	message.Content = content
	message.MediaValue = nil
	message.StickerID = nil
	message.ReplyTo = nil

	if err := s.db.Save(&message).Error; err != nil {
		return nil, fmt.Errorf("failed to mark message as deleted: %w", err)
	}

	// Загружаем автора для ответа (получаем ndcId из треда)
	var thread chat.Thread
	if err := s.db.Select("ndc_id").First(&thread, "thread_id = ?", threadID).Error; err == nil {
		s.db.Preload("Author", authorScope(getNdcIdFromThread(&thread))).First(&message, message.ID)
	} else {
		s.db.Preload("Author").First(&message, message.ID)
	}

	return &message, nil
}

// CreateSystemMessage - создание системного сообщения (вступление, выход, удаление)
func (s *ChatService) CreateSystemMessage(threadID, userUID string, msgType int, content string) (*chat.Message, error) {
	message := &chat.Message{
		MessageID: generateUID(),
		ThreadID:  threadID,
		UID:       userUID,
		Content:   content,
		Type:      msgType,
	}

	if err := s.db.Create(message).Error; err != nil {
		return nil, fmt.Errorf("failed to create system message: %w", err)
	}

	// Обновить время последней активности в чате
	now := utils.CustomTime{Time: time.Now()}
	s.db.Model(&chat.Thread{}).Where("thread_id = ?", threadID).Updates(map[string]interface{}{
		"latest_activity_time": now,
		"modified_time":        now,
	})

	// Загрузить автора для ответа (получаем ndcId из треда)
	var thread chat.Thread
	if err := s.db.Select("ndc_id").First(&thread, "thread_id = ?", threadID).Error; err == nil {
		s.db.Preload("Author", authorScope(getNdcIdFromThread(&thread))).First(message, message.ID)
	} else {
		s.db.Preload("Author").First(message, message.ID)
	}

	return message, nil
}

// SendMessagePushNotifications sends push notifications to offline chat members about new message
func (s *ChatService) SendMessagePushNotifications(message *chat.Message, thread *chat.Thread, senderUID string, hub interface{}) {
	// Only send push if service is enabled
	if !IsPushEnabled() {
		return
	}

	pushSvc := GetPushService()
	if pushSvc == nil {
		return
	}

	// Get presence manager from hub to check who's online
	type HubWithPresence interface {
		GetPresenceManager() interface{}
	}

	type PresenceManager interface {
		IsUserInChat(userUID, chatID string) bool
	}

	// Get all thread members except sender
	var members []chat.ThreadMember
	if err := s.db.Where("thread_id = ? AND user_uid != ?", message.ThreadID, senderUID).Find(&members).Error; err != nil {
		return
	}

	// Get sender info for notification
	ndcId := getNdcIdFromThread(thread)
	var sender user.User
	if err := s.db.First(&sender, "uid = ? AND ndc_id = ?", senderUID, ndcId).Error; err != nil {
		return
	}

	threadTitle := "Chat"
	if thread.Title != nil && *thread.Title != "" {
		threadTitle = *thread.Title
	}

	// Send push to each offline member
	for _, member := range members {
		// Skip if member has notifications muted (AlertOption != 0 means muted)
		// AlertOption: 0 = all notifications, 1 = mentions only, 2 = none
		if member.AlertOption != 0 {
			continue
		}

		// Check if user is online and in this chat using presence manager
		isOnline := false
		if hubObj, ok := hub.(HubWithPresence); ok {
			if pm := hubObj.GetPresenceManager(); pm != nil {
				if presenceMgr, ok := pm.(PresenceManager); ok {
					isOnline = presenceMgr.IsUserInChat(member.UserUID, message.ThreadID)
				}
			}
		}

		// Only send push if user is NOT in the chat
		if !isOnline {
			// Prepare message preview (limit to 100 chars)
			messagePreview := message.Content
			if len(messagePreview) > 100 {
				messagePreview = messagePreview[:97] + "..."
			}

			pushData := pushService.NotificationData{
				Title:          sender.Nickname + " in " + threadTitle,
				Body:           messagePreview,
				NotificationID: message.MessageID,
				Type:           2, // Chat message type
				ObjectType:     2, // ObjectTypeChat
				ObjectID:       message.ThreadID,
				NdcID:          ndcId,
				ImageURL:       sender.Icon,
			}

			// Send push asynchronously
			go func(userUID string, data pushService.NotificationData) {
				if err := pushSvc.SendPushToUser(userUID, data); err != nil {
					// Silently log error
				}
			}(member.UserUID, pushData)
		}
	}
}
