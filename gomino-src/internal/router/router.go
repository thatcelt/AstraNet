package router

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	httpSwagger "github.com/swaggo/http-swagger"
	"gorm.io/gorm"

	_ "github.com/AugustLigh/GoMino/docs"
	"github.com/AugustLigh/GoMino/internal/middleware"

	adminModule "github.com/AugustLigh/GoMino/internal/api/admin"
	apikeyModule "github.com/AugustLigh/GoMino/internal/api/apikey"
	authModule "github.com/AugustLigh/GoMino/internal/api/auth"
	blogModule "github.com/AugustLigh/GoMino/internal/api/blog"
	chatModule "github.com/AugustLigh/GoMino/internal/api/chat"
	communityModule "github.com/AugustLigh/GoMino/internal/api/community"
	deviceModule "github.com/AugustLigh/GoMino/internal/api/device"
	liveModule "github.com/AugustLigh/GoMino/internal/api/live"
	mediaModule "github.com/AugustLigh/GoMino/internal/api/media"
	notificationModule "github.com/AugustLigh/GoMino/internal/api/notification"
	searchModule "github.com/AugustLigh/GoMino/internal/api/search"
	userModule "github.com/AugustLigh/GoMino/internal/api/user"
	"github.com/AugustLigh/GoMino/internal/ws"
	"github.com/AugustLigh/GoMino/pkg/config"
)

func SetLiveKitConfig(cfg config.LiveKitConfig) {
	liveModule.SetConfig(cfg)
}

