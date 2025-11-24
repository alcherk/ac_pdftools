package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExtractImagePreview extracts an image from a PDF file for preview
// Returns the path to the extracted image file
func ExtractImagePreview(pdfFile, outputDir, elementID string, metadata map[string]string) (string, error) {
	// Get image ID and try to find which page it's on
	imgID := ""
	if id, ok := metadata["image_id"]; ok && id != "" {
		imgID = id
	}
	
	if imgID == "" {
		return "", fmt.Errorf("cannot extract image: missing image_id in metadata")
	}
	
	// For repeating elements, we need to find a page where it appears
	// We'll analyze the PDF to find which page has this image
	page := 1 // Default to page 1
	if pageStr, ok := metadata["page_count"]; ok && pageStr != "" {
		// page_count tells us how many pages, but we need an actual page number
		// We'll extract from page 1 as a representative sample
		page = 1
	}
	
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %v", err)
	}
	
	// Generate output filename
	outputFile := filepath.Join(outputDir, fmt.Sprintf("preview_%s.png", sanitizeID(elementID)))
	
	// Extract images from the page
	extractDir := filepath.Join(outputDir, fmt.Sprintf("extract_%s", sanitizeID(elementID)))
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create extract directory: %v", err)
	}
	defer os.RemoveAll(extractDir) // Clean up extract directory
	
	// Use pdfcpu extract to extract images from the page
	pageSpec := fmt.Sprintf("%d", page)
	output, err := execCommandWithTimeout(DefaultCLITimeout, "pdfcpu", "extract", "-mode=image", "-pages="+pageSpec, pdfFile, extractDir)
	if err != nil {
		return "", fmt.Errorf("pdfcpu extract failed: %v\nOutput: %s", err, string(output))
	}
	
	// Find the extracted image file
	// pdfcpu extracts images with names like "page_1_img_0.png" or similar
	files, err := os.ReadDir(extractDir)
	if err != nil {
		return "", fmt.Errorf("failed to read extract directory: %v", err)
	}
	
	var imageFile string
	// Try to find image by ID in filename or use first image
	// pdfcpu extracts images with names that may include the ID
	for _, file := range files {
		if !file.IsDir() && (strings.HasSuffix(strings.ToLower(file.Name()), ".png") || 
			strings.HasSuffix(strings.ToLower(file.Name()), ".jpg") ||
			strings.HasSuffix(strings.ToLower(file.Name()), ".jpeg")) {
			// Try to match by image ID (ID might be in filename or we use first match)
			if imgID != "" && strings.Contains(file.Name(), imgID) {
				imageFile = filepath.Join(extractDir, file.Name())
				break
			}
		}
	}
	
	// If not found by ID, use first image file found
	if imageFile == "" {
		for _, file := range files {
			if !file.IsDir() && (strings.HasSuffix(strings.ToLower(file.Name()), ".png") || 
				strings.HasSuffix(strings.ToLower(file.Name()), ".jpg") ||
				strings.HasSuffix(strings.ToLower(file.Name()), ".jpeg")) {
				imageFile = filepath.Join(extractDir, file.Name())
				break
			}
		}
	}
	
	if imageFile == "" {
		return "", fmt.Errorf("no image file found in extract directory")
	}
	
	// Copy the extracted image to the output location
	inputData, err := os.ReadFile(imageFile)
	if err != nil {
		return "", fmt.Errorf("failed to read extracted image: %v", err)
	}
	
	if err := os.WriteFile(outputFile, inputData, 0644); err != nil {
		return "", fmt.Errorf("failed to write output image: %v", err)
	}
	
	return outputFile, nil
}

// sanitizeID sanitizes an ID string for use in filenames
func sanitizeID(id string) string {
	sanitized := strings.ReplaceAll(id, " ", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, "\\", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}
	return sanitized
}

