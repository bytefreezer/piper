// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"fmt"
	"sync"
	"time"

	"github.com/bytefreezer/goodies/log"
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
	lastReload    time.Time
	mutex         sync.RWMutex
	enricherReady bool
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
// Note: This feature requires PostgreSQL database access which is not available
// in Control API mode. Enricher data must be pre-loaded or fetched from Control API.
func (f *EnricherFilter) loadEnricherDataFromDB(ctx *FilterContext) error {
	// Enricher data loading from database is not available without PostgreSQL
	// This is expected in Control API mode - enricher data should be loaded differently
	log.Debugf("Enricher %s: database loading not available (using Control API mode)", f.EnricherID)
	return nil // Don't fail, just skip database loading
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
