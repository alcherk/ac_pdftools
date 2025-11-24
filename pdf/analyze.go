package pdf

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

// imageInfo represents processed image information for analysis
type imageInfo struct {
	id         string
	obj        string
	width      int
	height     int
	size       string
	softMask   bool
	imgMask    bool
	colorSpace string
}

// imageWithPage represents an image with its page number for unwanted element detection
type imageWithPage struct {
	img  imageInfo
	page int
}

// rawImageData represents raw image data from pdfcpu output
type rawImageData struct {
	page       int
	obj        string
	id         string
	imgType    string
	softMask   string
	imgMask    string
	width      int
	height     int
	colorSpace string
	components int
	bpc        int
	interp     string
	size       string
}

// UnwantedElementCandidate represents a potential unwanted element found in the PDF
type UnwantedElementCandidate struct {
	Type        string            `json:"type"`        // "image" or "text"
	ID          string            `json:"id"`          // unique identifier
	Page        int               `json:"page"`        // page number
	Description string            `json:"description"` // human-readable description
	Confidence  float64           `json:"confidence"`  // 0-1 confidence score
	Metadata    map[string]string `json:"metadata"`    // additional info
}

// UnwantedElementsAnalysis represents the complete analysis result
type UnwantedElementsAnalysis struct {
	TotalPages        int                        `json:"total_pages"`
	ImageCandidates   []UnwantedElementCandidate `json:"image_candidates"`
	TextCandidates    []UnwantedElementCandidate `json:"text_candidates"`
	OverallConfidence float64                    `json:"overall_confidence"`
	Recommendations   []string                   `json:"recommendations"`
	DebugLogs         []string                   `json:"debug_logs"` // Debug information for troubleshooting
}

// AnalyzeUnwantedElements analyzes a PDF file and returns potential unwanted element candidates
func AnalyzeUnwantedElements(filename string) (*UnwantedElementsAnalysis, error) {
	analysis := &UnwantedElementsAnalysis{
		ImageCandidates: []UnwantedElementCandidate{},
		TextCandidates:  []UnwantedElementCandidate{},
		Recommendations: []string{},
		DebugLogs:       []string{},
	}
	
	// Create a debug log collector
	debugLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		analysis.DebugLogs = append(analysis.DebugLogs, msg)
		log.Printf(msg) // Also log to console
	}

	// Get total pages using pdfcpu info
	pages, err := getPageCount(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %v", err)
	}
	analysis.TotalPages = pages

	// Analyze images using pdfcpu images list
	imageCandidates, err := analyzeImages(filename, pages, debugLog)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze images: %v", err)
	}
	analysis.ImageCandidates = imageCandidates

	// Analyze content for potential unwanted text elements
	textCandidates, err := analyzeContent(filename, pages)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze content: %v", err)
	}
	analysis.TextCandidates = textCandidates

	// Calculate overall confidence
	totalCandidates := len(analysis.ImageCandidates) + len(analysis.TextCandidates)
	if totalCandidates > 0 {
		analysis.OverallConfidence = 0.5 // Base confidence if candidates found
		if totalCandidates > analysis.TotalPages {
			analysis.OverallConfidence = 0.8 // High if many candidates relative to pages
		}
	} else {
		analysis.OverallConfidence = 0.0 // No candidates found
	}

	// Add recommendations
	if len(analysis.ImageCandidates) > 0 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Images detected that may be unwanted elements - review and select for removal")
	}
	if len(analysis.TextCandidates) > 0 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Text elements detected that may be unwanted elements - review and select for removal")
	}
	if len(analysis.ImageCandidates) == 0 && len(analysis.TextCandidates) == 0 {
		analysis.Recommendations = append(analysis.Recommendations,
			"No obvious unwanted element candidates found - the PDF may not contain unwanted elements")
	}

	return analysis, nil
}

