# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o pdf_editor .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl
WORKDIR /root/

# Install pdfcpu CLI binary
# Download pdfcpu v0.7.4 (latest stable) for Linux amd64
RUN curl -L https://github.com/pdfcpu/pdfcpu/releases/download/v0.7.4/pdfcpu_0.7.4_Linux_x86_64.tar.gz -o /tmp/pdfcpu.tar.gz && \
    tar -xzf /tmp/pdfcpu.tar.gz -C /tmp && \
    mv /tmp/pdfcpu /usr/local/bin/pdfcpu && \
    chmod +x /usr/local/bin/pdfcpu && \
    rm /tmp/pdfcpu.tar.gz

# Copy the binary from builder stage
COPY --from=builder /app/pdf_editor .

# Create temp directory for file processing and set permissions
RUN mkdir -p ./temp && \
    chmod 755 ./temp

# Create non-root user for security
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /root

# Expose port
EXPOSE 8080

# Switch to non-root user
USER appuser

# Run the application
CMD ["./pdf_editor"]
