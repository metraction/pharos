package mappers

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
)

type Header struct {
	Version string `yaml:"version"`
	Kind    string `yaml:"kind"`
}

// Policy represents a policy loaded from an HBS template file.
type Policy[T any] struct {
	tmpl *template.Template
}

/*
Analog of https://lodash.com/docs/4.17.15#map
At current moment mapping function is to pick field from struct
*/

var mapOperators = map[string]func(item reflect.Value, args ...string) interface{}{
	"field": func(item reflect.Value, args ...string) interface{} {
		// Handle struct case
		if item.Kind() == reflect.Struct {
			field := item.FieldByName(args[0])
			if !field.IsValid() {
				return nil
			}
			return field.Interface()
		}

		// Handle interface case
		if item.Kind() == reflect.Interface {
			// Try to convert to map[string]interface{}
			if m, ok := item.Interface().(map[string]interface{}); ok {
				if val, exists := m[args[0]]; exists {
					return val
				}
			}
		}

		return nil
	},
}

var filterOperators = map[string]func(item interface{}, fieldName string, criteria string) bool{
	"startsWith": func(item interface{}, fieldName string, criteria string) bool {
		// Handle struct case
		iv := reflect.ValueOf(item)
		if iv.Kind() == reflect.Struct {
			field := iv.FieldByName(fieldName)
			if !field.IsValid() {
				return false
			}

			// Convert field value to string
			var fieldValue string
			switch field.Kind() {
			case reflect.String:
				fieldValue = field.String()
			default:
				fieldValue = fmt.Sprintf("%v", field.Interface())
			}

			return strings.HasPrefix(fieldValue, criteria)
		}
		// Handle map case
		if m, ok := item.(map[string]interface{}); ok {
			if val, exists := m[fieldName]; exists {
				strVal := fmt.Sprintf("%v", val)
				return strings.HasPrefix(strVal, criteria)
			}
		}
		return false
	},
	"matchWildcard": func(item interface{}, fieldName string, criteria string) bool {
		// Get field value as string
		var pattern string
		iv := reflect.ValueOf(item)
		if iv.Kind() == reflect.Struct {
			field := iv.FieldByName(fieldName)
			if !field.IsValid() {
				return false
			}

			// Convert field value to string
			switch field.Kind() {
			case reflect.String:
				pattern = field.String()
			default:
				pattern = fmt.Sprintf("%v", field.Interface())
			}
		} else if m, ok := item.(map[string]interface{}); ok {
			if val, exists := m[fieldName]; exists {
				pattern = fmt.Sprintf("%v", val)
			} else {
				return false
			}
		} else {
			return false
		}

		// Check if pattern contains wildcard
		if strings.Contains(pattern, "%") {
			// Convert pattern to regex
			regexPattern := strings.ReplaceAll(pattern, ".", "\\.")
			regexPattern = strings.ReplaceAll(regexPattern, "%", ".*")
			regexPattern = "^" + regexPattern + "$"

			// Compile regex
			reg, err := regexp.Compile(regexPattern)
			if err != nil {
				return false
			}

			// Match criteria against pattern
			return reg.MatchString(criteria)
		}

		// No wildcard, use exact match
		return pattern == criteria
	},
}

func mapOperator(fn string, fieldName string, structs interface{}) ([]interface{}, error) {
	sv := reflect.ValueOf(structs)
	if sv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("expected a slice, got %T", structs)
	}
	result := make([]interface{}, sv.Len())
	for i := 0; i < sv.Len(); i++ {
		result[i] = mapOperators[fn](sv.Index(i), fieldName)
	}
	return result, nil
}

func filterOperator(fieldName string, fn string, criteria string, structs interface{}) ([]interface{}, error) {
	sv := reflect.ValueOf(structs)
	if sv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("expected a slice, got %T", structs)
	}
	var filtered []interface{}
	for i := 0; i < sv.Len(); i++ {
		item := sv.Index(i).Interface()
		if filterOperators[fn](item, fieldName, criteria) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func sum(nums []interface{}) int {
	var total int = 0
	for _, num := range nums {
		switch v := num.(type) {
		case int:
			total += v
		case float64:
			total += int(v)
		}
	}
	return total
}

func count(nums []interface{}) int {
	return len(nums)
}

func toYaml(v interface{}) (string, error) {
	out, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// NewRiskPolicy loads a policy template from the given file path and returns a RiskPolicy instance.
func NewPolicy[T any](templatePath string) (*Policy[T], error) {
	file, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}

	// Create a function map with Sprig functions and our custom functions
	funcMap := sprig.FuncMap()
	// Add our custom functions
	funcMap["sum"] = sum
	funcMap["count"] = count
	funcMap["map"] = mapOperator
	funcMap["filter"] = filterOperator
	funcMap["toYaml"] = toYaml
	tmpl, err := template.New("risk").Funcs(funcMap).Parse(string(file))
	if err != nil {
		return nil, err
	}
	return &Policy[T]{tmpl: tmpl}, nil
}

// Evaluate applies the RiskPolicy template to the given Image and returns the appropriate Risk structure based on version.
func (rp *Policy[T]) Evaluate(context any) (*bytes.Buffer, error) {

	// Convert Image to map[string]interface{} using JSON marshaling

	var buf bytes.Buffer
	if err := rp.tmpl.Execute(&buf, context); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return &buf, nil
}

func numberOutput(buf *bytes.Buffer) string {
	lines := strings.Split(buf.String(), "\n")
	numberedLines := make([]string, len(lines))

	// Calculate the width needed for line numbers to align all content
	lineNumWidth := len(fmt.Sprintf("%d", len(lines)))

	// Find the maximum line length to align comments (up to a reasonable limit)
	maxLineLength := 0
	for _, line := range lines {
		if len(line) > maxLineLength {
			maxLineLength = len(line)
		}
	}
	// Cap the max length to avoid excessive spacing
	if maxLineLength > 40 {
		maxLineLength = 40
	}

	for i, line := range lines {
		// Format line numbers as YAML comments to preserve the original YAML structure
		if strings.TrimSpace(line) == "" {
			// For empty lines, just add the comment
			numberedLines[i] = fmt.Sprintf("# Line %*d", lineNumWidth, i+1)
		} else {
			// For non-empty lines, preserve the original line and add the comment
			// Align all comments to the same position
			if len(line) > maxLineLength {
				// If line is longer than max, truncate it and add ellipsis
				truncated := line[:maxLineLength-3] + "..."
				numberedLines[i] = fmt.Sprintf("%-*s # Line %*d", maxLineLength, truncated, lineNumWidth, i+1)
			} else {
				// Otherwise pad with spaces to align all comments
				numberedLines[i] = fmt.Sprintf("%-*s # Line %*d", maxLineLength, line, lineNumWidth, i+1)
			}
		}
	}
	numberedContent := strings.Join(numberedLines, "\n")
	return numberedContent
}
