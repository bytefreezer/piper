// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package parsers

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"github.com/bytedance/sonic"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Cistern/sflow"
)

// CSVParser parses CSV data to NDJSON
type CSVParser struct {
	name      string
	delimiter rune
	headers   []string
}

func NewCSVParser(config map[string]interface{}) (Parser, error) {
	delimiter := ','
	if d, ok := config["delimiter"].(string); ok && len(d) > 0 {
		delimiter = rune(d[0])
	}

	return &CSVParser{
		name:      "csv-parser",
		delimiter: delimiter,
	}, nil
}

func (p *CSVParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = p.delimiter

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no CSV records found")
	}

	// Use first row as headers if not set
	if p.headers == nil && len(records) > 0 {
		p.headers = records[0]
		records = records[1:]
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no data records after headers")
	}

	// Parse first data record
	record := records[0]
	if len(record) != len(p.headers) {
		return nil, fmt.Errorf("column count mismatch: headers=%d, record=%d", len(p.headers), len(record))
	}

	result := make(map[string]interface{})
	for i, value := range record {
		result[p.headers[i]] = value
	}

	return result, nil
}

func (p *CSVParser) Name() string { return p.name }
func (p *CSVParser) Type() string { return "csv" }
func (p *CSVParser) Configure(config map[string]interface{}) error {
	if headers, ok := config["headers"].([]string); ok {
		p.headers = headers
	}
	return nil
}

// TSVParser parses Tab-separated values
type TSVParser struct {
	csvParser *CSVParser
}

func NewTSVParser(config map[string]interface{}) (Parser, error) {
	config["delimiter"] = "\t"
	csvParser, err := NewCSVParser(config)
	if err != nil {
		return nil, err
	}

	return &TSVParser{
		csvParser: csvParser.(*CSVParser),
	}, nil
}

func (p *TSVParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	return p.csvParser.Parse(ctx, data)
}

func (p *TSVParser) Name() string { return "tsv-parser" }
func (p *TSVParser) Type() string { return "tsv" }
func (p *TSVParser) Configure(config map[string]interface{}) error {
	return p.csvParser.Configure(config)
}

// ApacheLogParser parses Apache access logs
type ApacheLogParser struct {
	name  string
	regex *regexp.Regexp
}

func NewApacheLogParser(config map[string]interface{}) (Parser, error) {
	// Common Log Format pattern
	pattern := `^(\S+) \S+ \S+ \[([^\]]+)\] "([^"]*)" (\d+) (\S+)`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile Apache log regex: %w", err)
	}

	return &ApacheLogParser{
		name:  "apache-log-parser",
		regex: regex,
	}, nil
}

func (p *ApacheLogParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	line := strings.TrimSpace(string(data))
	matches := p.regex.FindStringSubmatch(line)
	if len(matches) < 6 {
		return nil, fmt.Errorf("line does not match Apache log format")
	}

	status, _ := strconv.Atoi(matches[4])
	size := matches[5]
	if size == "-" {
		size = "0"
	}

	result := map[string]interface{}{
		"remote_addr": matches[1],
		"timestamp":   matches[2],
		"request":     matches[3],
		"status":      status,
		"body_size":   size,
		"log_type":    "apache",
		"raw":         line,
		"parsed_time": time.Now().UTC().Format(time.RFC3339),
	}

	return result, nil
}

func (p *ApacheLogParser) Name() string                                  { return p.name }
func (p *ApacheLogParser) Type() string                                  { return "apache" }
func (p *ApacheLogParser) Configure(config map[string]interface{}) error { return nil }

// NginxLogParser parses Nginx access logs
type NginxLogParser struct {
	name  string
	regex *regexp.Regexp
}

func NewNginxLogParser(config map[string]interface{}) (Parser, error) {
	// Nginx default log format pattern
	pattern := `^(\S+) - \S+ \[([^\]]+)\] "([^"]*)" (\d+) (\d+) "([^"]*)" "([^"]*)"`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile Nginx log regex: %w", err)
	}

	return &NginxLogParser{
		name:  "nginx-log-parser",
		regex: regex,
	}, nil
}

func (p *NginxLogParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	line := strings.TrimSpace(string(data))
	matches := p.regex.FindStringSubmatch(line)
	if len(matches) < 8 {
		return nil, fmt.Errorf("line does not match Nginx log format")
	}

	status, _ := strconv.Atoi(matches[4])
	bodySize, _ := strconv.Atoi(matches[5])

	result := map[string]interface{}{
		"remote_addr":     matches[1],
		"timestamp":       matches[2],
		"request":         matches[3],
		"status":          status,
		"body_size":       bodySize,
		"http_referer":    matches[6],
		"http_user_agent": matches[7],
		"log_type":        "nginx",
		"raw":             line,
		"parsed_time":     time.Now().UTC().Format(time.RFC3339),
	}

	return result, nil
}

