package api

import "time"

const (
	// FileCleanupDelay is the delay before cleaning up temp files after response is sent
	FileCleanupDelay = 2 * time.Second
	
	// AnalysisCleanupDelay is the delay before cleaning up analysis temp files
	AnalysisCleanupDelay = 1 * time.Second
	
	// DefaultFilePermissions for temp directory creation
	DefaultFilePermissions = 0755
)

