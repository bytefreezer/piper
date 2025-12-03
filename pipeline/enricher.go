// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bytefreezer/goodies/log"
	"github.com/bytefreezer/piper/storage"
)

// EnricherFilter enriches events with data from customer-uploaded lookup tables
type EnricherFilter struct {
	EnricherID   string
	EnricherName string
	TenantID     string
	SourceField  string // field in event to match against enricher's index column
	TargetField  string // where to store enriched data (defaults to src.{source_field}-{enricher_name_sanitized})
	CacheTTL     time.Duration

	// Runtime data
	header        []string
	data          [][]string
	index         map[string][]int // index column value -> row indices
	indexColumn   string           // the enricher's index column (first index column)
	lastReload    time.Time
	mutex         sync.RWMutex
	enricherReady bool
}

// sanitizeName replaces all non-alphanumeric characters with dashes
func sanitizeName(name string) string {
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}
	return strings.ToLower(result.String())
}

// NewEnricherFilter creates a new enricher filter
func NewEnricherFilter(config map[string]interface{}) (Filter, error) {
	filter := &EnricherFilter{
		CacheTTL: 5 * time.Minute,
	}

	// Parse enricher_id (also accept "enrichment" as alias)
	if enricherID, ok := config["enricher_id"].(string); ok {
		filter.EnricherID = enricherID
	} else if enricherID, ok := config["enrichment"].(string); ok {
		filter.EnricherID = enricherID
	} else {
		return nil, fmt.Errorf("enricher filter requires 'enricher_id' parameter")
	}

	// Parse enricher_name (optional, used for target_field default)
	if enricherName, ok := config["enricher_name"].(string); ok {
		filter.EnricherName = enricherName
	}

	// Parse tenant_id (required for database fetch)
	if tenantID, ok := config["tenant_id"].(string); ok {
		filter.TenantID = tenantID
	} else {
		return nil, fmt.Errorf("enricher filter requires 'tenant_id' parameter")
	}

	// Parse source_field (required - field in event to match against enricher's index column)
	if sourceField, ok := config["source_field"].(string); ok {
		filter.SourceField = sourceField
	} else {
		return nil, fmt.Errorf("enricher filter requires 'source_field' parameter")
	}

	// Parse target_field (optional - defaults to src.{source_field}-{enricher_name_sanitized})
	if targetField, ok := config["target_field"].(string); ok {
		filter.TargetField = targetField
	}
	// target_field default is set in loadEnricherDataFromDB when we know the enricher name

	// Parse cache_ttl
	if cacheTTL, ok := config["cache_ttl"].(float64); ok {
		filter.CacheTTL = time.Duration(cacheTTL) * time.Second
	} else if cacheTTLInt, ok := config["cache_ttl"].(int); ok {
		filter.CacheTTL = time.Duration(cacheTTLInt) * time.Second
	}

	return filter, nil
}

// Type returns the filter type
func (f *EnricherFilter) Type() string {
	return "enricher"
}

// Validate validates the filter configuration
func (f *EnricherFilter) Validate(config map[string]interface{}) error {
	_, hasEnricherID := config["enricher_id"].(string)
	_, hasEnrichment := config["enrichment"].(string)
	if !hasEnricherID && !hasEnrichment {
		return fmt.Errorf("'enricher_id' or 'enrichment' parameter is required")
	}
	if _, ok := config["tenant_id"].(string); !ok {
		return fmt.Errorf("'tenant_id' parameter is required")
	}
	if _, ok := config["source_field"].(string); !ok {
		return fmt.Errorf("'source_field' parameter is required")
	}
	return nil
}