func (p *NginxLogParser) Name() string                                  { return p.name }
func (p *NginxLogParser) Type() string                                  { return "nginx" }
func (p *NginxLogParser) Configure(config map[string]interface{}) error { return nil }

// InfluxLineProtocolParser parses InfluxDB line protocol
type InfluxLineProtocolParser struct {
	name string
}

func NewInfluxLineProtocolParser(config map[string]interface{}) (Parser, error) {
	return &InfluxLineProtocolParser{
		name: "influx-parser",
	}, nil
}

func (p *InfluxLineProtocolParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	line := strings.TrimSpace(string(data))

	// Parse InfluxDB line protocol: measurement,tag1=value1,tag2=value2 field1=value1,field2=value2 timestamp
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid InfluxDB line protocol format")
	}

	// Parse measurement and tags
	measurementPart := parts[0]
	fieldsPart := parts[1]

	measurementAndTags := strings.Split(measurementPart, ",")
	measurement := measurementAndTags[0]

	result := map[string]interface{}{
		"measurement": measurement,
		"tags":        make(map[string]string),
		"fields":      make(map[string]interface{}),
		"log_type":    "influx",
		"raw":         line,
		"parsed_time": time.Now().UTC().Format(time.RFC3339),
	}

	// Parse tags
	tags := result["tags"].(map[string]string)
	for i := 1; i < len(measurementAndTags); i++ {
		tagParts := strings.SplitN(measurementAndTags[i], "=", 2)
		if len(tagParts) == 2 {
			tags[tagParts[0]] = tagParts[1]
		}
	}

	// Parse fields
	fields := result["fields"].(map[string]interface{})
	fieldPairs := strings.Split(fieldsPart, ",")
	for _, pair := range fieldPairs {
		fieldParts := strings.SplitN(pair, "=", 2)
		if len(fieldParts) == 2 {
			// Try to parse as number, fallback to string
			if val, err := strconv.ParseFloat(fieldParts[1], 64); err == nil {
				fields[fieldParts[0]] = val
			} else {
				fields[fieldParts[0]] = strings.Trim(fieldParts[1], `"`)
			}
		}
	}

	// Parse timestamp if present
	if len(parts) >= 3 {
		if ts, err := strconv.ParseInt(parts[2], 10, 64); err == nil {
			result["timestamp"] = ts
		}
	}

	return result, nil
}

func (p *InfluxLineProtocolParser) Name() string                                  { return p.name }
func (p *InfluxLineProtocolParser) Type() string                                  { return "influx" }
func (p *InfluxLineProtocolParser) Configure(config map[string]interface{}) error { return nil }

// CEFParser parses Common Event Format (CEF) logs
type CEFParser struct {
	name  string
	regex *regexp.Regexp
}

func NewCEFParser(config map[string]interface{}) (Parser, error) {
	// CEF format: CEF:Version|Device Vendor|Device Product|Device Version|Device Event Class ID|Name|Severity|Extension
	pattern := `^CEF:(\d+)\|([^|]*)\|([^|]*)\|([^|]*)\|([^|]*)\|([^|]*)\|([^|]*)\|(.*)$`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile CEF regex: %w", err)
	}

	return &CEFParser{
		name:  "cef-parser",
		regex: regex,
	}, nil
}

func (p *CEFParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	line := strings.TrimSpace(string(data))
	matches := p.regex.FindStringSubmatch(line)
	if len(matches) != 9 {
		return nil, fmt.Errorf("line does not match CEF format")
	}

	severity, _ := strconv.Atoi(matches[7])

	result := map[string]interface{}{
		"cef_version":    matches[1],
		"device_vendor":  matches[2],
		"device_product": matches[3],
		"device_version": matches[4],
		"event_class_id": matches[5],
		"name":           matches[6],
		"severity":       severity,
		"extension":      matches[8],
		"log_type":       "cef",
		"raw":            line,
		"parsed_time":    time.Now().UTC().Format(time.RFC3339),
	}

	// Parse extension fields
	if matches[8] != "" {
		extensions := make(map[string]string)
		// Simple key=value parsing for extensions
		extensionPairs := strings.Split(matches[8], " ")
		for _, pair := range extensionPairs {
			if keyValue := strings.SplitN(pair, "=", 2); len(keyValue) == 2 {
				extensions[keyValue[0]] = keyValue[1]
			}
		}
		result["extensions"] = extensions
	}

	return result, nil
}

func (p *CEFParser) Name() string                                  { return p.name }
func (p *CEFParser) Type() string                                  { return "cef" }
func (p *CEFParser) Configure(config map[string]interface{}) error { return nil }

