# PDF Editor

A Go-based web service for editing PDF files with a simple web interface.

## Features

- **Resave PDF**: Optimize and compress PDF files with quality preservation
- **Remove Pages**: Delete specified pages using flexible syntax (e.g., "1,3,5-7") with automatic validation
- **Advanced Watermark Detection**: Intelligent multi-criteria watermark detection including:
  - Full-page watermark detection (appears on all pages with same prefix, size ≥30KB)
  - Repeating watermark detection (appears on 80%+ of pages)
  - Pattern-based detection (same prefix, same file size)
  - Confidence scoring (0-100%)
- **Selective Element Removal**: Review and choose which detected elements to remove
- **Web UI**: Clean, responsive web interface for easy file uploads and operations
- **REST API**: Programmatic access to all PDF editing functions
- **Docker Support**: Production-ready containerized deployment with pdfcpu CLI included
- **Health Checks**: `/health` endpoint for container orchestration
- **Production Features**: Graceful shutdown, timeout handling, secure file operations

## Requirements

- Go 1.21 or later
- pdfcpu CLI tool (automatically installed in Docker, or install manually for local development)
- Docker (optional, for containerized deployment)

## Quick Start

### Local Development

1. Clone the repository and navigate to the project directory
2. Install pdfcpu CLI (required for PDF operations):
   - **macOS**: `brew install pdfcpu`
   - **Linux**: Download from [pdfcpu releases](https://github.com/pdfcpu/pdfcpu/releases)
   - **Windows**: Download from [pdfcpu releases](https://github.com/pdfcpu/pdfcpu/releases)
3. Install dependencies:
   ```bash
   go mod tidy
   ```
4. Run the server:
   ```bash
   go run main.go
   ```
   The server will check for pdfcpu availability on startup and exit with a clear error if not found.
5. Open your browser and visit `http://localhost:8080`

### Docker Deployment

1. Build the Docker image:
   ```bash
   docker build -t pdf-editor .
   ```
   The Dockerfile automatically installs pdfcpu CLI and sets up a non-root user for security.
2. Run the container:
   ```bash
   docker run -p 8080:8080 pdf-editor
   ```
   Or with custom environment variables:
   ```bash
   docker run -p 8080:8080 \
     -e PORT=8080 \
     -e MAX_FILE_SIZE=10485760 \
     -e TEMP_DIR=/tmp \
     pdf-editor
   ```
3. Access the web interface at `http://localhost:8080`
4. Check health status: `http://localhost:8080/health`

## API Endpoints

### GET /health
Health check endpoint for container orchestration and monitoring.

**Response**: JSON with service status
```json
{
  "status": "healthy",
  "service": "pdf_editor"
}
```

### POST /api/pdf/upload
Upload a PDF file to the server.

**Request**: Multipart form data with `pdf` field
**Response**: JSON with upload details
**Validation**: File size limits, PDF header validation, filename sanitization

### POST /api/pdf/resave
Re-save and optimize a PDF file using pdfcpu CLI.

**Request**: Multipart form data with `pdf` file
**Response**: Processed PDF file download
**Timeout**: 30 seconds

### POST /api/pdf/remove-pages
Remove specified pages from a PDF with automatic validation.

**Request**: Multipart form data with:
- `pdf`: PDF file
- `pages`: Page specification (e.g., "1,3,5-7")

**Response**: Processed PDF file download
**Validation**: Validates page numbers against total page count before processing  
**Timeout**: 30 seconds

### POST /api/pdf/remove-elements
Remove overlay elements (watermarks, images) from a PDF.

**Request**: Multipart form data with:
- `pdf`: PDF file
- `type`: Element type ("watermark" or "image")

**Response**: Processed PDF file download
**Timeout**: 30 seconds

### POST /api/pdf/analyze-watermarks
Analyze PDF for potential watermark candidates with intelligent detection.

**Request**: Multipart form data with `pdf` file
**Response**: JSON with analysis results including:
- Total pages
- Image candidates with confidence scores (0-100%)
- Text candidates (extensible)
- Recommendations for removal

**Detection Features**:
- Full-page watermarks: Images appearing on ALL pages with same prefix and size ≥30KB (95% confidence)
- Repeating watermarks: Images appearing on 80%+ of pages with pattern matching
- Same-prefix grouping: Groups images by name prefix (e.g., "Image-1", "Image-2" → prefix "Image")
- File size filtering: Only considers images ≥30KB for watermark detection

**Timeout**: 60 seconds

### POST /api/pdf/remove-selected-elements
Remove selected watermark elements (foundation implemented).

**Request**: Multipart form data with:
- `pdf`: PDF file
- `elements`: Comma-separated list of element IDs

**Response**: Processed PDF file download

## Advanced Watermark Management

Access the dedicated watermark management interface at:
- **URL**: `/watermarks`
- **Features**:
  - **Enhanced Detection Algorithms**:
    - Full-page watermark detection (100% coverage)
    - Repeating watermark detection (80%+ coverage)
    - Same-prefix pattern recognition
    - File size-based filtering (≥30KB)
  - Visual candidate review with detailed metadata including:
    - Confidence scores (0-100%)
    - Page coverage percentage
    - File size and dimensions
    - Prefix and signature information
  - Selective removal (foundation implemented)
  - Progress tracking and user feedback

This specialized interface provides advanced watermark analysis beyond the basic operations available on the main page.

## Project Structure

```
pdf_editor/
├── main.go                    # Application entry point
├── api/
│   ├── routes.go             # API routes configuration
│   └── handlers.go           # HTTP request handlers
├── api/                      # API layer
│   ├── handlers.go           # HTTP request handlers with security features
│   ├── routes.go             # API routes configuration
│   └── constants.go          # API-level constants
├── pdf/                      # PDF processing functions
│   ├── analyze.go            # Advanced watermark detection system
│   ├── cli_utils.go          # CLI operation utilities with timeouts
│   ├── constants.go          # PDF processing constants
│   ├── page_utils.go         # Page specification parsing utilities
│   ├── remove_elements.go    # Element removal operations
│   ├── remove_pages.go       # Page removal with pdfcpu CLI
│   └── resave.go             # PDF optimization functionality
├── static/                   # Static web assets
│   ├── styles.css            # CSS styles
│   └── app.js                # Frontend JavaScript
├── templates/
│   └── index.html            # Main web interface template
├── Dockerfile                # Docker container definition
└── README.md                 # This file
```

## Web Interface Usage

1. Open the web interface in your browser
2. Click "Choose File" and select a PDF file
3. Click "Upload PDF" to upload the file
4. Once uploaded, you'll see available operations:
   - **Analyze Watermarks**: Automatically detect potential watermarks with confidence scoring
   - **Resave PDF**: Optimize and compress the file
   - **Remove Pages**: Delete specific pages (supports ranges like "1,3,5-7")
   - **Remove Elements**: Bulk removal by element type

### Intelligent Watermark Analysis Workflow
1. After upload, click "Analyze Watermarks"
2. Wait for analysis to complete (up to 60 seconds for large PDFs)
3. Review detected watermark candidates with confidence scores:
   - **Very High (95%+)**: Full-page watermarks appearing on all pages with same prefix and size ≥30KB
   - **High (80-95%)**: Repeating watermarks appearing on 80%+ of pages
   - **Medium (50-80%)**: Potential watermarks with moderate page coverage
   - **Low (<50%)**: Unlikely watermarks or isolated images
4. Each candidate shows detailed metadata:
   - Page coverage percentage
   - File size in KB
   - Image dimensions and color space
   - Prefix pattern (if applicable)
5. Select specific elements to remove using checkboxes
6. Click "Remove Selected Elements" for precise watermark removal

### Traditional Operations
For bulk operations without analysis:
- **Bulk Watermark Removal**: Removes all detected watermarks automatically
- **Page Removal**: Specify pages to delete (e.g., "1,3,5-7" removes pages 1, 3, 5, 6, and 7)

## Development

### Adding New PDF Operations

1. Add new handler functions in `api/handlers.go`
2. Add corresponding routes in `api/routes.go`
3. Update the web interface in `templates/index.html` and `static/app.js`

### PDF Processing Implementation

The implementation uses pdfcpu CLI for all PDF operations:
- **Resave**: Uses `pdfcpu optimize` command
- **Remove Pages**: Uses `pdfcpu pages remove` with validation
- **Remove Elements**: Uses `pdfcpu watermarks remove`
- **Analyze**: Uses `pdfcpu info` and `pdfcpu images list`

All operations include:
- Timeout handling (30s default, 60s for analysis)
- Error handling with user-friendly messages
- Automatic file cleanup
- Page count validation

### Configuration

The server supports environment variables for configuration:

- `PORT`: Server port (default: `8080`)
- `MAX_FILE_SIZE`: Maximum upload file size in bytes (default: `10485760` = 10MB)
- `TEMP_DIR`: Temporary directory for file processing (default: `./temp`)

Example:
```bash
PORT=9000 MAX_FILE_SIZE=52428800 TEMP_DIR=/tmp/pdf_temp go run main.go
```

### Security Features

- **Filename Sanitization**: Prevents path traversal attacks
- **Unique File IDs**: Prevents file collisions in concurrent requests
- **Configurable Temp Directories**: Uses environment-configured temp paths
- **Request Timeouts**: Prevents hanging operations
- **File Cleanup**: Automatic cleanup of temporary files
- **Non-root Docker User**: Runs as non-root user in containers

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## License

This project is open source. See LICENSE file for details.
