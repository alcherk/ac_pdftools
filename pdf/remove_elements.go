package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RemoveElementFromPDF removes overlay elements (watermarks or images) from a PDF file using pdfcpu CLI
func RemoveElementFromPDF(inFile, outFile, elementType string) error {
	return RemoveElementsByIDs(inFile, outFile, elementType, nil)
}

// RemoveElementsByIDs removes specific elements by their IDs from a PDF file using pdfcpu CLI
// elementIDs is a list of candidate IDs to remove (can be nil to remove all of a type)
func RemoveElementsByIDs(inFile, outFile, elementType string, elementIDs []string) error {
	// Validate element type
	if elementType != "watermark" && elementType != "image" {
		return fmt.Errorf("invalid element type: %s (supported: watermark, image)", elementType)
	}

	switch elementType {
	case "watermark":
		// pdfcpu watermark remove command
		// Syntax: pdfcpu watermark remove -- input.pdf output.pdf
		// Note: The "--" separator is required
		// Note: This only removes watermarks added by pdfcpu, not regular embedded images
		output, err := execCommandWithTimeout(DefaultCLITimeout, "pdfcpu", "watermark", "remove", "--", inFile, outFile)
		if err != nil {
			outputStr := string(output)

			// Check if error is because no watermarks were found
			// This happens when images are embedded as regular images, not pdfcpu watermarks
			if strings.Contains(outputStr, "no watermarks found") || strings.Contains(outputStr, "no stamps found") {
				// Try stamp remove as an alternative (stamps are similar to watermarks)
				log.Printf("No pdfcpu watermarks found, trying stamp remove as alternative...")
				stampOutput, stampErr := execCommandWithTimeout(DefaultCLITimeout, "pdfcpu", "stamp", "remove", "--", inFile, outFile)
				if stampErr == nil {
					if stampStr := string(stampOutput); stampStr != "" {
						log.Printf("pdfcpu stamp remove output: %s", stampStr)
					}
					return nil
				}

				// Check if stamp remove also failed with "no stamps found"
				stampOutputStr := string(stampOutput)
				if strings.Contains(stampOutputStr, "no stamps found") {
					// Neither watermark nor stamp removal worked - images are embedded in content
					return fmt.Errorf("the detected images are embedded as regular content in the PDF, not as pdfcpu watermarks or stamps. pdfcpu CLI can only remove watermarks/stamps that were added using pdfcpu's watermark or stamp commands. To remove regular embedded images, you would need to use a PDF library that can manipulate the PDF structure directly, or edit the PDF manually in a PDF editor")
				}

				// Stamp remove failed for a different reason, return original error
				if stampOutputStr != "" {
					return fmt.Errorf("pdfcpu stamp remove also failed: %v\nOutput: %s", stampErr, stampOutputStr)
				}
				return fmt.Errorf("pdfcpu stamp remove also failed: %v", stampErr)
			}

			// Include output in error for debugging
			if outputStr != "" {
				return fmt.Errorf("pdfcpu watermark remove failed: %v\nOutput: %s", err, outputStr)
			}
			return fmt.Errorf("pdfcpu watermark remove failed: %v", err)
		}
		// Log output for debugging even on success
		if outputStr := string(output); outputStr != "" {
			log.Printf("pdfcpu watermark remove output: %s", outputStr)
		}
		return nil
	case "image":
		// Image removal using pdfcpu images update
		// We replace images with a 1x1 transparent PNG to effectively remove them
		if elementIDs == nil || len(elementIDs) == 0 {
			return fmt.Errorf("image removal requires element IDs to identify which images to remove")
		}

		// For now, we can't remove specific images without re-analyzing the PDF
		// because we need the object numbers or (page, ID) pairs
		// This requires either:
		// 1. Re-analyzing the PDF to get object numbers for the selected IDs
		// 2. Storing object numbers in candidate metadata and passing them along
		// 3. Encoding object numbers in the candidate ID itself
		return removeImagesByIDs(inFile, outFile, elementIDs)
	default:
		return fmt.Errorf("unsupported element type: %s", elementType)
	}
}

