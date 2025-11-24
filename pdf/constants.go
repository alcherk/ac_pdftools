package pdf

const (
	// MinPageCoverageThreshold is the minimum percentage of pages (80%) for watermark detection
	MinPageCoverageThreshold = 0.8
	
	// BaseConfidence is the base confidence score for watermark detection
	BaseConfidence = 0.3
	
	// SizeBasedConfidenceBonus is added confidence for small images
	SizeBasedConfidenceBonus = 0.3
	
	// PositionBasedConfidenceBonus is added for images on first/last page
	PositionBasedConfidenceBonus = 0.2
	
	// MinWatermarkFileSizeKB is the minimum file size in KB for watermark detection (30KB)
	MinWatermarkFileSizeKB = 30
	
	// FullPageCoverageThreshold is 100% page coverage - image appears on all pages
	FullPageCoverageThreshold = 1.0
)

