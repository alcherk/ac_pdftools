package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"pdf_editor/api"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// DefaultMaxFileSize is the default maximum file size (10MB)
	DefaultMaxFileSize = 10 * 1024 * 1024
	
	// DefaultPort is the default server port
	DefaultPort = "8080"
	
	// DefaultTempDir is the default temporary directory
	DefaultTempDir = "./temp"
	
	// ServerReadTimeout is the HTTP server read timeout
	ServerReadTimeout = 15 * time.Second
	
	// ServerWriteTimeout is the HTTP server write timeout
	ServerWriteTimeout = 15 * time.Second
	
	// ServerIdleTimeout is the HTTP server idle timeout
	ServerIdleTimeout = 60 * time.Second
	
	// GracefulShutdownTimeout is the timeout for graceful shutdown
	GracefulShutdownTimeout = 10 * time.Second
)

func main() {
	// Load configuration
	config := &api.Config{
		Port:        getEnv("PORT", DefaultPort),
		MaxFileSize: getEnvInt64("MAX_FILE_SIZE", DefaultMaxFileSize),
		TempDir:     getEnv("TEMP_DIR", DefaultTempDir),
	}

	// Check pdfcpu availability on startup
	if err := checkPdfCpuAvailable(); err != nil {
		log.Fatalf("pdfcpu CLI not available: %v. Please install pdfcpu to continue.", err)
	}
	log.Println("pdfcpu CLI is available")

	r := gin.Default()

	// Static files for web UI
	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	// API routes with config
	api.SetupRoutes(r, config)

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"service": "pdf_editor",
		})
	})

	// Web UI route
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "PDF Editor",
		})
	})

	// Create HTTP server with timeout settings
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", config.Port),
		Handler:      r,
		ReadTimeout:  ServerReadTimeout,
		WriteTimeout: ServerWriteTimeout,
		IdleTimeout:  ServerIdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on %s", srv.Addr)
	log.Printf("Max file size: %d bytes", config.MaxFileSize)
	log.Printf("Temp directory: %s", config.TempDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), GracefulShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// checkPdfCpuAvailable verifies that pdfcpu CLI is available in PATH
func checkPdfCpuAvailable() error {
	cmd := exec.Command("pdfcpu", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pdfcpu command not found or not executable: %v", err)
	}
	return nil
}
