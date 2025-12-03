// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"fmt"
	"net"
	"time"

	"github.com/bytefreezer/goodies/log"
	"github.com/oschwald/geoip2-golang"
)

// GeoIPFilter adds geographic information for IP addresses
type GeoIPFilter struct {
	SourceField  string
	TargetField  string
	DatabasePath string
	Fields       []string
	db           *geoip2.Reader
}

// NewGeoIPFilterComplete creates a complete GeoIP filter
func NewGeoIPFilterComplete(config map[string]interface{}) (Filter, error) {
	filter := &GeoIPFilter{
		SourceField:  "ip_address",
		TargetField:  "geoip",
		DatabasePath: "/var/lib/geoip/GeoLite2-City.mmdb",
		Fields:       []string{"country_name", "city_name", "latitude", "longitude"},
	}

	// Parse source_field
	if sourceField, ok := config["source_field"].(string); ok {
		filter.SourceField = sourceField
	}

	// Parse target_field
	if targetField, ok := config["target_field"].(string); ok {
		filter.TargetField = targetField
	}

	// Parse database path
	if dbPath, ok := config["database"].(string); ok {
		filter.DatabasePath = dbPath
	}

	// Parse fields
	if fields, ok := config["fields"].([]interface{}); ok {
		filter.Fields = make([]string, 0, len(fields))
		for _, field := range fields {
			if fieldStr, ok := field.(string); ok {
				filter.Fields = append(filter.Fields, fieldStr)
			}
		}
	}

	// Open GeoIP database
	db, err := geoip2.Open(filter.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open GeoIP database at '%s': %w", filter.DatabasePath, err)
	}
	filter.db = db

	return filter, nil
}

// Type returns the filter type
func (f *GeoIPFilter) Type() string {
	return "geoip"
}

// Validate validates the filter configuration
func (f *GeoIPFilter) Validate(config map[string]interface{}) error {
	return nil
}

// Apply applies the GeoIP filter to a record
func (f *GeoIPFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Get source field value
	sourceValue, exists := record[f.SourceField]
	if !exists {
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Convert to string
	sourceStr, ok := sourceValue.(string)
	if !ok {
		log.Debugf("GeoIP filter: source field '%s' is not a string, skipping", f.SourceField)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Parse IP address
	ip := net.ParseIP(sourceStr)
	if ip == nil {
		log.Debugf("GeoIP filter: invalid IP address '%s', skipping", sourceStr)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Lookup IP in GeoIP database
	city, err := f.db.City(ip)
	if err != nil {
		log.Debugf("GeoIP filter: failed to lookup IP '%s': %v", sourceStr, err)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Build GeoIP result
	geoInfo := make(map[string]interface{})

	// Extract requested fields
	for _, field := range f.Fields {
		switch field {
		case "country_name":
			if city.Country.Names != nil {
				if name, ok := city.Country.Names["en"]; ok {
					geoInfo["country_name"] = name
				}
			}

		case "country_code", "country_iso_code":
			geoInfo["country_code"] = city.Country.IsoCode

		case "country_code2":
			geoInfo["country_code2"] = city.Country.IsoCode

		case "country_code3":
			// GeoIP2 uses ISO 3166-1 alpha-2, not alpha-3
			geoInfo["country_code3"] = city.Country.IsoCode

		case "city_name":
			if city.City.Names != nil {
				if name, ok := city.City.Names["en"]; ok {
					geoInfo["city_name"] = name
				}
			}

		case "latitude":
			geoInfo["latitude"] = city.Location.Latitude

		case "longitude":
			geoInfo["longitude"] = city.Location.Longitude

		case "location":
			geoInfo["location"] = map[string]interface{}{
				"lat": city.Location.Latitude,
				"lon": city.Location.Longitude,
			}

		case "timezone":
			geoInfo["timezone"] = city.Location.TimeZone

		case "continent_name":
			if city.Continent.Names != nil {
				if name, ok := city.Continent.Names["en"]; ok {
					geoInfo["continent_name"] = name
				}
			}

		case "continent_code":
			geoInfo["continent_code"] = city.Continent.Code

		case "region_name", "subdivision_name":
			if len(city.Subdivisions) > 0 && city.Subdivisions[0].Names != nil {
				if name, ok := city.Subdivisions[0].Names["en"]; ok {
					geoInfo["region_name"] = name
				}
			}

		case "region_code", "subdivision_code":
			if len(city.Subdivisions) > 0 {
				geoInfo["region_code"] = city.Subdivisions[0].IsoCode
			}

		case "postal_code", "zip_code":
			geoInfo["postal_code"] = city.Postal.Code

		case "metro_code":
			geoInfo["metro_code"] = city.Location.MetroCode

		case "accuracy_radius":
			geoInfo["accuracy_radius"] = city.Location.AccuracyRadius
		}
	}

	// Store GeoIP data
	record[f.TargetField] = geoInfo

	log.Debugf("GeoIP filter: enriched IP '%s' with %d fields", sourceStr, len(geoInfo))

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}