// RawTextParser handles raw text files requiring custom parsing
type RawTextParser struct {
	name string
}

func NewRawTextParser(config map[string]interface{}) (Parser, error) {
	return &RawTextParser{
		name: "raw-text-parser",
	}, nil
}

func (p *RawTextParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	line := strings.TrimSpace(string(data))

	result := map[string]interface{}{
		"message":     line,
		"log_type":    "raw",
		"raw":         line,
		"parsed_time": time.Now().UTC().Format(time.RFC3339),
		"line_length": len(line),
	}

	// Try to detect if it looks like JSON
	if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
		var jsonData map[string]interface{}
		if err := sonic.Unmarshal([]byte(line), &jsonData); err == nil {
			result["detected_format"] = "json"
			result["json_data"] = jsonData
		}
	}

	return result, nil
}

func (p *RawTextParser) Name() string                                  { return p.name }
func (p *RawTextParser) Type() string                                  { return "raw" }
func (p *RawTextParser) Configure(config map[string]interface{}) error { return nil }

// SflowParser parses sFlow v5 packets to JSON format using the Cistern sFlow library
type SflowParser struct {
	name string
}

func NewSflowParser(config map[string]interface{}) (Parser, error) {
	return &SflowParser{
		name: "sflow-parser",
	}, nil
}

func (p *SflowParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	// Create decoder for sFlow v5 packets
	decoder := sflow.NewDecoder(bytes.NewReader(data))

	// Decode the sFlow datagram
	dgram, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode sFlow packet: %w", err)
	}

	result := map[string]interface{}{
		"log_type":      "sflow",
		"parsed_time":   time.Now().UTC().Format(time.RFC3339),
		"version":       dgram.Version,
		"agent_address": dgram.IpAddress.String(),
		"sub_agent_id":  dgram.SubAgentId,
		"sequence":      dgram.SequenceNumber,
		"uptime":        dgram.Uptime,
		"sample_count":  dgram.NumSamples,
		"samples":       []map[string]interface{}{},
	}

	// Process each sample in the datagram
	for _, sample := range dgram.Samples {
		sampleData := map[string]interface{}{
			"sample_type": sample.SampleType(),
		}

		switch s := sample.(type) {
		case *sflow.FlowSample:
			sampleData["type"] = "flow"
			sampleData["sequence"] = s.SequenceNum
			sampleData["source_id_type"] = s.SourceIdType
			sampleData["source_id_index"] = s.SourceIdIndexVal
			sampleData["sampling_rate"] = s.SamplingRate
			sampleData["sample_pool"] = s.SamplePool
			sampleData["drops"] = s.Drops
			sampleData["input_interface"] = s.Input
			sampleData["output_interface"] = s.Output
			sampleData["record_count"] = len(s.Records)

			// Process flow records
			records := []map[string]interface{}{}
			for _, record := range s.Records {
				recordData := p.parseRecord(record)
				if recordData != nil {
					records = append(records, recordData)
				}
			}
			sampleData["records"] = records

		case *sflow.CounterSample:
			sampleData["type"] = "counter"
			sampleData["sequence"] = s.SequenceNum
			sampleData["source_id_type"] = s.SourceIdType
			sampleData["source_id_index"] = s.SourceIdIndexVal
			sampleData["record_count"] = len(s.Records)

			// Process counter records
			records := []map[string]interface{}{}
			for _, record := range s.Records {
				recordData := p.parseRecord(record)
				if recordData != nil {
					records = append(records, recordData)
				}
			}
			sampleData["records"] = records
		}

		result["samples"] = append(result["samples"].([]map[string]interface{}), sampleData)
	}

	return result, nil
}

// parseRecord extracts data from sFlow records (both flow and counter records)
func (p *SflowParser) parseRecord(record sflow.Record) map[string]interface{} {
	recordData := map[string]interface{}{
		"record_type": record.RecordType(),
	}

	// Convert record to JSON for generic parsing
	// This is a simpler approach that handles all record types
	recordJSON, err := sonic.Marshal(record)
	if err == nil {
		var recordMap map[string]interface{}
		if sonic.Unmarshal(recordJSON, &recordMap) == nil {
			// Merge the record data
			for k, v := range recordMap {
				recordData[k] = v
			}
		}
	}

	// Add some enhanced parsing for common flow records
	recordStr := fmt.Sprintf("%T", record)
	recordData["record_type_name"] = recordStr

	return recordData
}

func (p *SflowParser) Name() string                                  { return p.name }
func (p *SflowParser) Type() string                                  { return "sflow" }
func (p *SflowParser) Configure(config map[string]interface{}) error { return nil }
