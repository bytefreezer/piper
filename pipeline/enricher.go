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

	"github.com/n0needt0/bytefreezer-piper/storage"
	"github.com/n0needt0/go-goodies/log"
)

// EnricherFilter enriches events with data from customer-uploaded lookup tables
type EnricherFilter struct {
	EnricherID      string
	EnricherName    string
	TenantID        string
	MatchFields     map[string]string // incoming field -> enricher column mapping
	TargetField     string
	MultipleMatches string // "first", "array", "ignore"
	CacheTTL        time.Duration

	// Runtime data
	header       []string
	data         [][]string
	index        map[string][]int // index column value -> row indices
	indexColumns []string
	lastReload   time.Time
	mutex        sync.RWMutex
}

// NewEnricherFilter creates a new enricher filter
func NewEnricherFilter(config map[string]interface{}) (Filter, error) {
	filter := &EnricherFilter{
		MatchFields:     make(map[string]string),
		MultipleMatches: "first",
		CacheTTL:        5 * time.Minute,
	}

	// Parse enricher_id
	if enricherID, ok := config["enricher_id"].(string); ok {
		filter.EnricherID = enricherID
	} else {
		return nil, fmt.Errorf("enricher filter requires 'enricher_id' parameter")
	}

	// Parse enricher_name (optional)
	if enricherName, ok := config["enricher_name"].(string); ok {
		filter.EnricherName = enricherName
	}

	// Parse tenant_id (required for database fetch)
	if tenantID, ok := config["tenant_id"].(string); ok {
		filter.TenantID = tenantID
	} else {
		return nil, fmt.Errorf("enricher filter requires 'tenant_id' parameter")
	}

	// Parse match_fields
	if matchFields, ok := config["match_fields"].(map[string]interface{}); ok {
		for incomingField, enricherColumn := range matchFields {
			if enricherColStr, ok := enricherColumn.(string); ok {
				filter.MatchFields[incomingField] = enricherColStr
			}
		}
	} else {
		return nil, fmt.Errorf("enricher filter requires 'match_fields' parameter")
	}

	if len(filter.MatchFields) == 0 {
		return nil, fmt.Errorf("enricher filter requires at least one match field")
	}

	// Parse target_field
	if targetField, ok := config["target_field"].(string); ok {
		filter.TargetField = targetField
	} else {
		filter.TargetField = "enriched_data"
	}

	// Parse multiple_matches
	if multipleMatches, ok := config["multiple_matches"].(string); ok {
		filter.MultipleMatches = multipleMatches
	}

	// Parse cache_ttl
	if cacheTTL, ok := config["cache_ttl"].(float64); ok {
		filter.CacheTTL = time.Duration(cacheTTL) * time.Second
	} else if cacheTTLInt, ok := config["cache_ttl"].(int); ok {
		filter.CacheTTL = time.Duration(cacheTTLInt) * time.Second
	}

	// Note: enricher data will be loaded on first Apply() call with database access

	return filter, nil
}

// Type returns the filter type
func (f *EnricherFilter) Type() string {
	return "enricher"
}

// Validate validates the filter configuration
func (f *EnricherFilter) Validate(config map[string]interface{}) error {
	if _, ok := config["enricher_id"].(string); !ok {
		return fmt.Errorf("'enricher_id' parameter is required")
	}
	if _, ok := config["tenant_id"].(string); !ok {
		return fmt.Errorf("'tenant_id' parameter is required")
	}
	if _, ok := config["match_fields"].(map[string]interface{}); !ok {
		return fmt.Errorf("'match_fields' parameter is required")
	}
	return nil
}

// Apply applies the enricher filter to a record
func (f *EnricherFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Check if we need to reload data
	if time.Since(f.lastReload) > f.CacheTTL {
		if err := f.loadEnricherDataFromDB(ctx); err != nil {
			log.Warnf("Failed to reload enricher data: %v", err)
		}
	}

	// Lock for reading
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	// Check if data is loaded
	if len(f.header) == 0 {
		log.Debugf("Enricher %s has no data loaded, skipping", f.EnricherID)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Build lookup key from incoming record
	lookupKey := f.buildLookupKey(record)
	if lookupKey == "" {
		// No match fields found in record
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

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

	// Apply enrichment based on multiple_matches strategy
	switch f.MultipleMatches {
	case "first":
		// Add first match
		enrichedData := f.rowToMap(f.data[rowIndices[0]])
		record[f.TargetField] = enrichedData

	case "array":
		// Add all matches as array
		enrichedArray := make([]map[string]interface{}, 0, len(rowIndices))
		for _, idx := range rowIndices {
			enrichedData := f.rowToMap(f.data[idx])
			enrichedArray = append(enrichedArray, enrichedData)
		}
		record[f.TargetField] = enrichedArray

	case "ignore":
		// Multiple matches found, ignore (don't enrich)
		if len(rowIndices) > 1 {
			return &FilterResult{
				Record:   record,
				Skip:     false,
				Applied:  false,
				Duration: time.Since(start),
			}, nil
		}
		// Single match, add it
		enrichedData := f.rowToMap(f.data[rowIndices[0]])
		record[f.TargetField] = enrichedData

	default:
		// Default to first
		enrichedData := f.rowToMap(f.data[rowIndices[0]])
		record[f.TargetField] = enrichedData
	}

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

	// Fetch enricher binary data from database
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fileData, err := stateManager.GetEnricherData(dbCtx, f.TenantID, f.EnricherID)
	if err != nil {
		log.Warnf("Enricher data not available in database: %v", err)
		return nil // Don't fail, just log
	}

	if len(fileData) == 0 {
		log.Warnf("Enricher %s has no data", f.EnricherID)
		return nil
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

	// Determine index columns from match fields
	indexColumns := make([]string, 0)
	for _, enricherCol := range f.MatchFields {
		indexColumns = append(indexColumns, enricherCol)
	}

	// Build index
	index := make(map[string][]int)
	for i, row := range data {
		key := f.buildRowKey(row, header, indexColumns)
		if key != "" {
			index[key] = append(index[key], i)
		}
	}

	// Update filter data
	f.header = header
	f.data = data
	f.index = index
	f.indexColumns = indexColumns
	f.lastReload = time.Now()

	log.Infof("Loaded enricher %s from database: %d rows, %d unique keys", f.EnricherID, len(data), len(index))

	return nil
}

// buildLookupKey builds a lookup key from incoming record
func (f *EnricherFilter) buildLookupKey(record map[string]interface{}) string {
	parts := make([]string, 0, len(f.MatchFields))

	for incomingField := range f.MatchFields {
		value, exists := record[incomingField]
		if !exists {
			return ""
		}
		parts = append(parts, fmt.Sprintf("%v", value))
	}

	return strings.Join(parts, "|||")
}

// buildRowKey builds an index key from a row
func (f *EnricherFilter) buildRowKey(row []string, header []string, indexColumns []string) string {
	parts := make([]string, 0, len(indexColumns))

	for _, indexCol := range indexColumns {
		// Find column index
		colIdx := -1
		for i, col := range header {
			if col == indexCol {
				colIdx = i
				break
			}
		}

		if colIdx == -1 || colIdx >= len(row) {
			return ""
		}

		parts = append(parts, row[colIdx])
	}

	return strings.Join(parts, "|||")
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
