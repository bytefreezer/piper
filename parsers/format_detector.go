package parsers

import (
	"path/filepath"
	"strings"
)

// FormatHint represents a data format hint extracted from filename
type FormatHint struct {
	Format    string   // The detected format
	TenantID  string   // Tenant identifier
	DatasetID string   // Dataset identifier
	Extension string   // File extension (gz, bz2, etc.)
	Basename  string   // Original basename without extension
	Binary    bool     // Whether this is a binary format
}

// FormatDetector detects data format from filename hints
type FormatDetector struct {
	// Map of hint string to format name
	hintMap map[string]string
	// Map of format to whether it's binary
	binaryFormats map[string]bool
}

// NewFormatDetector creates a new format detector with predefined mappings
func NewFormatDetector() *FormatDetector {
	detector := &FormatDetector{
		hintMap: make(map[string]string),
		binaryFormats: make(map[string]bool),
	}

	// Text-based formats
	detector.hintMap["ndjson"] = "ndjson"
	detector.hintMap["csv"] = "csv"
	detector.hintMap["tsv"] = "tsv"
	detector.hintMap["apache"] = "apache"
	detector.hintMap["nginx"] = "nginx"
	detector.hintMap["iis"] = "iis"
	detector.hintMap["squid"] = "squid"
	detector.hintMap["influx"] = "influx"
	detector.hintMap["prom"] = "prom"
	detector.hintMap["statsd"] = "statsd"
	detector.hintMap["graphite"] = "graphite"
	detector.hintMap["syslog"] = "syslog"
	detector.hintMap["cef"] = "cef"
	detector.hintMap["gelf"] = "gelf"
	detector.hintMap["leef"] = "leef"
	detector.hintMap["log"] = "log"
	detector.hintMap["fix"] = "fix"
	detector.hintMap["hl7"] = "hl7"
	detector.hintMap["raw"] = "raw"

	// Binary formats
	detector.hintMap["sflow"] = "sflow"
	detector.hintMap["netflow"] = "netflow"
	detector.hintMap["ipfix"] = "ipfix"

	// Mark binary formats
	detector.binaryFormats["sflow"] = true
	detector.binaryFormats["netflow"] = true
	detector.binaryFormats["ipfix"] = true

	return detector
}

// DetectFormat extracts format hint from filename
// Expected format: tenant--dataset--hint.gz or tenant--dataset--hint.extension
func (fd *FormatDetector) DetectFormat(filename string) (*FormatHint, error) {
	// Remove directory path, keep only filename
	basename := filepath.Base(filename)

	// Handle compressed files by removing compression extensions
	extension := ""
	cleanName := basename

	// Check for compression extensions
	if strings.HasSuffix(basename, ".gz") {
		extension = "gz"
		cleanName = strings.TrimSuffix(basename, ".gz")
	} else if strings.HasSuffix(basename, ".bz2") {
		extension = "bz2"
		cleanName = strings.TrimSuffix(basename, ".bz2")
	} else if strings.HasSuffix(basename, ".xz") {
		extension = "xz"
		cleanName = strings.TrimSuffix(basename, ".xz")
	}

	// Split by double dashes to extract tenant--dataset--hint
	parts := strings.Split(cleanName, "--")

	hint := &FormatHint{
		Basename:  basename,
		Extension: extension,
	}

	// Parse different filename patterns
	if len(parts) >= 3 {
		// Format: tenant--dataset--hint[.ext]
		hint.TenantID = parts[0]
		hint.DatasetID = parts[1]

		// Extract hint from remaining parts (could have multiple dashes)
		hintPart := strings.Join(parts[2:], "--")

		// Remove any remaining file extensions from hint
		if dotIndex := strings.LastIndex(hintPart, "."); dotIndex > 0 {
			hintPart = hintPart[:dotIndex]
		}

		// Look up format from hint
		if format, exists := fd.hintMap[hintPart]; exists {
			hint.Format = format
			hint.Binary = fd.binaryFormats[format]
		} else {
			// Default to raw if hint not recognized
			hint.Format = "raw"
			hint.Binary = false
		}
	} else if len(parts) >= 2 {
		// Format: tenant--dataset[.ext] - no hint, default to raw
		hint.TenantID = parts[0]
		hint.DatasetID = strings.Split(parts[1], ".")[0] // Remove extension
		hint.Format = "raw"
		hint.Binary = false
	} else {
		// Single filename, try to detect from extension
		hint.TenantID = "unknown"
		hint.DatasetID = "unknown"

		// Try to detect format from file extension
		if strings.HasSuffix(cleanName, ".json") {
			hint.Format = "ndjson"
		} else if strings.HasSuffix(cleanName, ".csv") {
			hint.Format = "csv"
		} else if strings.HasSuffix(cleanName, ".log") {
			hint.Format = "log"
		} else {
			hint.Format = "raw"
		}
		hint.Binary = false
	}

	return hint, nil
}

// GetSupportedFormats returns a list of all supported format hints
func (fd *FormatDetector) GetSupportedFormats() []string {
	formats := make([]string, 0, len(fd.hintMap))
	for hint := range fd.hintMap {
		formats = append(formats, hint)
	}
	return formats
}

// IsBinaryFormat checks if a format is binary
func (fd *FormatDetector) IsBinaryFormat(format string) bool {
	return fd.binaryFormats[format]
}

// AddFormatMapping adds a new format mapping
func (fd *FormatDetector) AddFormatMapping(hint, format string, isBinary bool) {
	fd.hintMap[hint] = format
	fd.binaryFormats[format] = isBinary
}