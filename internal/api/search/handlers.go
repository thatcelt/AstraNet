package search

import (
	"strconv"

	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/gofiber/fiber/v3"
)

// GlobalSearch godoc
// @Summary Global search across communities, users, and chats
// @Description Search for communities, users, or chats globally
// @Tags search
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param type query string false "Search type: all, community, user, chat" default(all)
// @Param start query int false "Pagination offset" default(0)
// @Param size query int false "Page size (max 100)" default(25)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/search [get]
const MaxSearchQueryLength = 200

func GlobalSearch(c fiber.Ctx) error {
	query := c.Query("q")
	if query == "" || len(query) > MaxSearchQueryLength {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	searchType := c.Query("type", "all")
	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))
	if size > 100 {
		size = 100
	}
	if size < 1 {
		size = 25
	}

	uid := middleware.GetAUIDFromContext(c)
	db := middleware.GetDBFromContext(c)
	svc := service.NewSearchService(db)

	result, err := svc.GlobalSearch(query, searchType, uid, start, size)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(result)
}

// CommunitySearch godoc
// @Summary Search within a specific community
// @Description Search for chats, blogs, or users within a community
// @Tags search
// @Accept json
// @Produce json
// @Param comId path string true "Community ID"
// @Param q query string true "Search query"
// @Param type query string false "Search type: all, chat, blog, user" default(all)
// @Param start query int false "Pagination offset" default(0)
// @Param size query int false "Page size (max 100)" default(25)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /x{comId}/s/search [get]
func CommunitySearch(c fiber.Ctx) error {
	comIdStr := c.Params("comId")
	ndcId, err := strconv.Atoi(comIdStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	query := c.Query("q")
	if query == "" || len(query) > MaxSearchQueryLength {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	searchType := c.Query("type", "all")
	start, _ := strconv.Atoi(c.Query("start", "0"))
	size, _ := strconv.Atoi(c.Query("size", "25"))
	if size > 100 {
		size = 100
	}
	if size < 1 {
		size = 25
	}

	uid := middleware.GetAUIDFromContext(c)
	db := middleware.GetDBFromContext(c)
	svc := service.NewSearchService(db)

	result, err := svc.CommunitySearch(ndcId, query, searchType, uid, start, size)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(result)
}