func SetupRoutes(app *fiber.App, db *gorm.DB, cfg *config.Config) {
	// Initialize WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()
	chatModule.SetHub(hub)
	liveModule.SetHub(hub)
	communityModule.SetHub(hub)
	userModule.SetHub(hub)
	notificationModule.SetHub(hub)

	// Enable Swagger documentation only if configured
	if cfg.Server.EnableDocs {
		app.Get("/swagger/*", adaptor.HTTPHandler(httpSwagger.WrapHandler))
	}

	// WebSocket Route — registered BEFORE security middleware
	// (browsers cannot send custom headers on WebSocket connections,
	// the handler validates JWT token from query params internally)
	app.Get("/api/v1/chat/web-socket", ws.ServeWs(hub))

	base_api := app.Group("/api/v1")

	// Apply API key middleware first (to set bypass flag if valid key present)
	base_api.Use(middleware.APIKeyMiddleware)

	// Apply security middleware to all API routes (will be bypassed if API key is valid)
	base_api.Use(middleware.SignatureMiddleware)
	base_api.Use(middleware.DPoPMiddleware)

	global_api := base_api.Group("/g/s")

	global_api.Post("/media/upload", middleware.AuthMiddleware, mediaModule.UploadMedia)

	authRoutes := global_api.Group("/auth")
	authRoutes.Post("/login", authModule.Login)
	authRoutes.Post("/register", authModule.Register)
	authRoutes.Post("/request-security-validation", authModule.Validation)
	authRoutes.Post("/refresh", authModule.RefreshToken)
	authRoutes.Post("/reset-password", authModule.ResetPassword)

	// Account settings (authenticated)
	accountRoutes := global_api.Group("/account", middleware.AuthMiddleware)
	accountRoutes.Post("/content-region", authModule.UpdateContentRegion)
	accountRoutes.Get("/push-settings", deviceModule.GetPushSettings(db))
	accountRoutes.Post("/push-settings", deviceModule.UpdatePushSettings(db))

	// Device Token Routes (authenticated)
	global_api.Post("/device-token", middleware.AuthMiddleware, deviceModule.RegisterDeviceToken(db))
	global_api.Delete("/device-token", middleware.AuthMiddleware, deviceModule.DeleteDeviceToken(db))

	// Developer API Key routes
	developerRoutes := global_api.Group("/developer")
	developerRoutes.Post("/api-keys", middleware.AuthMiddleware, apikeyModule.CreateAPIKey)
	developerRoutes.Get("/api-keys", middleware.AuthMiddleware, apikeyModule.ListAPIKeys)
	developerRoutes.Get("/api-keys/:keyId", middleware.AuthMiddleware, apikeyModule.GetAPIKey)
	developerRoutes.Delete("/api-keys/:keyId", middleware.AuthMiddleware, apikeyModule.RevokeAPIKey)
	developerRoutes.Patch("/api-keys/:keyId", middleware.AuthMiddleware, apikeyModule.UpdateAPIKey)

	// Admin routes (Bootstrap API Key only)
	adminRoutes := global_api.Group("/admin")
	adminRoutes.Post("/set-role", adminModule.SetUserRole)
	adminRoutes.Get("/get-role", adminModule.GetUserRole)

	userRoutes := global_api.Group("/user-profile")
	// Static routes MUST be registered before parametric /:userId
	userRoutes.Get("/blocked", middleware.AuthMiddleware, userModule.GetBlockedUsers)
	// Parametric routes
	userRoutes.Get("/:userId", userModule.GetUserInfo)
	userRoutes.Post("/:userId", middleware.AuthMiddleware, userModule.UpdateUserInfo)
	userRoutes.Get("/:userId/joined", userModule.GetUserFollowing)
	userRoutes.Get("/:userId/member", userModule.GetMembers)                                 // followers list
	userRoutes.Post("/:userId/member", middleware.AuthMiddleware, userModule.FollowUser)     // follow to user
	userRoutes.Delete("/:userId/member", middleware.AuthMiddleware, userModule.UnfollowUser) // unfollow from user
	userRoutes.Get("/:userId/g-comment", userModule.GetWallComments)
	userRoutes.Post("/:userId/comment", middleware.AuthMiddleware, userModule.ReplyToComment)
	userRoutes.Post("/:userId/g-comment", middleware.AuthMiddleware, userModule.AddWallComment)
	userRoutes.Delete("/:userId/comment/:commentId", middleware.AuthMiddleware, userModule.DeleteWallComment)
	// Global ban (Astranet only)
	userRoutes.Post("/:userId/global-ban", middleware.AuthMiddleware, userModule.GlobalBanUser)
	userRoutes.Post("/:userId/global-unban", middleware.AuthMiddleware, userModule.GlobalUnbanUser)
	// User blocking (any authenticated user)
	userRoutes.Post("/:userId/block", middleware.AuthMiddleware, userModule.BlockUser)
	userRoutes.Post("/:userId/unblock", middleware.AuthMiddleware, userModule.UnblockUser)
	userRoutes.Get("/:userId/blocked-status", middleware.AuthMiddleware, userModule.IsBlocked)

	chatGroup := global_api.Group("/chat", middleware.AuthMiddleware)
	chatGroup.Post("/thread", chatModule.CreateThread)
	chatGroup.Get("/thread", chatModule.GetThreads)
	// chatGroup.Post("/thread/apply-bubble", nil) // TODO бублы

	threadRoutes := chatGroup.Group("/thread/:threadId")
	threadRoutes.Get("", chatModule.GetThreadInfo)
	threadRoutes.Post("", chatModule.UpdateThread)
	threadRoutes.Delete("", chatModule.DeleteThread)
	threadRoutes.Get("/member", chatModule.GetMembers)
	// Static routes MUST be registered before parametric /member/:userId
	threadRoutes.Post("/member/invite", chatModule.InviteToThread)
	threadRoutes.Post("/member/view-only/enable", chatModule.EnableViewOnlyMode)
	threadRoutes.Post("/member/view-only/disable", chatModule.DisableViewOnlyMode)
	threadRoutes.Post("/member/members-can-invite/enable", chatModule.EnableCanInvite)
	threadRoutes.Post("/member/members-can-invite/disable", chatModule.DisableCanInvite)
	threadRoutes.Post("/member/transfer-organizer", chatModule.TransferOrganizer)
	// Parametric routes after static ones
	threadRoutes.Post("/member/:userId", chatModule.JoinThread)
	threadRoutes.Delete("/member/:userId", chatModule.LeaveThread)
	threadRoutes.Post("/member/:userId/alert", chatModule.DoNotDisturb)
	threadRoutes.Post("/member/:userId/background", chatModule.SetBackgroundImage)
	threadRoutes.Post("/member/:userId/co-host", chatModule.SetCoHost)
	threadRoutes.Delete("/member/:userId/co-host", chatModule.RemoveCoHost)

	threadRoutes.Post("/mark-as-read", chatModule.MarkThreadAsRead)
	// threadRoutes.Post("/vvchat-presenter/invite/", nil) // TODO когда паявятся гч
	// threadRoutes.Post("/vvchat-permission", nil)
	// threadRoutes.Get("/avchat-reputation", nil)
	// threadRoutes.Post("/avchat-reputation", nil)

	// threadRoutes.Post("/admin", nil) // TODO когда будут сообщества

	messageRoutes := threadRoutes.Group("/message")
	messageRoutes.Post("", chatModule.SendMessage)
	messageRoutes.Get("", chatModule.GetMessages)
	messageRoutes.Post("/:messageId", chatModule.EditMessage)
	messageRoutes.Delete("/:messageId", chatModule.DeleteMessage)
	// messageRoutes.Delete("/:messageId/admin", chatModule.DeleteMessage) // TODO когда будут сообщества

	// Live Room Routes
	threadRoutes.Post("/live-room", liveModule.StartRoom)
	threadRoutes.Get("/live-room", liveModule.GetActiveRoom)
	threadRoutes.Post("/live-room/:roomId/join", liveModule.JoinRoom)
	threadRoutes.Post("/live-room/:roomId/leave", liveModule.LeaveRoom)
	threadRoutes.Post("/live-room/:roomId/end", liveModule.EndRoom)
	threadRoutes.Post("/live-room/:roomId/lock", liveModule.LockRoom)

	global_api.Post("/community", middleware.AuthMiddleware, communityModule.CreateCommunity)
	global_api.Get("/community/joined", middleware.AuthMiddleware, communityModule.GetJoinedCommunities) // get com list

	// Featured Communities Routes
	global_api.Get("/community/featured", middleware.OptionalAuthMiddleware, communityModule.GetFeaturedCommunities) // get featured for segment
	global_api.Post("/community/featured", middleware.AuthMiddleware, communityModule.SetFeaturedCommunities) // set featured (Astranet only)
	global_api.Post("/community/featured/add", middleware.AuthMiddleware, communityModule.AddFeaturedCommunity) // add single (Astranet only)
	global_api.Delete("/community/featured/:ndcId", middleware.AuthMiddleware, communityModule.RemoveFeaturedCommunity) // remove (Astranet only)
	global_api.Get("/community/featured/segments", communityModule.GetAvailableSegments)                     // get available segments
	global_api.Post("/community/featured/by-ids", middleware.OptionalAuthMiddleware, communityModule.GetFeaturedCommunitiesByIds) // get by IDs list

	// Banners Routes
	global_api.Get("/community/banners", communityModule.GetBanners)                                          // get banners for segment
	global_api.Post("/community/banners", middleware.AuthMiddleware, communityModule.SetBanners)               // set banners (Astranet only)

	// Global Search
	global_api.Get("/search", middleware.AuthMiddleware, searchModule.GlobalSearch)

	global_api.Post("/blog", middleware.AuthMiddleware, blogModule.CreateGlobalBlog)

	// Global Notification Routes
	notificationRoutes := global_api.Group("/notification", middleware.AuthMiddleware)
	notificationRoutes.Get("", notificationModule.GetNotifications)
	notificationRoutes.Get("/count", notificationModule.GetUnreadCount)
	notificationRoutes.Post("/:id/read", notificationModule.MarkAsRead)
	notificationRoutes.Post("/checked", notificationModule.MarkAllAsRead)
	notificationRoutes.Delete("/:id", notificationModule.DeleteNotification)
	notificationRoutes.Delete("", notificationModule.DeleteAllNotifications)

	sx_api := base_api.Group("/g/s-x:comId")
	sx_api.Get("/community", middleware.OptionalAuthMiddleware, communityModule.GetCommunity) // get com info

	communityGroup := base_api.Group("/x:comId/s")
	communityGroup.Post("/community/join", middleware.AuthMiddleware, communityModule.JoinCommunity)
	communityGroup.Post("/community/leave", middleware.AuthMiddleware, communityModule.LeaveCommunity)
	communityGroup.Post("/community/settings", middleware.AuthMiddleware, communityModule.UpdateCommunitySettings)
	communityGroup.Get("/community/online-activity", middleware.AuthMiddleware, communityModule.GetOnlineActivity)
	communityGroup.Get("/community/member", middleware.AuthMiddleware, communityModule.GetCommunityMembers)

	// Community Search
	communityGroup.Get("/search", middleware.AuthMiddleware, searchModule.CommunitySearch)

	communityGroup.Get("/feed/blog-all", middleware.OptionalAuthMiddleware, blogModule.GetCommunityBlogFeed)
	communityGroup.Get("/feed/featured", middleware.OptionalAuthMiddleware, blogModule.GetFeaturedBlogs)
	communityGroup.Post("/blog", middleware.AuthMiddleware, blogModule.CreateCommunityBlog)
	communityGroup.Get("/blog", middleware.OptionalAuthMiddleware, blogModule.GetCommunityUserBlogs) // user blogs f"/x{self.comId}/s/blog?type=user&q={userId}&start={start}&size={size}"
	communityGroup.Delete("/blog/:blogId", middleware.AuthMiddleware, blogModule.DeleteCommunityBlog)
	communityGroup.Post("/blog/:blogId", middleware.AuthMiddleware, blogModule.UpdateCommunityBlog)        //edit blog
	communityGroup.Post("/blog/:blogId/g-vote", middleware.AuthMiddleware, blogModule.VoteBlog) // like blog
	communityGroup.Post("/blog/:blogId/feature", middleware.AuthMiddleware, blogModule.FeatureBlog)
	communityGroup.Delete("/blog/:blogId/feature", middleware.AuthMiddleware, blogModule.UnfeatureBlog)
	// Blog moderation (Curator+)
	communityGroup.Post("/blog/:blogId/hide", middleware.AuthMiddleware, blogModule.HideBlog)
	communityGroup.Post("/blog/:blogId/unhide", middleware.AuthMiddleware, blogModule.UnhideBlog)

	communityGroup.Get("/blog/:blogId", middleware.OptionalAuthMiddleware, blogModule.GetSingleBlog)

	// Blog Comments
	communityGroup.Get("/blog/:blogId/comment", blogModule.GetBlogComments)
	communityGroup.Post("/blog/:blogId/comment", middleware.AuthMiddleware, blogModule.PostBlogComment)
	communityGroup.Get("/blog/:blogId/comment/:commentId/reply", blogModule.GetCommentReplies)
	communityGroup.Delete("/blog/:blogId/comment/:commentId", middleware.AuthMiddleware, blogModule.DeleteBlogComment)

	communityGroup.Get("/live-layer", NotImplemented)

	// Community Chat Routes
	comChatGroup := communityGroup.Group("/chat", middleware.AuthMiddleware)
	comChatGroup.Post("/thread", chatModule.CreateThread)
	comChatGroup.Get("/thread", chatModule.GetThreads)

	comThreadRoutes := comChatGroup.Group("/thread/:threadId")
	comThreadRoutes.Get("", chatModule.GetThreadInfo)
	comThreadRoutes.Post("", chatModule.UpdateThread)
	comThreadRoutes.Delete("", chatModule.DeleteThread)
	comThreadRoutes.Get("/member", chatModule.GetMembers)
	// Static routes MUST be registered before parametric /member/:userId
	comThreadRoutes.Post("/member/invite", chatModule.InviteToThread)
	comThreadRoutes.Post("/member/view-only/enable", chatModule.EnableViewOnlyMode)
	comThreadRoutes.Post("/member/view-only/disable", chatModule.DisableViewOnlyMode)
	comThreadRoutes.Post("/member/members-can-invite/enable", chatModule.EnableCanInvite)
	comThreadRoutes.Post("/member/members-can-invite/disable", chatModule.DisableCanInvite)
	comThreadRoutes.Post("/member/transfer-organizer", chatModule.TransferOrganizer)
	// Parametric routes after static ones
	comThreadRoutes.Post("/member/:userId", chatModule.JoinThread)
	comThreadRoutes.Delete("/member/:userId", chatModule.LeaveThread)
	comThreadRoutes.Post("/member/:userId/alert", chatModule.DoNotDisturb)
	comThreadRoutes.Post("/member/:userId/background", chatModule.SetBackgroundImage)
	comThreadRoutes.Post("/member/:userId/co-host", chatModule.SetCoHost)
	comThreadRoutes.Delete("/member/:userId/co-host", chatModule.RemoveCoHost)
	comThreadRoutes.Post("/mark-as-read", chatModule.MarkThreadAsRead)
	// Chat moderation (Curator+)
	comThreadRoutes.Post("/hide", chatModule.HideThread)
	comThreadRoutes.Post("/unhide", chatModule.UnhideThread)

	comMessageRoutes := comThreadRoutes.Group("/message")
	comMessageRoutes.Post("", chatModule.SendMessage)
	comMessageRoutes.Get("", chatModule.GetMessages)
	comMessageRoutes.Post("/:messageId", chatModule.EditMessage)
	comMessageRoutes.Delete("/:messageId", chatModule.DeleteMessage)

	// Community Live Room Routes
	comThreadRoutes.Post("/live-room", liveModule.StartRoom)
	comThreadRoutes.Get("/live-room", liveModule.GetActiveRoom)
	comThreadRoutes.Post("/live-room/:roomId/join", liveModule.JoinRoom)
	comThreadRoutes.Post("/live-room/:roomId/leave", liveModule.LeaveRoom)
	comThreadRoutes.Post("/live-room/:roomId/end", liveModule.EndRoom)
	comThreadRoutes.Post("/live-room/:roomId/lock", liveModule.LockRoom)

	communityUser := communityGroup.Group("/user-profile")
	// Static routes before parametric /:userId
	communityUser.Get("/blocked", middleware.AuthMiddleware, userModule.GetBlockedUsers)
	communityUser.Post("/:userId/transfer-agent", middleware.AuthMiddleware, communityModule.TransferAgent)
	communityUser.Post("/:userId/leader", middleware.AuthMiddleware, communityModule.PromoteToLeader)
	communityUser.Post("/:userId/curator", middleware.AuthMiddleware, communityModule.PromoteToCurator)
	communityUser.Post("/:userId/ban", middleware.AuthMiddleware, communityModule.BanUser)
	communityUser.Post("/:userId/unban", middleware.AuthMiddleware, communityModule.UnbanUser)
	communityUser.Get("/:userId", userModule.GetUserInfo)
	communityUser.Post("/:userId", middleware.AuthMiddleware, userModule.UpdateUserInfo)
	communityUser.Get("/:userId/joined", userModule.GetUserFollowing)
	communityUser.Get("/:userId/member", userModule.GetMembers)                                 // followers list
	communityUser.Post("/:userId/member", middleware.AuthMiddleware, userModule.FollowUser)     // follow to user
	communityUser.Delete("/:userId/member", middleware.AuthMiddleware, userModule.UnfollowUser) // unfollow from user
	communityUser.Get("/:userId/g-comment", userModule.GetWallComments)
	communityUser.Post("/:userId/comment", middleware.AuthMiddleware, userModule.ReplyToComment)
	communityUser.Post("/:userId/g-comment", middleware.AuthMiddleware, userModule.AddWallComment)
	communityUser.Delete("/:userId/comment/:commentId", middleware.AuthMiddleware, userModule.DeleteWallComment)
	// User blocking (same global handlers, accessible from community context)
	communityUser.Post("/:userId/block", middleware.AuthMiddleware, userModule.BlockUser)
	communityUser.Post("/:userId/unblock", middleware.AuthMiddleware, userModule.UnblockUser)
	communityUser.Get("/:userId/blocked-status", middleware.AuthMiddleware, userModule.IsBlocked)

	// Community Notification Routes
	comNotificationRoutes := communityGroup.Group("/notification", middleware.AuthMiddleware)
	comNotificationRoutes.Get("", notificationModule.GetNotifications)
	comNotificationRoutes.Get("/count", notificationModule.GetUnreadCount)
	comNotificationRoutes.Post("/:id/read", notificationModule.MarkAsRead)
	comNotificationRoutes.Post("/checked", notificationModule.MarkAllAsRead)
	comNotificationRoutes.Delete("/:id", notificationModule.DeleteNotification)
	comNotificationRoutes.Delete("", notificationModule.DeleteAllNotifications)
	
	communityGroup.Get("/admin/operation", middleware.AuthMiddleware, communityModule.GetModerationHistory)
}

func NotImplemented(c fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"message": "Not implemented yet",
	})
}
