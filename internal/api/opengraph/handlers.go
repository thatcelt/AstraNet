package opengraph

import (
	"fmt"
	"html"
	"strings"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/AugustLigh/GoMino/internal/models/blog"
	"github.com/AugustLigh/GoMino/internal/models/chat"
	"github.com/AugustLigh/GoMino/internal/models/community"
	"github.com/AugustLigh/GoMino/internal/models/user"
)

var db *gorm.DB

const (
	siteURL      = "https://testsite.astranetapp.com"
	siteName     = "Astranet"
	defaultIcon  = "https://media.astranetapp.com/default_icon.png"
	mediaBaseURL = "https://media.astranetapp.com/"
)

func SetDB(database *gorm.DB) {
	db = database
}

// normalizeImageURL converts local media URLs to public URLs
func normalizeImageURL(imageURL string) string {
	if imageURL == "" {
		return ""
	}
	// If it's already a public media URL, return as is
	if strings.HasPrefix(imageURL, mediaBaseURL) {
		return imageURL
	}
	// If it's a local URL (contains port like :8081), extract filename and use public URL
	if strings.Contains(imageURL, ":8081/") {
		parts := strings.Split(imageURL, ":8081/")
		if len(parts) > 1 {
			return mediaBaseURL + parts[1]
		}
	}
	// If it's just a filename, prepend the media base URL
	if !strings.HasPrefix(imageURL, "http") {
		return mediaBaseURL + imageURL
	}
	return imageURL
}

// IsBotUserAgent checks if the User-Agent belongs to a social media bot
func IsBotUserAgent(ua string) bool {
	ua = strings.ToLower(ua)
	bots := []string{
		"telegrambot",
		"twitterbot",
		"facebookexternalhit",
		"linkedinbot",
		"whatsapp",
		"slackbot",
		"discordbot",
		"vkshare",
		"skypeuripreview",
	}
	for _, bot := range bots {
		if strings.Contains(ua, bot) {
			return true
		}
	}
	return false
}

// RenderOGPage renders HTML with Open Graph meta tags
func RenderOGPage(title, description, image, url string) string {
	image = normalizeImageURL(image)
	if image == "" {
		image = defaultIcon
	}
	if title == "" {
		title = siteName
	}
	if description == "" {
		description = "Join us on " + siteName
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta property="og:title" content="%s">
    <meta property="og:description" content="%s">
    <meta property="og:image" content="%s">
    <meta property="og:url" content="%s">
    <meta property="og:type" content="website">
    <meta property="og:site_name" content="%s">
    <meta name="twitter:card" content="summary_large_image">
    <meta name="twitter:title" content="%s">
    <meta name="twitter:description" content="%s">
    <meta name="twitter:image" content="%s">
    <title>%s</title>
    <meta http-equiv="refresh" content="0;url=%s">
</head>
<body>
    <p>Redirecting to <a href="%s">%s</a>...</p>
</body>
</html>`,
		html.EscapeString(title),
		html.EscapeString(description),
		html.EscapeString(image),
		html.EscapeString(url),
		siteName,
		html.EscapeString(title),
		html.EscapeString(description),
		html.EscapeString(image),
		html.EscapeString(title),
		html.EscapeString(url),
		html.EscapeString(url),
		html.EscapeString(siteName),
	)
}

// HandleCommunity handles /og/c/:comId
func HandleCommunity(c fiber.Ctx) error {
	comId := c.Params("comId")
	url := fmt.Sprintf("%s/c/%s", siteURL, comId)

	// Try to get community info
	var com community.Community
	if err := db.Where("ndc_id = ?", comId).First(&com).Error; err == nil {
		tagline := ""
		if com.Tagline != nil {
			tagline = *com.Tagline
		}
		htmlContent := RenderOGPage(
			com.Name,
			tagline,
			com.Icon,
			url,
		)
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(htmlContent)
	}

	// Fallback
	htmlContent := RenderOGPage("Community", "Join this community on Astranet", "", url)
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(htmlContent)
}

// HandleChat handles /og/c/:comId/t/:threadId and /og/t/:threadId
func HandleChat(c fiber.Ctx) error {
	comId := c.Params("comId")
	threadId := c.Params("threadId")

	var url string
	if comId != "" {
		url = fmt.Sprintf("%s/c/%s/t/%s", siteURL, comId, threadId)
	} else {
		url = fmt.Sprintf("%s/t/%s", siteURL, threadId)
	}

	// Try to get chat info
	var thread chat.Thread
	if err := db.Where("thread_id = ?", threadId).First(&thread).Error; err == nil {
		title := "Chat"
		if thread.Title != nil {
			title = *thread.Title
		}
		icon := ""
		if thread.Icon != nil {
			icon = *thread.Icon
		}
		htmlContent := RenderOGPage(
			title,
			"Join this chat on Astranet",
			icon,
			url,
		)
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(htmlContent)
	}

	// Fallback
	htmlContent := RenderOGPage("Chat", "Join this chat on Astranet", "", url)
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(htmlContent)
}

// HandlePost handles /og/c/:comId/p/:postId
func HandlePost(c fiber.Ctx) error {
	comId := c.Params("comId")
	postId := c.Params("postId")
	url := fmt.Sprintf("%s/c/%s/p/%s", siteURL, comId, postId)

	// Try to get blog info
	var b blog.Blog
	if err := db.Where("blog_id = ?", postId).First(&b).Error; err == nil {
		// Get first image from media list if available
		var image string
		if b.MediaList != nil && len(*b.MediaList) > 0 {
			mediaItem := (*b.MediaList)[0]
			if mediaItem.URL != "" {
				image = mediaItem.URL
			}
		}

		description := b.Content
		if len(description) > 200 {
			description = description[:200] + "..."
		}

		htmlContent := RenderOGPage(
			b.Title,
			description,
			image,
			url,
		)
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(htmlContent)
	}

	// Fallback
	htmlContent := RenderOGPage("Post", "View this post on Astranet", "", url)
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(htmlContent)
}

// HandleUser handles /og/u/:userId
func HandleUser(c fiber.Ctx) error {
	userId := c.Params("userId")
	url := fmt.Sprintf("%s/u/%s", siteURL, userId)

	// Try to get user info
	var u user.User
	if err := db.Where("uid = ?", userId).First(&u).Error; err == nil {
		description := "View profile on Astranet"
		if u.Content != nil && *u.Content != "" {
			description = *u.Content
			if len(description) > 200 {
				description = description[:200] + "..."
			}
		}

		htmlContent := RenderOGPage(
			u.Nickname,
			description,
			u.Icon,
			url,
		)
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(htmlContent)
	}

	// Fallback
	htmlContent := RenderOGPage("User Profile", "View this profile on Astranet", "", url)
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(htmlContent)
}
