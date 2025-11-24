package pdf

import (
	"fmt"
	"strings"
)

// RemovePagesFromPDF removes specified pages from a PDF file using pdfcpu CLI
func RemovePagesFromPDF(inFile, outFile, pages string) error {
	// Parse page specification
	pageNumbers, err := ParsePageSpecifier(pages)
	if err != nil {
		return err
	}

	// Validate page numbers against PDF page count before processing
	totalPages, err := getPageCount(inFile)
	if err != nil {
		return fmt.Errorf("failed to get page count: %v", err)
	}

	if err := ValidatePageNumbers(pageNumbers, totalPages); err != nil {
		return err
	}

	// Convert page numbers to strings for CLI
	pageStrs := make([]string, len(pageNumbers))
	for i, p := range pageNumbers {
		pageStrs[i] = fmt.Sprintf("%d", p)
	}

	// pdfcpu pages remove command: pdfcpu pages remove -p pages -- inFile outFile
	pagesArg := strings.Join(pageStrs, ",")
	output, err := execCommandWithTimeout(DefaultCLITimeout, "pdfcpu", "pages", "remove", "-p", pagesArg, "--", inFile, outFile)
	if err != nil {
		return fmt.Errorf("pdfcpu remove failed: %v", err)
	}
	
	_ = output // Suppress unused variable warning

	return nil
}
