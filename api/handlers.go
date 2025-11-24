package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	pdfPkg "pdf_editor/pdf"

	"github.com/gin-gonic/gin"
)

func HandleUpload(c *gin.Context, config *Config) {
	file, header, err := c.Request.FormFile("pdf")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	// Validate PDF file
	if err := validatePDFFile(file, header, config.MaxFileSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save the uploaded file temporarily
	if err := ensureTempDir(config.TempDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp directory"})
		return
	}

	// Sanitize filename to prevent path traversal
	safeFilename := sanitizeFilename(header.Filename)
	uniqueID := generateUniqueID()
	filename := filepath.Join(config.TempDir, uniqueID+"_"+safeFilename)

	out, err := os.Create(filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer out.Close()

	_, err = out.ReadFrom(file)
	if err != nil {
		os.Remove(filename) // Clean up on error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"filename": header.Filename, "path": filename})
}

func HandleResave(c *gin.Context, config *Config) {
	handlePDFFile(c, config, pdfPkg.ResavePDF, "resaved")
}

func HandleRemovePages(c *gin.Context, config *Config) {
	pagesParam := c.PostForm("pages")
	if pagesParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No pages specified"})
		return
	}

	handlePDFFile(c, config, func(inFile, outFile string) error {
		return pdfPkg.RemovePagesFromPDF(inFile, outFile, pagesParam)
	}, "pages_removed")
}

func HandleRemoveElements(c *gin.Context, config *Config) {
	elementType := c.PostForm("type")
	handlePDFFile(c, config, func(inFile, outFile string) error {
		return pdfPkg.RemoveElementFromPDF(inFile, outFile, elementType)
	}, "elements_removed")
}

func HandleAnalyzeUnwantedElements(c *gin.Context, config *Config) {
	file, header, err := c.Request.FormFile("pdf")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No PDF file provided"})
		return
	}
	defer file.Close()

	// Validate PDF file
	if err := validatePDFFile(file, header, config.MaxFileSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create temp input file
	if err := ensureTempDir(config.TempDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp directory"})
		return
	}

	uniqueID := generateUniqueID()
	inFile := filepath.Join(config.TempDir, "analysis_"+uniqueID+".pdf")

	out, err := os.Create(inFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
		return
	}

	_, err = out.ReadFrom(file)
	out.Close()
	if err != nil {
		os.Remove(inFile) // Clean up on error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save input file"})
		return
	}

	// Perform unwanted elements analysis
	analysis, err := pdfPkg.AnalyzeUnwantedElements(inFile)

	if err != nil {
		// Clean up temp file on error
		go func() {
			time.Sleep(AnalysisCleanupDelay)
			os.Remove(inFile)
		}()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unwanted elements analysis failed"})
		return
	}

	// Add PDF file ID to response so frontend can request previews
	// The uniqueID is already generated above, use it as the file identifier
	response := gin.H{
		"total_pages":        analysis.TotalPages,
		"image_candidates":   analysis.ImageCandidates,
		"text_candidates":    analysis.TextCandidates,
		"overall_confidence": analysis.OverallConfidence,
		"recommendations":    analysis.Recommendations,
		"debug_logs":         analysis.DebugLogs,
		"pdf_file_id":        uniqueID, // Include file ID for preview requests
	}

	c.JSON(http.StatusOK, response)

	// Clean up temp file after response is sent
	// Wait for response to be sent first to avoid race condition
	defer func() {
		// Small delay to ensure file is not being read
		go func() {
			time.Sleep(AnalysisCleanupDelay)
			os.Remove(inFile)
		}()
	}()
}

func HandlePreviewImage(c *gin.Context, config *Config) {
	// Get parameters
	pdfFileID := c.Query("pdf_file_id")
	elementID := c.Query("element_id")

	if pdfFileID == "" || elementID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pdf_file_id and element_id are required"})
		return
	}

	// Find the uploaded PDF file by ID
	// Look for files matching the pattern: analysis_{pdfFileID}.pdf in temp directory
	// The file is saved as analysis_{uniqueID}.pdf in HandleAnalyzeUnwantedElements
	pdfFile := filepath.Join(config.TempDir, "analysis_"+pdfFileID+".pdf")

	// Check if file exists
	if _, err := os.Stat(pdfFile); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "PDF file not found"})
		return
	}

	// Re-analyze to get metadata for the element
	analysis, err := pdfPkg.AnalyzeUnwantedElements(pdfFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze PDF"})
		return
	}

	// Find the element in the analysis
	var elementMetadata map[string]string
	for _, candidate := range analysis.ImageCandidates {
		if candidate.ID == elementID {
			elementMetadata = candidate.Metadata
			break
		}
	}

	if elementMetadata == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Element not found in analysis"})
		return
	}

	// Extract image preview
	previewDir := filepath.Join(config.TempDir, "previews")
	previewPath, err := pdfPkg.ExtractImagePreview(pdfFile, previewDir, elementID, elementMetadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to extract image: %v", err)})
		return
	}

	// Serve the image file
	c.File(previewPath)

	// Clean up after a delay (image should be loaded by browser by then)
	go func() {
		time.Sleep(5 * time.Minute)
		os.Remove(previewPath)
	}()
}

