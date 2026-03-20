package service

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/AugustLigh/GoMino/internal/models/blog"
	"github.com/AugustLigh/GoMino/internal/models/chat"
	"github.com/AugustLigh/GoMino/internal/models/community"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"gorm.io/gorm"
)

// capitalizeFirst returns string with first rune uppercased
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// caseVariants generates case variants for a term to handle Unicode in SQLite
// SQLite LIKE is only case-insensitive for ASCII, so we generate multiple patterns
func caseVariants(term string) []string {
	lower := strings.ToLower(term)
	upper := strings.ToUpper(term)
	titled := capitalizeFirst(lower)

	seen := map[string]bool{}
	var variants []string
	for _, v := range []string{lower, upper, titled, term} {
		if !seen[v] {
			seen[v] = true
			variants = append(variants, v)
		}
	}
	return variants
}

// buildFuzzyPatterns создаёт паттерны для нечёткого поиска с вариантами регистра
func buildFuzzyPatterns(query string) []string {
	query = strings.TrimSpace(query)
	seen := map[string]bool{}
	var patterns []string

	// Full query variants
	for _, v := range caseVariants(query) {
		p := "%" + v + "%"
		if !seen[p] {
			seen[p] = true
			patterns = append(patterns, p)
		}
	}

	// Per-word variants
	words := strings.Fields(query)
	if len(words) > 1 {
		for _, word := range words {
			if utf8.RuneCountInString(word) >= 2 {
				for _, v := range caseVariants(word) {
					p := "%" + v + "%"
					if !seen[p] {
						seen[p] = true
						patterns = append(patterns, p)
					}
				}
			}
		}
	}
	return patterns
}

type SearchService struct {
	db *gorm.DB
}

func NewSearchService(db *gorm.DB) *SearchService {
	return &SearchService{db: db}
}

// SearchResult contains results for different entity types
type SearchResult struct {
	CommunityList   []community.Community `json:"communityList"`
	UserProfileList []user.User           `json:"userProfileList"`
	ThreadList      []chat.Thread         `json:"threadList"`
	BlogList        []blog.Blog           `json:"blogList"`
}

// GlobalSearch performs search across all global entities
func (s *SearchService) GlobalSearch(query, searchType, uid string, start, size int) (*SearchResult, error) {
	result := &SearchResult{
		CommunityList:   []community.Community{},
		UserProfileList: []user.User{},
		ThreadList:      []chat.Thread{},
		BlogList:        []blog.Blog{},
	}
	patterns := buildFuzzyPatterns(query)

	switch searchType {
	case "community":
		s.searchCommunities(patterns, start, size, result)
	case "user":
		s.searchGlobalUsers(patterns, start, size, result)
	case "chat":
		s.searchGlobalThreads(patterns, start, size, result)
	case "all":
		s.searchCommunities(patterns, 0, 10, result)
		s.searchGlobalUsers(patterns, 0, 10, result)
		s.searchGlobalThreads(patterns, 0, 10, result)
	}

	return result, nil
}

// CommunitySearch performs search within a specific community
func (s *SearchService) CommunitySearch(ndcId int, query, searchType, uid string, start, size int) (*SearchResult, error) {
	result := &SearchResult{
		CommunityList:   []community.Community{},
		UserProfileList: []user.User{},
		ThreadList:      []chat.Thread{},
		BlogList:        []blog.Blog{},
	}
	patterns := buildFuzzyPatterns(query)

	switch searchType {
	case "chat":
		s.searchCommunityThreads(patterns, ndcId, start, size, result)
	case "blog":
		s.searchBlogs(patterns, ndcId, start, size, result)
	case "user":
		s.searchCommunityUsers(patterns, ndcId, start, size, result)
	case "all":
		s.searchCommunityThreads(patterns, ndcId, 0, 10, result)
		s.searchBlogs(patterns, ndcId, 0, 10, result)
		s.searchCommunityUsers(patterns, ndcId, 0, 10, result)
	}

	return result, nil
}

// buildOrCondition строит условие OR для нескольких паттернов
func (s *SearchService) buildOrCondition(patterns []string, fields ...string) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	for _, pattern := range patterns {
		for _, field := range fields {
			conditions = append(conditions, field+" LIKE ?")
			args = append(args, pattern)
		}
	}

	return "(" + strings.Join(conditions, " OR ") + ")", args
}

// searchCommunities searches for communities by name and keywords
func (s *SearchService) searchCommunities(patterns []string, start, size int, result *SearchResult) {
	var communities []community.Community
	cond, args := s.buildOrCondition(patterns, "name", "keywords", "tagline")
	args = append([]interface{}{true}, args...)

	s.db.Where("searchable = ? AND "+cond, args...).
		Order("members_count DESC").
		Offset(start).Limit(size).
		Find(&communities)
	result.CommunityList = communities
}

// searchGlobalUsers searches for global user profiles by nickname
func (s *SearchService) searchGlobalUsers(patterns []string, start, size int, result *SearchResult) {
	var users []user.User
	cond, args := s.buildOrCondition(patterns, "nickname")
	args = append(args, 0)

	s.db.Preload("AvatarFrame").Preload("CustomTitles").
		Where(cond+" AND ndc_id = ? AND status >= 0", args...).
		Order("reputation DESC, level DESC").
		Offset(start).Limit(size).
		Find(&users)
	result.UserProfileList = users
}

// searchCommunityUsers searches for users within a specific community
func (s *SearchService) searchCommunityUsers(patterns []string, ndcId, start, size int, result *SearchResult) {
	var users []user.User
	cond, args := s.buildOrCondition(patterns, "nickname")
	args = append(args, ndcId)

	s.db.Preload("AvatarFrame").Preload("CustomTitles").
		Where(cond+" AND ndc_id = ? AND status >= 0", args...).
		Order("reputation DESC, level DESC").
		Offset(start).Limit(size).
		Find(&users)
	result.UserProfileList = users
}

// searchGlobalThreads searches for public threads globally (ndc_id = 0)
func (s *SearchService) searchGlobalThreads(patterns []string, start, size int, result *SearchResult) {
	var threads []chat.Thread
	cond, args := s.buildOrCondition(patterns, "title", "content")
	args = append(args, 0)

	s.db.Preload("Author").
		Where(cond+" AND type = ? AND (ndc_id = 0 OR ndc_id IS NULL)", args...).
		Order("members_count DESC, latest_activity_time DESC").
		Offset(start).Limit(size).
		Find(&threads)
	result.ThreadList = threads
}

// searchCommunityThreads searches for public threads in a community
func (s *SearchService) searchCommunityThreads(patterns []string, ndcId, start, size int, result *SearchResult) {
	var threads []chat.Thread
	cond, args := s.buildOrCondition(patterns, "title", "content")
	args = append(args, ndcId, 0)

	s.db.Preload("Author").
		Where(cond+" AND ndc_id = ? AND type = ?", args...).
		Order("members_count DESC, latest_activity_time DESC").
		Offset(start).Limit(size).
		Find(&threads)
	result.ThreadList = threads
}

// searchBlogs searches for blog posts in a community
func (s *SearchService) searchBlogs(patterns []string, ndcId, start, size int, result *SearchResult) {
	var blogs []blog.Blog
	cond, args := s.buildOrCondition(patterns, "title", "content")
	args = append(args, ndcId, 0)

	s.db.Preload("Author").
		Where(cond+" AND ndc_id = ? AND status = ?", args...).
		Order("votes_count DESC, created_time DESC").
		Offset(start).Limit(size).
		Find(&blogs)
	result.BlogList = blogs
}