// getPageCount extracts the total number of pages from PDF
func getPageCount(filename string) (int, error) {
	output, err := execCommandWithTimeout(DefaultCLITimeout, "pdfcpu", "info", filename)
	if err != nil {
		return 0, fmt.Errorf("pdfcpu info failed: %v", err)
	}

	outputStr := string(output)

	// Look for page count in various formats
	patterns := []string{
		"Page count:\\s+(\\d+)",     // "Page count: 426" (pdfcpu v0.11.1 format)
		"Pages:\\s+(\\d+)",          // "Pages: 10"
		"pages\\s*=\\s*(\\d+)",      // "pages = 10"
		"No\\. of pages:\\s+(\\d+)", // "No. of pages: 10"
		"Pages: (\\d+)",             // "Pages: 10" (exact format)
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(outputStr)
		if len(matches) > 1 {
			if pageCount, err := strconv.Atoi(matches[1]); err == nil {
				return pageCount, nil
			}
		}
	}

	// Debug: include actual output in error
	return 0, fmt.Errorf("could not determine page count from output: %s", outputStr)
}

// analyzeImages uses pdfcpu to find images that might be unwanted elements
// debugLog is a function to collect debug messages (can be nil)
func analyzeImages(filename string, totalPages int, debugLog func(string, ...interface{})) ([]UnwantedElementCandidate, error) {
	if debugLog != nil {
		debugLog("[DEBUG] Starting unwanted elements analysis for file: %s (total pages: %d)", filename, totalPages)
	}
	
	output, err := execCommandWithTimeout(AnalysisTimeout, "pdfcpu", "images", "list", filename)
	if err != nil {
		return nil, fmt.Errorf("pdfcpu images list failed: %v", err)
	}

	if debugLog != nil {
		debugLog("[DEBUG] pdfcpu images list output length: %d bytes", len(output))
		// Show first 500 characters of output to debug format
		outputSample := string(output)
		if len(outputSample) > 500 {
			outputSample = outputSample[:500] + "..."
		}
		debugLog("[DEBUG] pdfcpu output sample (first 500 chars):\n%s", outputSample)
	}

	// First pass: collect all images by page
	imagesByPage := make(map[int][]imageInfo)
	var allImages []rawImageData

	// Parse the table output to extract image information
	lines := strings.Split(string(output), "\n")
	inTable := false
	headerLine := ""
	headerFound := false
	linesProcessed := 0
	linesSkipped := 0

	for i, line := range lines {
		lineTrimmed := strings.TrimSpace(line)
		
		// Look for table header - be more flexible
		if !headerFound && strings.Contains(lineTrimmed, "Page") && (strings.Contains(lineTrimmed, "Obj") || strings.Contains(lineTrimmed, "Type") || strings.Contains(lineTrimmed, "Id") || strings.Contains(lineTrimmed, "ID")) {
			inTable = true // Table header found
			headerFound = true
			headerLine = lineTrimmed
			if debugLog != nil {
				debugLog("[DEBUG] Found table header at line %d: %s", i+1, headerLine)
			}
			continue
		}
		
		// Skip empty lines and summary lines
		if !inTable || lineTrimmed == "" {
			continue
		}
		
		// Stop at summary lines
		if strings.Contains(strings.ToLower(lineTrimmed), "images available") || 
		   strings.Contains(strings.ToLower(lineTrimmed), "total images") ||
		   strings.Contains(strings.ToLower(lineTrimmed), "no images") {
			if debugLog != nil {
				debugLog("[DEBUG] Reached end of table at line %d: %s", i+1, lineTrimmed)
			}
			break
		}
		
		// Skip separator lines (lines with only dashes, equals, or box-drawing characters)
		if matched, _ := regexp.MatchString(`^[\s│|\-=_]+$`, lineTrimmed); matched {
			continue
		}

		// Try multiple parsing strategies
		var parts []string
		
		// Strategy 1: Split by │ (box-drawing character)
		if strings.Contains(line, "│") {
			parts = strings.Split(line, "│")
		} else if strings.Contains(line, "|") {
			// Strategy 2: Split by | (pipe character)
			parts = strings.Split(line, "|")
		} else if strings.Contains(line, "\t") {
			// Strategy 3: Tab-separated
			parts = strings.Split(line, "\t")
		} else {
			// Strategy 4: Multiple spaces
			parts = regexp.MustCompile(`\s{2,}`).Split(line, -1)
		}
		
		// Trim spaces from all parts
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		
		// Remove empty parts
		filteredParts := []string{}
		for _, p := range parts {
			if p != "" {
				filteredParts = append(filteredParts, p)
			}
		}
		parts = filteredParts
		
		// Need at least 3 fields (Page, ID, and something else)
		if len(parts) < 3 {
			linesSkipped++
			if debugLog != nil && linesSkipped <= 5 {
				preview := lineTrimmed
				if len(preview) > 100 {
					preview = preview[:100]
				}
				debugLog("[DEBUG] Skipped line %d (too few fields, got %d): %s", i+1, len(parts), preview)
			}
			continue
		}

		linesProcessed++
		
		// Try to extract fields - be more flexible with column positions
		var pageStr, objStr, idStr, imgType, softMask, imgMask, widthStr, heightStr, colorSpace, compStr, bpcStr, interp, sizeStr string
		
		if len(parts) > 0 {
			pageStr = parts[0]
		}
		if len(parts) > 1 {
			objStr = parts[1]
		}
		if len(parts) > 2 {
			idStr = parts[2]
		}
		if len(parts) > 3 {
			imgType = parts[3]
		}
		if len(parts) > 4 {
			softMask = parts[4]
		}
		if len(parts) > 5 {
			imgMask = parts[5]
		}
		if len(parts) > 6 {
			widthStr = parts[6]
		}
		if len(parts) > 7 {
			heightStr = parts[7]
		}
		if len(parts) > 8 {
			colorSpace = parts[8]
		}
		if len(parts) > 9 {
			compStr = parts[9]
		}
		if len(parts) > 10 {
			bpcStr = parts[10]
		}
		if len(parts) > 11 {
			interp = parts[11]
		}
		if len(parts) > 12 {
			sizeStr = parts[12]
		} else if len(parts) > 8 {
			// Size might be in a different position, try to find it
			// Look for size-like strings (contains KB, MB, or numbers)
			for i := 8; i < len(parts); i++ {
				if strings.Contains(strings.ToUpper(parts[i]), "KB") || 
				   strings.Contains(strings.ToUpper(parts[i]), "MB") ||
				   strings.Contains(strings.ToUpper(parts[i]), "B") {
					sizeStr = parts[i]
					break
				}
			}
		}
		
		// Skip if we don't have essential fields
		if pageStr == "" || idStr == "" {
			continue
		}

		// Parse numeric values, default to 0 if parsing fails
			page, _ := strconv.Atoi(pageStr)
		if page == 0 {
			continue // Skip if we can't parse the page number
		}
		
			width, _ := strconv.Atoi(widthStr)
			height, _ := strconv.Atoi(heightStr)
			comp, _ := strconv.Atoi(compStr)
			bpc, _ := strconv.Atoi(bpcStr)

		// If size is empty, try to find it elsewhere or set default
		if sizeStr == "" {
			// Try to find size in other fields - sometimes it might be in a different position
			for _, part := range parts {
				if strings.Contains(strings.ToUpper(part), "KB") || 
				   strings.Contains(strings.ToUpper(part), "MB") ||
				   (strings.Contains(strings.ToUpper(part), "B") && len(part) > 1) {
					sizeStr = part
					break
				}
			}
		}

			rawImg := rawImageData{
				page:       page,
				obj:        objStr,
				id:         idStr,
				imgType:    imgType,
				softMask:   softMask,
				imgMask:    imgMask,
				width:      width,
				height:     height,
				colorSpace: colorSpace,
				components: comp,
				bpc:        bpc,
				interp:     interp,
				size:       sizeStr,
			}

		// Debug: Print each image found
		if debugLog != nil {
			prefix := extractIdPrefix(idStr)
			fileSizeKB := parseFileSizeKB(sizeStr)
			debugLog("[DEBUG] Image found - Page: %d, ID: %s, Prefix: '%s', Size: %s (%.1fKB), Dimensions: %dx%d, ColorSpace: %s",
				page, idStr, prefix, sizeStr, fileSizeKB, width, height, colorSpace)
		}

			allImages = append(allImages, rawImg)
			imagesByPage[page] = append(imagesByPage[page], imageInfo{
				id:         idStr,
				obj:        objStr,
				width:      width,
				height:     height,
				size:       sizeStr,
				softMask:   softMask == "*",
				imgMask:    imgMask == "*",
				colorSpace: colorSpace,
			})
		}
	
	if debugLog != nil {
		debugLog("[DEBUG] Total lines processed: %d, Lines skipped: %d", linesProcessed, linesSkipped)
		debugLog("[DEBUG] Total images parsed: %d, Images by page count: %d", len(allImages), len(imagesByPage))
		if len(allImages) == 0 && linesProcessed > 0 {
			debugLog("[DEBUG] WARNING: Processed %d lines but parsed 0 images. Format might be unexpected.", linesProcessed)
			// Show sample of what was processed
			if len(lines) > 10 {
				debugLog("[DEBUG] Sample lines (10-20):")
				for i := 10; i < 20 && i < len(lines); i++ {
					linePreview := strings.TrimSpace(lines[i])
					if linePreview != "" && len(linePreview) > 0 {
						if len(linePreview) > 100 {
							linePreview = linePreview[:100] + "..."
						}
						debugLog("[DEBUG]   Line %d: %s", i+1, linePreview)
					}
				}
			}
		}
		if len(allImages) == 0 && !headerFound && len(output) > 0 {
			debugLog("[DEBUG] WARNING: Table header not found. Output might not be in expected format.")
		}
	}

	// Second pass: identify repeating unwanted element patterns
	candidates := []UnwantedElementCandidate{}

	// Check for images that appear on many pages (80%+ for broader detection)
	maxPages := totalPages
	minPages := int(float64(totalPages) * MinPageCoverageThreshold)
	if debugLog != nil {
		debugLog("[DEBUG] Minimum pages for watermark detection: %d (%.0f%% of %d total pages)", minPages, MinPageCoverageThreshold*100, totalPages)
	}

	// Group images by similar characteristics (size, position indicators, and naming patterns)
	imageSignatures := make(map[string][]int) // signature -> list of pages
	// Also group by prefix for enhanced detection
	imagesByPrefix := make(map[string][]imageWithPage) // prefix -> list of images with page numbers

	for page, imgs := range imagesByPage {
		for _, img := range imgs {
			// Create enhanced signature including naming patterns for watermark detection
			// Include image ID prefix for publisher unwanted element patterns (e.g., "Image-")
			prefix := extractIdPrefix(img.id)
			signature := fmt.Sprintf("%dx%d_%s_%s_prefix:%s", img.width, img.height, img.colorSpace, img.size, prefix)
			imageSignatures[signature] = append(imageSignatures[signature], page)
			
			// Group by prefix for enhanced detection
			if prefix != "unknown" {
				imagesByPrefix[prefix] = append(imagesByPrefix[prefix], imageWithPage{img: img, page: page})
				if debugLog != nil {
					debugLog("[DEBUG] Grouped image by prefix '%s': Page %d, ID: %s, Size: %s", prefix, page, img.id, img.size)
				}
			}
		}
	}
	
	if debugLog != nil {
		debugLog("[DEBUG] Prefix groups found: %d", len(imagesByPrefix))
		for prefix, imgs := range imagesByPrefix {
			debugLog("[DEBUG]   Prefix '%s': %d images", prefix, len(imgs))
		}
		debugLog("[DEBUG] Image signatures found: %d", len(imageSignatures))
	}

	// PRIORITY DETECTION: Images appearing on ALL pages with same prefix and size >= 30KB
	if debugLog != nil {
		debugLog("[DEBUG] Starting full-page unwanted element detection...")
	}
	fullPageCandidates := detectFullPageUnwantedElements(imagesByPrefix, totalPages, imageSignatures, debugLog)
	if debugLog != nil {
		debugLog("[DEBUG] Full-page unwanted element candidates found: %d", len(fullPageCandidates))
		for i, candidate := range fullPageCandidates {
			debugLog("[DEBUG]   Candidate %d: %s (confidence: %.1f%%)", i+1, candidate.Description, candidate.Confidence*100)
		}
	}
	candidates = append(candidates, fullPageCandidates...)
	
	// Track which signatures we've already handled to avoid duplicates
	handledSignatures := make(map[string]bool)
		for _, candidate := range fullPageCandidates {
			if sig, ok := candidate.Metadata["signature"]; ok {
				handledSignatures[sig] = true
			}
		}

		// Find signatures that appear on many pages (but not all - those were handled above)
	for signature, pages := range imageSignatures {
			if handledSignatures[signature] {
				continue // Skip if already detected as full-page unwanted element
			}
			
		if len(pages) >= minPages || hasContinuousRange(pages, minPages) {
				// This is likely a repeating unwanted element image
				// Either widespread (>=80% of pages) OR continuous range (>=80% consecutive pages)
			// Use the first occurrence as representative
			firstPage := pages[0]
			firstImg := imageInfo{}

			// Find the image data for this page
			for _, img := range imagesByPage[firstPage] {
				testSig := fmt.Sprintf("%dx%d_%s_%s", img.width, img.height, img.colorSpace, img.size)
				if testSig == signature {
					firstImg = img
					break
				}
			}

				sigPreview := signature
				if len(sigPreview) > 20 {
					sigPreview = sigPreview[:20] + "..."
				}
				if debugLog != nil {
					debugLog("[DEBUG] Repeating unwanted element detected - Signature: %s, Pages: %d/%d, ID: %s, Size: %s",
						sigPreview, len(pages), maxPages, firstImg.id, firstImg.size)
				}

				// Calculate enhanced confidence for repeating unwanted elements
				confidence := calculateRepeatingUnwantedElementConfidence(firstImg, len(pages), maxPages)

				// Extract prefix from signature for better description
				prefix := extractIdPrefix(firstImg.id)
				description := fmt.Sprintf("Repeating unwanted element image: size %dx%d (%s), appears on %d/%d pages (%s)",
					firstImg.width, firstImg.height, firstImg.colorSpace, len(pages), maxPages, firstImg.size)
				if prefix != "unknown" && prefix != "" {
					description = fmt.Sprintf("Repeating unwanted element image (prefix '%s'): size %dx%d (%s), file size %s, appears on %d/%d pages",
						prefix, firstImg.width, firstImg.height, firstImg.colorSpace, firstImg.size, len(pages), maxPages)
				}

				candidate := UnwantedElementCandidate{
				Type: "image",
					ID:   fmt.Sprintf("repeating_unwanted_element_%s", signature[:8]), // Use signature hash for unique ID
					Page: 0,                                                          // Appears on multiple pages
					Description: description,
				Confidence: confidence,
				Metadata: map[string]string{
					"signature":  signature,
					"page_count": strconv.Itoa(len(pages)),
					"max_pages":  strconv.Itoa(maxPages),
						"prefix":     prefix,
					"soft_mask":  strconv.FormatBool(firstImg.softMask),
					"image_mask": strconv.FormatBool(firstImg.imgMask),
						"type":       "repeating_unwanted_element",
						"object":     firstImg.obj,     // Store object number for removal
						"image_id":   firstImg.id,      // Store image ID for removal
				},
			}

				if debugLog != nil {
					debugLog("[DEBUG]   Created repeating unwanted element candidate: %s (confidence: %.1f%%)", candidate.Description, candidate.Confidence*100)
				}
			candidates = append(candidates, candidate)
		}
	}

		if debugLog != nil {
			debugLog("[DEBUG] Repeating unwanted element candidates found: %d", len(candidates)-len(fullPageCandidates))
		}

		// Skip individual suspicious images - only show images that appear on 80%+ pages
		// Individual images below the threshold are not shown as they're less likely to be unwanted elements
		if debugLog != nil {
			debugLog("[DEBUG] Skipping individual images below 80%% threshold (only showing repeating unwanted elements)")
		}

	// Count different types of candidates
	repeatingCount := 0
	individualCount := 0
	for _, c := range candidates {
		if c.Metadata["type"] == "repeating_unwanted_element" {
			repeatingCount++
		} else if c.Page > 0 {
			individualCount++
		}
	}
		if debugLog != nil {
			debugLog("[DEBUG] Total unwanted element candidates found: %d (full-page: %d, repeating: %d, individual: %d)",
				len(candidates), len(fullPageCandidates), repeatingCount, individualCount)
	}

	return candidates, nil
}