func HandleRemoveSelectedElements(c *gin.Context, config *Config) {
	elementsParam := c.PostForm("elements")
	if elementsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No elements selected for removal"})
		return
	}

	// Parse selected element IDs
	elementIDs := strings.Split(elementsParam, ",")
	for i := range elementIDs {
		elementIDs[i] = strings.TrimSpace(elementIDs[i])
	}

	// handlePDFFile already sends the file for download
	handlePDFFile(c, config, func(inFile, outFile string) error {
		// Try removing as images first (selective removal)
		// If that fails, fall back to watermark removal (removes all pdfcpu watermarks)
		err := pdfPkg.RemoveElementsByIDs(inFile, outFile, "image", elementIDs)
		if err != nil {
			// If image removal fails, try watermark removal as fallback
			log.Printf("Image removal failed: %v, trying watermark removal...", err)
			return pdfPkg.RemoveElementFromPDF(inFile, outFile, "watermark")
		}
		return nil
	}, "unwanted_elements_removed")
}

func handlePDFFile(c *gin.Context, config *Config, operation func(string, string) error, suffix string) {
	file, header, err := c.Request.FormFile("pdf")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No PDF file provided"})
		return
	}
	defer file.Close()

	// Validate PDF file
	if err := validatePDFFile(file, header, config.MaxFileSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create temp input file
	if err := ensureTempDir(config.TempDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp directory"})
		return
	}

	uniqueID := generateUniqueID()
	inFile := filepath.Join(config.TempDir, "input_"+uniqueID+".pdf")
	outFile := filepath.Join(config.TempDir, "output_"+uniqueID+"_"+suffix+".pdf")

	out, err := os.Create(inFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
		return
	}

	_, err = out.ReadFrom(file)
	out.Close()
	if err != nil {
		os.Remove(inFile) // Clean up on error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save input file"})
		return
	}

	// Perform operation
	err = operation(inFile, outFile)
	if err != nil {
		os.Remove(inFile) // Clean up input file on error
		if _, statErr := os.Stat(outFile); statErr == nil {
			os.Remove(outFile) // Clean up output file if it exists
		}
		log.Printf("PDF operation error: %v", err)
		// Return more detailed error message to client
		errorMsg := "PDF operation failed"
		if errStr := err.Error(); errStr != "" {
			// Truncate long error messages but include key info
			if len(errStr) > 200 {
				errorMsg = errStr[:200] + "..."
			} else {
				errorMsg = errStr
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
		return
	}

	// Verify output file exists before sending
	if _, err := os.Stat(outFile); os.IsNotExist(err) {
		os.Remove(inFile)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF operation did not produce output file"})
		return
	}

	// Set headers for file download
	c.Header("Content-Type", "application/pdf")

	// Get original filename from form if available, otherwise use default
	filename := "document_" + suffix + ".pdf"
	if header != nil {
		originalName := header.Filename
		// Remove .pdf extension if present, add suffix
		if strings.HasSuffix(strings.ToLower(originalName), ".pdf") {
			filename = originalName[:len(originalName)-4] + "_" + suffix + ".pdf"
		} else {
			filename = originalName + "_" + suffix + ".pdf"
		}
		filename = sanitizeFilename(filename)
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	// Return the processed file for download
	c.File(outFile)

	// Clean up temp files after response is sent to avoid race conditions
	// Use defer with goroutine to wait for file transfer completion
	defer func() {
		go func() {
			// Wait a bit to ensure file transfer completes
			time.Sleep(FileCleanupDelay)
			os.Remove(inFile)
			os.Remove(outFile)
		}()
	}()
}

// ensureTempDir creates the temp directory if it doesn't exist
func ensureTempDir(tempDir string) error {
	return os.MkdirAll(tempDir, DefaultFilePermissions)
}

// sanitizeFilename removes path traversal attempts and dangerous characters
func sanitizeFilename(filename string) string {
	// Remove directory separators and path traversal attempts
	filename = strings.ReplaceAll(filename, "..", "")
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	// Get just the base filename to prevent path issues
	filename = filepath.Base(filename)

	// Remove any remaining dangerous characters
	filename = strings.TrimSpace(filename)

	// If empty after sanitization, use default
	if filename == "" {
		filename = "document.pdf"
	}

	return filename
}

// generateUniqueID generates a unique identifier for temp files
func generateUniqueID() string {
	// Use timestamp + random bytes for uniqueness
	b := make([]byte, 8)
	rand.Read(b)
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%d_%s", timestamp, hex.EncodeToString(b))
}

// validatePDFFile checks if the file is a valid PDF by reading the header
func validatePDFFile(file multipart.File, header *multipart.FileHeader, maxSize int64) error {
	if header.Size > maxSize {
		return fmt.Errorf("file size %d exceeds maximum allowed %d bytes", header.Size, maxSize)
	}

	// Read first 4 bytes to check PDF header
	buffer := make([]byte, 4)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file header: %v", err)
	}

	if n >= 4 && string(buffer[:4]) != "%PDF" {
		return fmt.Errorf("invalid PDF file: header does not match")
	}

	// Seek back to beginning for subsequent reads
	_, err = file.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to reset file position: %v", err)
	}

	return nil
}
