package main

import (
	"log"
	"os"

	"oauth-client/database"
	"oauth-client/handlers"
	"oauth-client/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	db, err := database.Init(getEnv("DB_PATH", "./data/oauth.db"))
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := database.Seed(db); err != nil {
		log.Printf("Warning: seed failed: %v", err)
	}

	h := handlers.New(db)

	if getEnv("GIN_MODE", "release") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// Public API
	api := r.Group("/api")
	{
		api.GET("/providers", h.ListProviders)
		api.GET("/auth/:provider", h.StartOAuth)
		api.GET("/auth/:provider/callback", h.OAuthCallback)
		api.POST("/auth/login", h.Login)
		api.POST("/auth/register", h.Register)

		// Authenticated user routes
		auth := api.Group("/", middleware.AuthRequired(getEnv("JWT_SECRET", "change-me-in-production")))
		{
			auth.GET("/me", h.GetMe)
			auth.GET("/accounts", h.ListLinkedAccounts)
			auth.POST("/auth/:provider/link", h.StartOAuthLink)
			auth.POST("/accounts/:id/refresh", h.RefreshAccountToken)
			auth.DELETE("/accounts/:id", h.UnlinkAccount)
			auth.POST("/logout", h.Logout)
		}

		// Admin routes
		admin := api.Group("/admin", middleware.AuthRequired(getEnv("JWT_SECRET", "change-me-in-production")), middleware.AdminRequired())
		{
			admin.GET("/providers", h.AdminListProviders)
			admin.POST("/providers", h.AdminCreateProvider)
			admin.PUT("/providers/:id", h.AdminUpdateProvider)
			admin.DELETE("/providers/:id", h.AdminDeleteProvider)
			admin.GET("/users", h.AdminListUsers)
			admin.PUT("/users/:id/role", h.AdminUpdateUserRole)
			admin.GET("/stats", h.AdminGetStats)
		}
	}

	// Serve frontend static files
	staticDir := getEnv("STATIC_DIR", "./static")
	if _, err := os.Stat(staticDir); err == nil {
		r.Static("/assets", staticDir+"/assets")
		r.NoRoute(func(c *gin.Context) {
			c.File(staticDir + "/index.html")
		})
	}

	port := getEnv("PORT", "8080")
	log.Printf("Server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
