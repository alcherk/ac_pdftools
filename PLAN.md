# PDF Editor Implementation Plan

This document outlines the detailed plan for implementing core PDF editing features in the pdf_editor project.

## Implementation Plan for Core PDF Editing Features

### Phase 1: Infrastructure and Setup
1. **Select and Integrate PDF Library**
   - Research Go PDF libraries: pdfcpu (recommended in README), QPDF-go, PDFbox via JNI, unidoc
   - Add pdfcpu dependency: `go get github.com/pdfcpu/pdfcpu/pkg/api`
   - Create PDF processing module in `pdf/` directory
   - Implement basic PDF validation function
   - Handle library-specific error types and conversions

2. **Fix Frontend Upload Mechanism**
   - Modify operation forms to send actual file data instead of paths
   - Store uploaded file in browser memory/session storage after initial upload
   - Update JS to attach file to each operation FormData
   - Test file upload flow for all operations

3. **Add Error Handling and Validation**
   - Implement proper PDF file validation (check PDF header, basic structure)
   - Add file size enforcement in handlers
   - Create standardized error response format
   - Add timeout handling for long-running operations

### Phase 2: Core Operations Implementation

#### Operation A: PDF Resave (Optimize/Compress)
1. Create `pdf/resave.go` module
2. Implement resave function using pdfcpu API:
   ```go
   func ResavePDF(inFile, outFile string) error
   ```
3. Add compression options (configurable quality levels)
4. Update `HandleResave` to call real processing function
5. Test with various PDF files (size reduction verification)

#### Operation B: Remove Pages
1. Create page parsing utility function:
   ```go
   func ParsePageSpecifier(pages string) ([]int, error)
   ```
   - Support formats: "1", "1,3", "1-5", "1,3-5,7"
   - Validate page ranges against PDF total pages
2. Create `pdf/remove_pages.go` module
3. Implement page removal using pdfcpu
4. Update `HandleRemovePages` to call processing function
5. Add validation for page existence and overlapping ranges
6. Test edge cases (remove all pages, invalid ranges, etc.)

#### Operation C: Remove Overlay Elements (Watermarks/Images)
1. Create element types enum (watermark, image)
2. Research pdfcpu capabilities for element removal:
   - Watermarks: text-based overlays
   - Images: embedded raster graphics
3. Implement removal logic in `pdf/remove_elements.go`
4. Add confidence levels/thresholds for automatic detection
5. Consider adding manual element selection via page coordinates
6. Update `HandleRemoveElements` implementation
7. Test with sample PDFs containing various overlays

### Known Issues & Bugs

#### Bug: Repeating Unwanted Elements Removal Failure 🔴 **PRIORITY**
**Status**: Open / In Progress  
**Severity**: High - Core functionality affected

**Description**:
Repeating unwanted elements cannot be properly removed when:
- The detected prefix is "unknown" or empty
- The image_id metadata is missing or doesn't match actual PDF image IDs
- The signature pattern contains invalid dimensions (e.g., "0x0") due to parsing issues
- Multiple matching strategies fail to find corresponding images in the PDF

**Error Symptoms**:
- Warning: "Cannot find any occurrences for candidate repeating_unwanted_element_0x0__Dev (image_id: , prefix: unknown)"
- Error: "no matching images found for selected IDs. The images may be repeating watermarks that appear on multiple pages"
- Fallback to pdfcpu watermark/stamp removal also fails with: "pdfcpu: no watermarks found"

**Root Causes**:
1. **Prefix Extraction Failure**: When image IDs don't match expected patterns (e.g., no dash/underscore), prefix extraction returns "unknown"
2. **Dimension Parsing Issues**: Image dimensions may be parsed as "0x0" from pdfcpu output, causing signature mismatches
3. **Metadata Inconsistency**: Stored image_id in candidate metadata may not match actual image IDs in PDF (different format or encoding)
4. **Signature-Based Matching Incomplete**: Current signature matching doesn't fully re-analyze PDF to extract all matching images by signature pattern

