package pipeline

import (
	"crypto/md5"  // #nosec G501 - MD5 used for data fingerprinting, not cryptographic security
	"crypto/sha1" // #nosec G505 - SHA1 used for data fingerprinting, not cryptographic security
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// FingerprintFilter generates event fingerprints/hashes for deduplication
type FingerprintFilter struct {
	SourceFields  []string
	TargetField   string
	Method        string
	KeySeparator  string
	Base64Encode  bool
	IncludeKeys   bool
}

// NewFingerprintFilter creates a new fingerprint filter
func NewFingerprintFilter(config map[string]interface{}) (Filter, error) {
	filter := &FingerprintFilter{
		SourceFields:  make([]string, 0),
		TargetField:   "fingerprint",
		Method:        "SHA256",
		KeySeparator:  "|",
		Base64Encode:  false,
		IncludeKeys:   false,
	}

	// Parse source_fields
	if sourceFields, ok := config["source_fields"].([]interface{}); ok {
		for _, field := range sourceFields {
			if fieldStr, ok := field.(string); ok {
				filter.SourceFields = append(filter.SourceFields, fieldStr)
			}
		}
	}

	// Parse target_field
	if targetField, ok := config["target_field"].(string); ok {
		filter.TargetField = targetField
	}

	// Parse method
	if method, ok := config["method"].(string); ok {
		filter.Method = strings.ToUpper(method)
	}

	// Parse key_separator
	if keySeparator, ok := config["key_separator"].(string); ok {
		filter.KeySeparator = keySeparator
	}

	// Parse base64encode
	if base64encode, ok := config["base64encode"].(bool); ok {
		filter.Base64Encode = base64encode
	}

	// Parse include_keys
	if includeKeys, ok := config["include_keys"].(bool); ok {
		filter.IncludeKeys = includeKeys
	}

	// Validate method
	validMethods := map[string]bool{
		"MD5":    true,
		"SHA1":   true,
		"SHA256": true,
		"SHA512": true,
	}
	if !validMethods[filter.Method] {
		return nil, fmt.Errorf("invalid hash method '%s', must be one of: MD5, SHA1, SHA256, SHA512", filter.Method)
	}

	return filter, nil
}

// Type returns the filter type
func (f *FingerprintFilter) Type() string {
	return "fingerprint"
}

// Validate validates the filter configuration
func (f *FingerprintFilter) Validate(config map[string]interface{}) error {
	if _, ok := config["source_fields"]; !ok {
		return fmt.Errorf("fingerprint filter requires 'source_fields' list")
	}
	return nil
}

// Apply applies the fingerprint filter to a record
func (f *FingerprintFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Build concatenated string from source fields
	var parts []string

	for _, fieldName := range f.SourceFields {
		if value, exists := record[fieldName]; exists {
			var strValue string

			if f.IncludeKeys {
				strValue = fmt.Sprintf("%s=%v", fieldName, value)
			} else {
				strValue = fmt.Sprintf("%v", value)
			}

			parts = append(parts, strValue)
		}
	}

	if len(parts) == 0 {
		log.Debugf("Fingerprint filter: no source fields found, skipping")
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Concatenate with separator
	concatenated := strings.Join(parts, f.KeySeparator)

	// Generate hash
	hash := f.generateHash(concatenated)

	// Encode if requested
	if f.Base64Encode {
		hash = base64.StdEncoding.EncodeToString([]byte(hash))
	}

	// Store fingerprint
	record[f.TargetField] = hash

	log.Debugf("Fingerprint filter: generated %s hash for %d fields", f.Method, len(parts))

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// generateHash generates a hash using the configured method
func (f *FingerprintFilter) generateHash(input string) string {
	var hashBytes []byte

	switch f.Method {
	case "MD5":
		hash := md5.Sum([]byte(input)) // #nosec G401 - MD5 used for data fingerprinting, not cryptographic security
		hashBytes = hash[:]

	case "SHA1":
		hash := sha1.Sum([]byte(input)) // #nosec G401 - SHA1 used for data fingerprinting, not cryptographic security
		hashBytes = hash[:]

	case "SHA256":
		hash := sha256.Sum256([]byte(input))
		hashBytes = hash[:]

	case "SHA512":
		hash := sha512.Sum512([]byte(input))
		hashBytes = hash[:]

	default:
		// Default to SHA256
		hash := sha256.Sum256([]byte(input))
		hashBytes = hash[:]
	}

	return hex.EncodeToString(hashBytes)
}
