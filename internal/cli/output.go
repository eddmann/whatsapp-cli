package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
)

// Format represents the output format
type Format string

const (
	FormatJSON  Format = "json"
	FormatJSONL Format = "jsonl"
	FormatCSV   Format = "csv"
	FormatTSV   Format = "tsv"
	FormatHuman Format = "human"
)

// IsValid checks if a format string is valid
func (f Format) IsValid() bool {
	switch f {
	case FormatJSON, FormatJSONL, FormatCSV, FormatTSV, FormatHuman:
		return true
	}
	return false
}

// OutputOptions controls output behavior
type OutputOptions struct {
	Format   Format
	Fields   []string // Field names to include (empty = all)
	NoHeader bool     // Skip header row for CSV/TSV
}

// Validate checks if the options are valid
func (o OutputOptions) Validate() error {
	if !o.Format.IsValid() {
		return fmt.Errorf("invalid format %q, valid formats: json, jsonl, csv, tsv, human", o.Format)
	}
	return nil
}

// output prints data in the specified format
func output(data any, opts OutputOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	switch opts.Format {
	case FormatJSON:
		return outputJSON(data, opts.Fields)
	case FormatJSONL:
		return outputJSONL(data, opts.Fields)
	case FormatCSV:
		return outputDelimited(data, ',', opts.Fields, opts.NoHeader)
	case FormatTSV:
		return outputDelimited(data, '\t', opts.Fields, opts.NoHeader)
	case FormatHuman:
		return outputHuman(data, opts.Fields)
	default:
		return outputJSON(data, opts.Fields)
	}
}

// outputJSON prints data as formatted JSON
func outputJSON(data any, fields []string) error {
	data = filterFields(data, fields)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// outputJSONL prints data as JSON Lines (one JSON object per line)
func outputJSONL(data any, fields []string) error {
	v := derefValue(reflect.ValueOf(data))
	if !v.IsValid() {
		return nil
	}

	// For slices/arrays, output each element on its own line
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		enc := json.NewEncoder(os.Stdout)
		for i := 0; i < v.Len(); i++ {
			item := filterFields(v.Index(i).Interface(), fields)
			if err := enc.Encode(item); err != nil {
				return fmt.Errorf("encode item %d: %w", i, err)
			}
		}
		return nil
	}

	// For single objects, output as one line
	data = filterFields(data, fields)
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(data)
}

// outputDelimited prints data as CSV or TSV using encoding/csv
func outputDelimited(data any, delimiter rune, fields []string, noHeader bool) error {
	headers, rows := extractTableData(data, fields, formatCSVValue)
	if len(rows) == 0 {
		return nil
	}

	w := csv.NewWriter(os.Stdout)
	w.Comma = delimiter

	if !noHeader && len(headers) > 0 {
		if err := w.Write(headers); err != nil {
			return fmt.Errorf("write header: %w", err)
		}
	}

	for i, row := range rows {
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write row %d: %w", i, err)
		}
	}

	w.Flush()
	return w.Error()
}

// outputHuman prints data in human-readable format
func outputHuman(data any, fields []string) error {
	v := derefValue(reflect.ValueOf(data))
	if !v.IsValid() {
		fmt.Println("(nil)")
		return nil
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return outputTable(data, fields)
	case reflect.Struct, reflect.Map:
		return outputKeyValue(data, fields)
	default:
		fmt.Println(data)
		return nil
	}
}

// outputTable prints a slice as a table
func outputTable(data any, fields []string) error {
	headers, rows := extractTableData(data, fields, formatHumanValue)
	if len(rows) == 0 {
		fmt.Println("(no results)")
		return nil
	}

	// Uppercase headers for human display
	for i, h := range headers {
		headers[i] = strings.ToUpper(h)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)
	table.AppendBulk(rows)
	table.Render()
	return nil
}

// outputKeyValue prints a struct or map as key-value pairs
func outputKeyValue(data any, fields []string) error {
	v := derefValue(reflect.ValueOf(data))
	if !v.IsValid() {
		return nil
	}

	fieldSet := makeFieldSet(fields)

	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		var pairs []struct{ name, value string }

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !isExportedField(field) {
				continue
			}
			name := getFieldName(field)
			if len(fieldSet) > 0 && !fieldSet[name] {
				continue
			}
			pairs = append(pairs, struct{ name, value string }{
				name:  name,
				value: formatHumanValue(v.Field(i)),
			})
		}

		maxLen := 0
		for _, p := range pairs {
			if len(p.name) > maxLen {
				maxLen = len(p.name)
			}
		}
		for _, p := range pairs {
			fmt.Printf("%-*s  %s\n", maxLen, p.name+":", p.value)
		}

	case reflect.Map:
		maxLen := 0
		for _, key := range v.MapKeys() {
			keyStr := fmt.Sprint(key.Interface())
			if len(keyStr) > maxLen {
				maxLen = len(keyStr)
			}
		}
		for _, key := range v.MapKeys() {
			keyStr := fmt.Sprint(key.Interface())
			if len(fieldSet) > 0 && !fieldSet[keyStr] {
				continue
			}
			fmt.Printf("%-*s  %s\n", maxLen, keyStr+":", formatHumanValue(v.MapIndex(key)))
		}
	}
	return nil
}

// ============================================================================
// Reflection Helpers
// ============================================================================

// derefValue unwraps pointer and interface values
func derefValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

// isExportedField returns true if the struct field is exported
func isExportedField(f reflect.StructField) bool {
	return f.PkgPath == ""
}

