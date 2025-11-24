package api

import (
	"github.com/gin-gonic/gin"
)

// Config holds application configuration
type Config struct {
	Port        string
	MaxFileSize int64
	TempDir     string
}

func SetupRoutes(r *gin.Engine, config *Config) {
	apiGroup := r.Group("/api/pdf")
	{
		apiGroup.POST("/upload", func(c *gin.Context) { HandleUpload(c, config) })
		apiGroup.POST("/resave", func(c *gin.Context) { HandleResave(c, config) })
		apiGroup.POST("/remove-pages", func(c *gin.Context) { HandleRemovePages(c, config) })
		apiGroup.POST("/remove-elements", func(c *gin.Context) { HandleRemoveElements(c, config) })
		apiGroup.POST("/analyze-unwanted-elements", func(c *gin.Context) { HandleAnalyzeUnwantedElements(c, config) })
		apiGroup.GET("/preview-image", func(c *gin.Context) { HandlePreviewImage(c, config) })
		apiGroup.POST("/remove-selected-elements", func(c *gin.Context) { HandleRemoveSelectedElements(c, config) })
	}

	// Unwanted elements management page
	r.GET("/unwanted-elements", func(c *gin.Context) {
		c.HTML(200, "unwanted-elements.html", gin.H{
			"title": "Unwanted Elements Management - PDF Editor",
		})
	})
}
