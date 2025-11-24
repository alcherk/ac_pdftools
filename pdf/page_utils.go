package pdf

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ParsePageSpecifier parses a page specification string and returns a list of page numbers.
// Supports formats: "1", "1,3", "1-5", "1,3-5,7"
func ParsePageSpecifier(pages string) ([]int, error) {
	if pages == "" {
		return nil, fmt.Errorf("empty page specification")
	}

	// Remove all whitespace
	pages = regexp.MustCompile(`\s`).ReplaceAllString(pages, "")

	var pageList []int
	parts := strings.Split(pages, ",")

	for _, part := range parts {
		if strings.Contains(part, "-") {
			// Range like "1-5"
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}

			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid start page: %s", rangeParts[0])
			}

			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid end page: %s", rangeParts[1])
			}

			if start > end {
				return nil, fmt.Errorf("invalid range: start > end (%d > %d)", start, end)
			}

			for i := start; i <= end; i++ {
				pageList = append(pageList, i)
			}
		} else {
			// Single page like "3"
			pageNum, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid page number: %s", part)
			}
			pageList = append(pageList, pageNum)
		}
	}

	// Sort and remove duplicates
	sort.Ints(pageList)
	deduped := []int{}
	for i, page := range pageList {
		if i == 0 || page != pageList[i-1] {
			deduped = append(deduped, page)
		}
	}

	return deduped, nil
}

// ValidatePageNumbers checks if all page numbers are valid for a given total number of pages
func ValidatePageNumbers(pages []int, totalPages int) error {
	for _, page := range pages {
		if page < 1 {
			return fmt.Errorf("page numbers must be positive, got %d", page)
		}
		if page > totalPages {
			return fmt.Errorf("page %d exceeds total pages (%d)", page, totalPages)
		}
	}
	return nil
}