// getFieldName returns the json tag name or lowercased field name
func getFieldName(f reflect.StructField) string {
	name := f.Tag.Get("json")
	if name == "" || name == "-" {
		return strings.ToLower(f.Name)
	}
	// Remove options like ",omitempty"
	if idx := strings.Index(name, ","); idx != -1 {
		name = name[:idx]
	}
	return name
}

// makeFieldSet creates a set of field names for filtering
func makeFieldSet(fields []string) map[string]bool {
	if len(fields) == 0 {
		return nil
	}
	set := make(map[string]bool, len(fields))
	for _, f := range fields {
		set[strings.TrimSpace(f)] = true
	}
	return set
}

// extractTableData extracts headers and rows from slice data
func extractTableData(data any, fields []string, formatter func(reflect.Value) string) ([]string, [][]string) {
	v := derefValue(reflect.ValueOf(data))
	if !v.IsValid() {
		return nil, nil
	}

	// Wrap single objects in a slice
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		v = reflect.ValueOf([]any{data})
	}

	if v.Len() == 0 {
		return nil, nil
	}

	first := derefValue(v.Index(0))
	if !first.IsValid() || first.Kind() != reflect.Struct {
		// Fallback for non-struct types
		var rows [][]string
		for i := 0; i < v.Len(); i++ {
			rows = append(rows, []string{formatter(v.Index(i))})
		}
		return nil, rows
	}

	// Build headers and field indices
	t := first.Type()
	fieldSet := makeFieldSet(fields)
	var headers []string
	var indices []int

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !isExportedField(field) {
			continue
		}
		name := getFieldName(field)
		if len(fieldSet) > 0 && !fieldSet[name] {
			continue
		}
		headers = append(headers, name)
		indices = append(indices, i)
	}

	// Build rows
	var rows [][]string
	for i := 0; i < v.Len(); i++ {
		elem := derefValue(v.Index(i))
		if !elem.IsValid() {
			continue
		}
		var row []string
		for _, idx := range indices {
			row = append(row, formatter(elem.Field(idx)))
		}
		rows = append(rows, row)
	}

	return headers, rows
}

// filterFields filters struct/slice data to only include specified fields
func filterFields(data any, fields []string) any {
	if len(fields) == 0 {
		return data
	}

	v := derefValue(reflect.ValueOf(data))
	if !v.IsValid() {
		return data
	}

	fieldSet := makeFieldSet(fields)

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		var result []map[string]any
		for i := 0; i < v.Len(); i++ {
			item := filterStructToMap(derefValue(v.Index(i)), fieldSet)
			if item != nil {
				result = append(result, item)
			}
		}
		return result
	case reflect.Struct:
		return filterStructToMap(v, fieldSet)
	}

	return data
}

// filterStructToMap converts a struct to a map with only specified fields
func filterStructToMap(v reflect.Value, fieldSet map[string]bool) map[string]any {
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	result := make(map[string]any)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !isExportedField(field) {
			continue
		}
		name := getFieldName(field)
		if !fieldSet[name] {
			continue
		}
		result[name] = v.Field(i).Interface()
	}

	return result
}

// ============================================================================
// Value Formatters
// ============================================================================

// valueFormatterConfig configures how values are formatted
type valueFormatterConfig struct {
	TimeFormat string // Time format string (e.g., "2006-01-02 15:04" or time.RFC3339)
	EmptyValue string // String to use for nil/empty values (e.g., "-" or "")
	TrueValue  string // String for true booleans (e.g., "yes" or "true")
	FalseValue string // String for false booleans (e.g., "no" or "false")
	MaxLength  int    // Max string length before truncation (0 = no truncation)
}

var (
	humanFormatterConfig = valueFormatterConfig{
		TimeFormat: "2006-01-02 15:04",
		EmptyValue: "-",
		TrueValue:  "yes",
		FalseValue: "no",
		MaxLength:  50,
	}
	csvFormatterConfig = valueFormatterConfig{
		TimeFormat: time.RFC3339,
		EmptyValue: "",
		TrueValue:  "true",
		FalseValue: "false",
		MaxLength:  0, // No truncation for CSV
	}
)

// formatValue formats a value according to the config
func formatValue(v reflect.Value, cfg valueFormatterConfig) string {
	v = derefValue(v)
	if !v.IsValid() {
		return cfg.EmptyValue
	}

	val := v.Interface()

	switch t := val.(type) {
	case time.Time:
		if t.IsZero() {
			return cfg.EmptyValue
		}
		return t.Format(cfg.TimeFormat)
	case *time.Time:
		if t == nil || t.IsZero() {
			return cfg.EmptyValue
		}
		return t.Format(cfg.TimeFormat)
	case bool:
		if t {
			return cfg.TrueValue
		}
		return cfg.FalseValue
	case string:
		if t == "" {
			return cfg.EmptyValue
		}
		// Truncate if configured
		if cfg.MaxLength > 0 && len(t) > cfg.MaxLength {
			return t[:cfg.MaxLength-3] + "..."
		}
		return t
	case nil:
		return cfg.EmptyValue
	default:
		str := fmt.Sprint(val)
		if str == "" {
			return cfg.EmptyValue
		}
		return str
	}
}

// formatHumanValue formats a value for human-readable display
func formatHumanValue(v reflect.Value) string {
	return formatValue(v, humanFormatterConfig)
}

// formatCSVValue formats a value for CSV/TSV output
func formatCSVValue(v reflect.Value) string {
	return formatValue(v, csvFormatterConfig)
}