// analyzeContent looks for text that might be unwanted elements
func analyzeContent(filename string, totalPages int) ([]UnwantedElementCandidate, error) {
	candidates := []UnwantedElementCandidate{}

	// For now, return empty list - content analysis is complex
	// Could be enhanced to extract text and detect repeating patterns
	// Using pdfcpu extract -mode content filename outputdir/
	// Then analyze the raw content for watermark-like text

	return candidates, nil
}

// detectFullPageUnwantedElements detects images that appear on ALL pages with same prefix and size >= 30KB
func detectFullPageUnwantedElements(imagesByPrefix map[string][]imageWithPage, totalPages int, imageSignatures map[string][]int, debugLog func(string, ...interface{})) []UnwantedElementCandidate {
	candidates := []UnwantedElementCandidate{}
	
	// Group images by prefix that have same size and appear on all pages
	for prefix, images := range imagesByPrefix {
		// Need at least 80% coverage to be considered (but prefer 100%)
		minImagesNeeded := int(float64(totalPages) * MinPageCoverageThreshold)
		if debugLog != nil {
			debugLog("[DEBUG] Checking prefix '%s': %d images found (need %d for %.0f%% threshold)",
				prefix, len(images), minImagesNeeded, MinPageCoverageThreshold*100)
		}
		
		if len(images) < minImagesNeeded {
			if debugLog != nil {
				debugLog("[DEBUG]   Prefix '%s' skipped: not enough images (%d < %d)", prefix, len(images), minImagesNeeded)
			}
			continue // Not enough images to meet minimum threshold
		}
		
		// Group by file size (same size indicates same image)
		// Allow for slight size variations by rounding
		imagesBySize := make(map[string][]imageWithPage)
		for _, imgPage := range images {
			fileSizeKB := parseFileSizeKB(imgPage.img.size)
			if debugLog != nil {
				debugLog("[DEBUG]   Prefix '%s' image: Page %d, ID: %s, Size: %s (parsed: %.1fKB)",
					prefix, imgPage.page, imgPage.img.id, imgPage.img.size, fileSizeKB)
			}
			
			if fileSizeKB >= MinWatermarkFileSizeKB {
				// Use size as key (rounded to nearest KB for grouping similar sizes)
				// Round to handle slight variations (e.g., 110.2KB and 110.8KB both become 110KB)
				sizeKey := fmt.Sprintf("%.0fKB", fileSizeKB)
				imagesBySize[sizeKey] = append(imagesBySize[sizeKey], imgPage)
				if debugLog != nil {
					debugLog("[DEBUG]     Added to size group '%s' (meets %.0fKB threshold)", sizeKey, MinWatermarkFileSizeKB)
				}
			} else {
				if debugLog != nil {
					debugLog("[DEBUG]     Skipped: size %.1fKB < %.0fKB threshold", fileSizeKB, MinWatermarkFileSizeKB)
				}
			}
		}
		
		if debugLog != nil {
			debugLog("[DEBUG]   Prefix '%s' size groups: %d", prefix, len(imagesBySize))
		}
		
		// Also try grouping by similar size ranges if exact size grouping doesn't work well
		// Group images within ±5KB of each other
		if len(imagesBySize) == 0 {
			continue // No images met the size threshold
		}
		
		// Check each size group to see if it covers all pages
		for sizeKey, sizeGroup := range imagesBySize {
			if len(sizeGroup) < totalPages {
				continue
			}
			
			// Count unique pages covered by this size group
			pagesCovered := make(map[int]bool)
			var representativeImg imageInfo
			
			for _, imgPage := range sizeGroup {
				pagesCovered[imgPage.page] = true
				// Use first image as representative
				if representativeImg.id == "" {
					representativeImg = imgPage.img
				}
			}
			
			coverageCount := len(pagesCovered)
			coveragePercent := float64(coverageCount) / float64(totalPages)
			
			if debugLog != nil {
				debugLog("[DEBUG]     Size group '%s' for prefix '%s': %d images covering %d unique pages (%.1f%% of %d total)",
					sizeKey, prefix, len(sizeGroup), coverageCount, coveragePercent*100, totalPages)
			}
			
			// Check if covers enough pages (80%+ threshold)
			if coveragePercent >= MinPageCoverageThreshold {
				if debugLog != nil {
					debugLog("[DEBUG]       ✓ Meets %.0f%% threshold! Creating watermark candidate...", MinPageCoverageThreshold*100)
				}
				signature := fmt.Sprintf("%dx%d_%s_%s_prefix:%s", 
					representativeImg.width, representativeImg.height, 
					representativeImg.colorSpace, representativeImg.size, prefix)
				
				// Confidence based on coverage: 100% = 95%, 80%+ = 85-95%
				var confidence float64
				var coverageStr string
				var candidateType string
				
				if coverageCount == totalPages {
					confidence = 0.95 // Very high confidence for full-page watermarks
					coverageStr = "100%"
					candidateType = "fullpage_watermark"
				} else {
					// Scale confidence based on coverage percentage
					confidence = 0.80 + (coveragePercent * 0.15) // 80-95% confidence for 80-100% coverage
					coverageStr = fmt.Sprintf("%.0f%%", coveragePercent*100)
					candidateType = "repeating_watermark"
				}
				
				description := fmt.Sprintf("Unwanted element (prefix '%s'): size %dx%d (%s), file size %s, appears on %d/%d pages (%s)",
					prefix, representativeImg.width, representativeImg.height, 
					representativeImg.colorSpace, sizeKey, coverageCount, totalPages, coverageStr)
				
				candidate := UnwantedElementCandidate{
					Type: "image",
					ID:   fmt.Sprintf("%s_%s_%s", candidateType, prefix, sizeKey),
					Page: 0, // Appears on multiple pages
					Description: description,
					Confidence: confidence,
					Metadata: map[string]string{
						"signature":    signature,
						"prefix":       prefix,
						"file_size_kb": fmt.Sprintf("%.1f", parseFileSizeKB(representativeImg.size)),
						"page_count":   strconv.Itoa(coverageCount),
						"total_pages":  strconv.Itoa(totalPages),
						"coverage":     coverageStr,
						"type":         candidateType,
						"soft_mask":    strconv.FormatBool(representativeImg.softMask),
						"image_mask":   strconv.FormatBool(representativeImg.imgMask),
						"object":       representativeImg.obj,    // Store object number for removal
						"image_id":     representativeImg.id,     // Store image ID for removal
					},
				}
				
				if debugLog != nil {
					debugLog("[DEBUG]       Created candidate: %s (confidence: %.1f%%)", candidate.Description, candidate.Confidence*100)
				}
				candidates = append(candidates, candidate)
			} else {
				if debugLog != nil {
					debugLog("[DEBUG]       ✗ Does not meet %.0f%% threshold (%.1f%% coverage)", MinPageCoverageThreshold*100, coveragePercent*100)
				}
			}
		}
	}
	
	if debugLog != nil {
		debugLog("[DEBUG] Full-page unwanted element detection complete: %d candidates found", len(candidates))
	}
	return candidates
}