// removeImagesByIDs removes specific images by analyzing the PDF and matching IDs
func removeImagesByIDs(inFile, outFile string, elementIDs []string) error {
	// Re-analyze the PDF to get object numbers for selected IDs
	analysis, err := AnalyzeUnwantedElements(inFile)
	if err != nil {
		return fmt.Errorf("failed to analyze PDF to find images: %v", err)
	}

	// Create a set of selected IDs for quick lookup
	selectedIDs := make(map[string]bool)
	for _, id := range elementIDs {
		selectedIDs[strings.TrimSpace(id)] = true
	}

	// Find matching images and collect their identifiers
	type imageToRemove struct {
		objNr  string
		pageNr int
		id     string
	}
	imagesToRemove := []imageToRemove{}

	// First, collect all images from the PDF to find all occurrences
	// We'll use pdfcpu images list to get all image occurrences
	output, err := execCommandWithTimeout(DefaultCLITimeout, "pdfcpu", "images", "list", inFile)
	if err != nil {
		return fmt.Errorf("failed to list images: %v", err)
	}

	// Parse the images list to build:
	// 1. A map of image_id -> []{page, object}
	// 2. A list of all images with their metadata (for pattern matching)
	type imageOccurrence struct {
		page int
		obj  string
		id   string
	}
	imageOccurrences := make(map[string][]imageOccurrence) // image_id -> occurrences
	allImageOccurrences := []imageOccurrence{}             // All images for pattern matching

	lines := strings.Split(string(output), "\n")
	inTable := false
	headerFound := false

	for _, line := range lines {
		lineTrimmed := strings.TrimSpace(line)
		if lineTrimmed == "" {
			continue
		}

		// Look for table header
		if !headerFound && (strings.Contains(lineTrimmed, "Page") || strings.Contains(lineTrimmed, "ID")) {
			inTable = true
			headerFound = true
			continue
		}

		if !inTable {
			continue
		}

		// Parse table rows
		parts := strings.Fields(lineTrimmed)
		if len(parts) < 3 {
			continue
		}

		pageStr := parts[0]
		objStr := ""
		idStr := ""

		if len(parts) > 1 {
			objStr = parts[1]
		}
		if len(parts) > 2 {
			idStr = parts[2]
		}

		page, err := strconv.Atoi(pageStr)
		if err != nil || page == 0 || idStr == "" {
			continue
		}

		occ := imageOccurrence{
			page: page,
			obj:  objStr,
			id:   idStr,
		}

		imageOccurrences[idStr] = append(imageOccurrences[idStr], occ)
		allImageOccurrences = append(allImageOccurrences, occ)
	}

	// Helper function to extract prefix from image ID
	extractPrefix := func(id string) string {
		if idx := strings.Index(id, "-"); idx > 0 {
			return id[:idx+1] // Include the dash
		}
		// Try underscore
		if idx := strings.Index(id, "_"); idx > 0 {
			return id[:idx+1]
		}
		return ""
	}

	// Search through candidates
	for _, candidate := range analysis.ImageCandidates {
		if selectedIDs[candidate.ID] {
			imgID := ""
			if id, ok := candidate.Metadata["image_id"]; ok && id != "" {
				imgID = id
			}

			prefix := ""
			if p, ok := candidate.Metadata["prefix"]; ok && p != "" {
				prefix = p
			}

			foundOccurrences := []imageOccurrence{}

			// Strategy 1: If we have an exact image_id, find all occurrences by ID
			if imgID != "" {
				if occurrences, found := imageOccurrences[imgID]; found {
					foundOccurrences = occurrences
					log.Printf("Found %d occurrences of exact image ID %s for candidate %s", len(occurrences), imgID, candidate.ID)
				}
			}

			// Strategy 2: If no exact matches but we have a prefix, find all images with that prefix
			if len(foundOccurrences) == 0 && prefix != "" {
				for _, occ := range allImageOccurrences {
					// Match if image ID starts with prefix (with or without dash/underscore)
					// Also check extracted prefix for flexibility
					occPrefix := extractPrefix(occ.id)
					if strings.HasPrefix(occ.id, prefix) ||
						strings.HasPrefix(occ.id, prefix+"-") ||
						strings.HasPrefix(occ.id, prefix+"_") ||
						occPrefix == prefix {
						foundOccurrences = append(foundOccurrences, occ)
					}
				}
				if len(foundOccurrences) > 0 {
					log.Printf("Found %d occurrences by prefix '%s' for candidate %s", len(foundOccurrences), prefix, candidate.ID)
				}
			}

			// Strategy 3: If we have signature in metadata, try to match by signature pattern
			if len(foundOccurrences) == 0 {
				signature, hasSignature := candidate.Metadata["signature"]
				if hasSignature && signature != "" {
					log.Printf("Attempting signature-based matching for candidate %s (signature: %s, prefix: %s)", candidate.ID, signature, prefix)

					// Extract prefix from signature: format is "widthxheight_colorspace_size_prefix:prefix"
					sigPrefix := ""
					if prefixIdx := strings.LastIndex(signature, "prefix:"); prefixIdx >= 0 {
						sigPrefix = strings.TrimSpace(signature[prefixIdx+7:]) // 7 = len("prefix:")
						// Extract the base prefix (might have trailing whitespace)
						if spaceIdx := strings.IndexAny(sigPrefix, " \t\n"); spaceIdx > 0 {
							sigPrefix = sigPrefix[:spaceIdx]
						}
					}

					// Use signature prefix if available, otherwise keep existing prefix
					if sigPrefix != "" {
						log.Printf("Extracted prefix '%s' from signature for candidate %s", sigPrefix, candidate.ID)
						prefix = sigPrefix
					}

					// Try case-insensitive prefix matching with all variations
					if prefix != "" && prefix != "unknown" {
						prefixLower := strings.ToLower(prefix)
						prefixUpper := strings.ToUpper(prefix)
						prefixTitle := ""
						if len(prefix) > 0 {
							prefixTitle = strings.ToUpper(prefix[:1]) + strings.ToLower(prefix[1:])
						}

						for _, occ := range allImageOccurrences {
							occIDLower := strings.ToLower(occ.id)
							occPrefix := extractPrefix(occ.id)
							occPrefixLower := strings.ToLower(occPrefix)

							// Multiple matching strategies
							matched := false

							// Direct prefix match (case-insensitive)
							if strings.HasPrefix(occIDLower, prefixLower) {
								matched = true
							}
							// Prefix with dash/underscore
							if !matched && (strings.HasPrefix(occIDLower, prefixLower+"-") || strings.HasPrefix(occIDLower, prefixLower+"_")) {
								matched = true
							}
							// Extracted prefix match
							if !matched && (occPrefixLower == prefixLower || strings.HasPrefix(occPrefixLower, prefixLower)) {
								matched = true
							}
							// Try capitalized versions
							if !matched && prefixTitle != "" {
								if strings.HasPrefix(occ.id, prefixTitle) || strings.HasPrefix(occ.id, prefixTitle+"-") || strings.HasPrefix(occ.id, prefixTitle+"_") {
									matched = true
								}
							}
							// Try uppercase version
							if !matched && prefixUpper != "" {
								if strings.HasPrefix(occ.id, prefixUpper) || strings.HasPrefix(occ.id, prefixUpper+"-") || strings.HasPrefix(occ.id, prefixUpper+"_") {
									matched = true
								}
							}

							if matched {
								foundOccurrences = append(foundOccurrences, occ)
							}
						}

						if len(foundOccurrences) > 0 {
							log.Printf("Found %d occurrences by signature-derived prefix '%s' (case-insensitive) for candidate %s", len(foundOccurrences), prefix, candidate.ID)
						}
					}
				}
			}

			// Strategy 4: Fallback to single occurrence if we have object number and page
			if len(foundOccurrences) == 0 {
				page := candidate.Page
				objNr := ""
				if obj, ok := candidate.Metadata["object"]; ok && obj != "" {
					objNr = obj
				}

				if objNr != "" && page > 0 {
					// Try to find this specific occurrence
					if imgID != "" {
						for _, occ := range allImageOccurrences {
							if occ.id == imgID && occ.page == page {
								foundOccurrences = append(foundOccurrences, occ)
								break
							}
						}
					} else if objNr != "" {
						for _, occ := range allImageOccurrences {
							if occ.obj == objNr && occ.page == page {
								foundOccurrences = append(foundOccurrences, occ)
								break
							}
						}
					}
				}
			}

			// Add all found occurrences to removal list
			if len(foundOccurrences) > 0 {
				for _, occ := range foundOccurrences {
					imagesToRemove = append(imagesToRemove, imageToRemove{
						objNr:  occ.obj,
						pageNr: occ.page,
						id:     occ.id,
					})
				}
				log.Printf("Total occurrences to remove for candidate %s: %d", candidate.ID, len(foundOccurrences))
			} else {
				log.Printf("Warning: Cannot find any occurrences for candidate %s (image_id: %s, prefix: %s)", candidate.ID, imgID, prefix)
			}
		}
	}

	if len(imagesToRemove) == 0 {
		return fmt.Errorf("no matching images found for selected IDs. The images may be repeating watermarks that appear on multiple pages")
	}

	// Create a blank 1x1 transparent PNG to replace images with
	blankImagePath, err := createBlankImage(filepath.Dir(outFile))
	if err != nil {
		return fmt.Errorf("failed to create blank image: %v", err)
	}
	defer os.Remove(blankImagePath)

	// Process images one by one
	// For multiple images, we need to chain operations: inFile -> temp1 -> temp2 -> ... -> outFile
	currentFile := inFile
	tempFiles := []string{}

	for i, img := range imagesToRemove {
		var tempFile string
		if i == len(imagesToRemove)-1 {
			// Last one goes to final output
			tempFile = outFile
		} else {
			// Intermediate files
			tempFile = filepath.Join(filepath.Dir(outFile), fmt.Sprintf("temp_%d_%s", i, filepath.Base(outFile)))
			tempFiles = append(tempFiles, tempFile)
		}

		var output []byte
		var err error

		if img.objNr != "" {
			// Use object number
			output, err = execCommandWithTimeout(DefaultCLITimeout, "pdfcpu", "images", "update", currentFile, blankImagePath, tempFile, img.objNr)
		} else if img.pageNr > 0 && img.id != "" {
			// Use page number and ID: format is "pageNr Id"
			pageIdArg := fmt.Sprintf("%d %s", img.pageNr, img.id)
			output, err = execCommandWithTimeout(DefaultCLITimeout, "pdfcpu", "images", "update", currentFile, blankImagePath, tempFile, pageIdArg)
		} else {
			// Clean up temp files
			for _, tf := range tempFiles {
				os.Remove(tf)
			}
			return fmt.Errorf("cannot identify image for removal: missing object number and page/ID")
		}

		if err != nil {
			// Clean up temp files
			for _, tf := range tempFiles {
				os.Remove(tf)
			}
			outputStr := string(output)
			if outputStr != "" {
				return fmt.Errorf("failed to remove image (obj:%s, page:%d, id:%s): %v\nOutput: %s", img.objNr, img.pageNr, img.id, err, outputStr)
			}
			return fmt.Errorf("failed to remove image (obj:%s, page:%d, id:%s): %v", img.objNr, img.pageNr, img.id, err)
		}

		// If this wasn't the last image, update currentFile for next iteration
		if i < len(imagesToRemove)-1 {
			currentFile = tempFile
		}
	}

	// Clean up intermediate temp files
	for _, tf := range tempFiles {
		if tf != outFile {
			os.Remove(tf)
		}
	}

	return nil
}

// createBlankImage creates a 1x1 transparent PNG file
func createBlankImage(dir string) (string, error) {
	// Create a 1x1 transparent PNG
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("failed to encode blank image: %v", err)
	}

	// Save to file
	filename := filepath.Join(dir, "blank_1x1.png")
	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create blank image file: %v", err)
	}
	defer file.Close()

	_, err = file.Write(buf.Bytes())
	if err != nil {
		os.Remove(filename)
		return "", fmt.Errorf("failed to write blank image: %v", err)
	}

	return filename, nil
}
