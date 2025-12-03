// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"fmt"
	"time"

	"github.com/bytefreezer/goodies/log"
	"github.com/ua-parser/uap-go/uaparser"
)

// UserAgentFilter parses user agent strings into structured data
type UserAgentFilter struct {
	SourceField string
	TargetField string
	parser      *uaparser.Parser
}

// NewUserAgentFilter creates a new user agent filter
func NewUserAgentFilter(config map[string]interface{}) (Filter, error) {
	filter := &UserAgentFilter{
		SourceField: "user_agent",
		TargetField: "ua",
	}

	// Parse source_field
	if sourceField, ok := config["source_field"].(string); ok {
		filter.SourceField = sourceField
	}

	// Parse target_field
	if targetField, ok := config["target_field"].(string); ok {
		filter.TargetField = targetField
	}

	// Initialize parser (uses embedded regexes)
	parser, err := uaparser.New("")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user agent parser: %w", err)
	}
	filter.parser = parser

	return filter, nil
}

// Type returns the filter type
func (f *UserAgentFilter) Type() string {
	return "useragent"
}

// Validate validates the filter configuration
func (f *UserAgentFilter) Validate(config map[string]interface{}) error {
	return nil
}

// Apply applies the user agent filter to a record
func (f *UserAgentFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
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
		log.Debugf("UserAgent filter: source field '%s' is not a string, skipping", f.SourceField)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Parse user agent
	client := f.parser.Parse(sourceStr)

	// Build result structure
	uaInfo := make(map[string]interface{})

	// Browser/User Agent info
	if client.UserAgent != nil {
		uaInfo["name"] = client.UserAgent.Family
		if client.UserAgent.Major != "" {
			version := client.UserAgent.Major
			if client.UserAgent.Minor != "" {
				version += "." + client.UserAgent.Minor
				if client.UserAgent.Patch != "" {
					version += "." + client.UserAgent.Patch
				}
			}
			uaInfo["version"] = version
			uaInfo["major"] = client.UserAgent.Major
			if client.UserAgent.Minor != "" {
				uaInfo["minor"] = client.UserAgent.Minor
			}
			if client.UserAgent.Patch != "" {
				uaInfo["patch"] = client.UserAgent.Patch
			}
		}
	}

	// Operating System info
	if client.Os != nil {
		uaInfo["os"] = client.Os.Family
		if client.Os.Major != "" {
			osVersion := client.Os.Major
			if client.Os.Minor != "" {
				osVersion += "." + client.Os.Minor
				if client.Os.Patch != "" {
					osVersion += "." + client.Os.Patch
				}
			}
			uaInfo["os_version"] = osVersion
			uaInfo["os_major"] = client.Os.Major
			if client.Os.Minor != "" {
				uaInfo["os_minor"] = client.Os.Minor
			}
		}
	}

	// Device info
	if client.Device != nil {
		if client.Device.Family != "" && client.Device.Family != "Other" {
			uaInfo["device"] = client.Device.Family
		} else {
			// Infer device type from OS
			if client.Os != nil {
				switch client.Os.Family {
				case "Android", "iOS", "Windows Phone", "BlackBerry OS":
					uaInfo["device"] = "Mobile"
				case "Windows", "Mac OS X", "Linux", "Ubuntu", "Fedora":
					uaInfo["device"] = "Desktop"
				default:
					uaInfo["device"] = "Other"
				}
			}
		}

		if client.Device.Brand != "" {
			uaInfo["device_brand"] = client.Device.Brand
		}
		if client.Device.Model != "" {
			uaInfo["device_model"] = client.Device.Model
		}
	}

	// Store parsed data
	record[f.TargetField] = uaInfo

	log.Debugf("UserAgent filter: parsed user agent from '%s'", f.SourceField)

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}