**Current Workaround**:
- System attempts multiple matching strategies (exact ID, prefix-based, signature-based, case-insensitive)
- Falls back to pdfcpu watermark/stamp removal (only works for pdfcpu-generated watermarks)
- User receives error message indicating removal failure

**Proposed Solutions**:
1. **Improve Prefix Extraction**:
   - Enhance `extractIdPrefix()` to handle more image ID patterns
   - Fallback to first N characters when no delimiter found
   - Case-insensitive prefix matching

2. **Fix Dimension Parsing**:
   - Improve pdfcpu output parsing to correctly extract width/height
   - Handle edge cases where dimensions may be in different format
   - Validate parsed dimensions before using in signatures

3. **Enhanced Signature Matching**:
   - Re-analyze PDF images during removal to build signature map
   - Match by signature components separately (dimensions, colorspace, size)
   - Use fuzzy matching for signature components when exact match fails

4. **Alternative Removal Strategy**:
   - Store all image occurrences during analysis phase
   - Include list of all (page, object, id) tuples in candidate metadata
   - Use stored occurrences directly during removal instead of re-matching

**Testing Requirements**:
- Test with PDFs containing repeating unwanted elements with various ID formats
- Test with images that have unusual naming patterns
- Test with images where prefix extraction fails
- Test with images where dimensions are 0x0 or incorrectly parsed
- Verify all occurrences are found and removed across all pages

**Related Files**:
- `pdf/remove_elements.go` - Main removal logic
- `pdf/analyze.go` - Image analysis and metadata extraction
- `api/handlers.go` - API endpoint handling removal requests

### Phase 3: Bug Fixes and Critical Issues

1. **Fix Repeating Unwanted Elements Removal Bug** 🔴 **PRIORITY**
   - Fix prefix extraction for edge cases
   - Improve dimension parsing from pdfcpu output
   - Enhance signature-based matching logic
   - Store all image occurrences in candidate metadata
   - Test with various PDFs containing repeating unwanted elements

### Phase 4: Quality Assurance and Production Readiness

1. **Add Comprehensive Testing**
   - Unit tests for PDF operations
   - Integration tests for API endpoints
   - Frontend E2E tests with Selenium/Puppeteer
   - Memory leak tests for large files
   - Regression tests for unwanted elements removal bug fix

2. **Performance Optimization**
   - Implement file size limits per operation type
   - Add processing timeouts
   - Memory usage monitoring and cleanup
   - Consider streaming for large files if necessary

3. **Security Enhancements**
   - Server-side PDF validation (not just MIME type)
   - Implement file quarantine/temp file security
   - Add rate limiting for API endpoints
   - Remove any potential file inclusion vulnerabilities

4. **Web Interface Improvements**
   - Add operation progress indicators
   - Show PDF preview thumbnails if possible
   - Add operation history/results display
   - Improve error messages and user feedback

### Phase 5: Additional Features and Polish

1. **Advanced Features**
   - Merge PDFs
   - Rotate pages
   - Add watermarks/text
   - Split PDF into individual pages

2. **Configuration and Deployment**
   - Add environment-based configuration
   - Improve Docker for production (health checks, proper user/non-root)
   - Add logging configurations
   - Performance benchmarking

3. **Documentation Updates**
   - Update README with new capabilities
   - Add API documentation (OpenAPI/Swagger)
   - Include usage examples and best practices

### Estimated Timeline and Dependencies
- **Phase 1**: 1-2 weeks (library research and fixes) ✅ COMPLETED
- **Phase 2**: 2-3 weeks (core operations) ✅ COMPLETED
- **Phase 3**: 1-2 weeks (bug fixes and critical issues) 🔴 **IN PROGRESS**
- **Phase 4**: 1-2 weeks (testing and security)
- **Phase 5**: 1 week (polish, optional)

**Dependencies Required**: `github.com/pdfcpu/pdfcpu/pkg/api` (current recommendation), testing frameworks like `testify`, potentially additional libraries for image processing if needed.