// parseFileSizeKB parses file size string (e.g., "30KB", "35.2kb", "1024B") and returns size in KB
func parseFileSizeKB(sizeStr string) float64 {
	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))
	
	// Remove common separators
	sizeStr = strings.ReplaceAll(sizeStr, ",", "")
	sizeStr = strings.ReplaceAll(sizeStr, " ", "")
	
	// Extract number and unit
	var numStr string
	var unit string
	
	for i, r := range sizeStr {
		if r >= '0' && r <= '9' || r == '.' {
			numStr += string(r)
		} else {
			unit = sizeStr[i:]
			break
		}
	}
	
	if numStr == "" {
		return 0
	}
	
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}
	
	// Convert to KB based on unit
	switch {
	case strings.HasPrefix(unit, "KB") || strings.HasPrefix(unit, "K"):
		return num
	case strings.HasPrefix(unit, "MB") || strings.HasPrefix(unit, "M"):
		return num * 1024
	case strings.HasPrefix(unit, "GB") || strings.HasPrefix(unit, "G"):
		return num * 1024 * 1024
	case strings.HasPrefix(unit, "B") || unit == "":
		return num / 1024 // Assume bytes if no unit or just "B"
	default:
		return num / 1024 // Default to bytes conversion
	}
}

// calculateImageConfidence determines how likely an image is to be a watermark
func calculateImageConfidence(width, height, page, totalPages int) float64 {
	confidence := BaseConfidence

	// Small images might be logos/watermarks
	if width < 200 || height < 200 {
		confidence += SizeBasedConfidenceBonus
	}

	// Images appearing on first/last page
	if page == 1 || page == totalPages {
		confidence += PositionBasedConfidenceBonus
	}

	// Cap at reasonable levels
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// calculateRepeatingUnwantedElementConfidence determines confidence for images that repeat across pages
func calculateRepeatingUnwantedElementConfidence(img imageInfo, pageCount, totalPages int) float64 {
	confidence := 0.4 // Base confidence for repeating images

	// The more pages it appears on, the more confident (most important factor)
	pageRatio := float64(pageCount) / float64(totalPages)
	confidence += pageRatio * 0.6 // Up to +0.6 for 100% coverage

	// Size factors: medium to large images are more likely watermarks
	if img.width > 300 || img.height > 400 {
		confidence += 0.15
	}
	if img.width > 500 || img.height > 700 {
		confidence += 0.15 // Additional bonus for very large images
	}

	// Transparency indicators (optional bonus, not required)
	if img.softMask {
		confidence += 0.05
	}

	// Cap at reasonable high confidence
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// extractIdPrefix extracts meaningful prefix patterns from image IDs for watermark detection
func extractIdPrefix(id string) string {
	if strings.HasPrefix(id, "Image-") {
		return "Image"
	}
	if strings.HasPrefix(id, "Img") {
		return "Img"
	}
	if strings.HasPrefix(id, "WM-") || strings.HasPrefix(id, "Watermark") {
		return "Watermark"
	}
	if strings.Contains(id, "-") {
		// Extract prefix before first dash for numbered patterns
		parts := strings.Split(id, "-")
		if len(parts) > 1 && len(parts[0]) > 0 {
			return parts[0]
		}
	}

	// Default: extract first 3-5 characters for pattern matching
	if len(id) >= 3 {
		prefixLen := 3
		if len(id) > 5 {
			prefixLen = 5
		}
		return id[:prefixLen]
	}

	return "unknown"
}

// hasContinuousRange checks if an array of pages contains a continuous range of sufficient length
func hasContinuousRange(pages []int, minLength int) bool {
	if len(pages) < minLength {
		return false
	}

	// Sort pages if not already sorted (though they should be from the map iteration)
	// In practice, the page numbers should be in order from the loop

	maxContinuous := 0
	currentContinuous := 1

	for i := 1; i < len(pages); i++ {
		if pages[i] == pages[i-1]+1 {
			currentContinuous++
			if currentContinuous > maxContinuous {
				maxContinuous = currentContinuous
			}
		} else {
			currentContinuous = 1
		}
	}

	return maxContinuous >= minLength
}

// For JSON marshaling
func (wa UnwantedElementsAnalysis) String() string {
	data, err := json.MarshalIndent(wa, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling analysis: %v", err)
	}
	return string(data)
}