// Apply applies the enricher filter to a record
func (f *EnricherFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Check if we need to reload data
	if !f.enricherReady || time.Since(f.lastReload) > f.CacheTTL {
		if err := f.loadEnricherDataFromDB(ctx); err != nil {
			log.Warnf("Failed to reload enricher data: %v", err)
		}
	}

	// Lock for reading
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	// Check if data is loaded
	if !f.enricherReady || len(f.header) == 0 {
		log.Debugf("Enricher %s has no data loaded, skipping", f.EnricherID)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Get source field value from record
	sourceValue, exists := record[f.SourceField]
	if !exists {
		// Source field not in record
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Convert source value to string for lookup
	lookupKey := fmt.Sprintf("%v", sourceValue)

	// Lookup matching rows
	rowIndices, found := f.index[lookupKey]
	if !found || len(rowIndices) == 0 {
		// No matches found
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Add first match to target field
	enrichedData := f.rowToMap(f.data[rowIndices[0]])
	record[f.TargetField] = enrichedData

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// loadEnricherDataFromDB loads enricher data from database
func (f *EnricherFilter) loadEnricherDataFromDB(ctx *FilterContext) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Get state manager from context
	if ctx == nil || ctx.StateManager == nil {
		return fmt.Errorf("no state manager available")
	}

	stateManager, ok := ctx.StateManager.(*storage.PostgreSQLStateManager)
	if !ok {
		return fmt.Errorf("state manager is not PostgreSQL type")
	}

	// Fetch enricher metadata and binary data from database
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	enricherMeta, fileData, err := stateManager.GetEnricherWithData(dbCtx, f.TenantID, f.EnricherID)
	if err != nil {
		log.Warnf("Enricher data not available in database: %v", err)
		return nil // Don't fail, just log
	}

	if len(fileData) == 0 {
		log.Warnf("Enricher %s has no data", f.EnricherID)
		return nil
	}

	// Set enricher name if not already set
	if f.EnricherName == "" && enricherMeta != nil {
		f.EnricherName = enricherMeta.Name
	}

	// Set default target field if not specified
	// Format: src.{source_field}-{enricher_name_sanitized}
	if f.TargetField == "" {
		sanitizedName := sanitizeName(f.EnricherName)
		f.TargetField = fmt.Sprintf("src.%s-%s", f.SourceField, sanitizedName)
	}

	// Get index column from enricher metadata
	var indexColumn string
	if enricherMeta != nil && len(enricherMeta.IndexColumns) > 0 {
		indexColumn = enricherMeta.IndexColumns[0] // Use first index column
	}

	// Parse JSON lines format from binary data
	reader := bytes.NewReader(fileData)
	scanner := bufio.NewScanner(reader)

	// First line is header
	if !scanner.Scan() {
		return fmt.Errorf("empty enricher data")
	}

	var header []string
	if err := json.Unmarshal(scanner.Bytes(), &header); err != nil {
		return fmt.Errorf("failed to parse header: %w", err)
	}

	// If no index column from metadata, use first column
	if indexColumn == "" && len(header) > 0 {
		indexColumn = header[0]
	}

	// Find index column position in header
	indexColIdx := -1
	for i, col := range header {
		if col == indexColumn {
			indexColIdx = i
			break
		}
	}

	if indexColIdx == -1 {
		return fmt.Errorf("index column %s not found in enricher data", indexColumn)
	}

	// Read data rows
	data := make([][]string, 0)
	for scanner.Scan() {
		var row []string
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			log.Warnf("Failed to parse row: %v", err)
			continue
		}
		data = append(data, row)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading data: %w", err)
	}

	// Build index using the index column
	index := make(map[string][]int)
	for i, row := range data {
		if indexColIdx < len(row) {
			key := row[indexColIdx]
			if key != "" {
				index[key] = append(index[key], i)
			}
		}
	}

	// Update filter data
	f.header = header
	f.data = data
	f.index = index
	f.indexColumn = indexColumn
	f.lastReload = time.Now()
	f.enricherReady = true

	log.Infof("Loaded enricher %s from database: %d rows, %d unique keys, index column: %s, target field: %s",
		f.EnricherID, len(data), len(index), indexColumn, f.TargetField)

	return nil
}

// rowToMap converts a row to a map using header
func (f *EnricherFilter) rowToMap(row []string) map[string]interface{} {
	result := make(map[string]interface{})

	for i, col := range f.header {
		if i < len(row) {
			result[col] = row[i]
		}
	}

	return result
}
