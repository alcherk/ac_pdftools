package pdf

import (
	"fmt"
)

// ResavePDF optimizes and compresses a PDF file using pdfcpu CLI
func ResavePDF(inFile, outFile string) error {
	output, err := execCommandWithTimeout(DefaultCLITimeout, "pdfcpu", "optimize", inFile, outFile)
	if err != nil {
		return fmt.Errorf("pdfcpu optimize failed: %v", err)
	}
	
	// Log output only if there's something meaningful (optional)
	_ = output // Suppress unused variable warning

	return nil
}
