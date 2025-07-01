package mappers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/reugn/go-streams/flow"
	"gopkg.in/yaml.v3"
)

func NewAppendFile[T any](file string) flow.MapFunction[T, map[string]interface{}] {
	// Extract the filename without extension and directories
	baseName := filepath.Base(file)
	// Remove extension
	fileKey := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	return func(data T) map[string]interface{} {
		// Read the YAML file content
		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Failed to read file %s: %v", file, err)
		}

		// Parse YAML content into map[string]interface{}
		var yamlContent map[string]interface{}
		if err := yaml.Unmarshal(content, &yamlContent); err != nil {
			log.Fatalf("Failed to unmarshal YAML from file %s: %v", file, err)
		}

		// Create the result map with data and YAML content
		result := map[string]interface{}{
			"data":  data,
			fileKey: yamlContent,
		}

		return result
	}
}

func NewPureHbs[T any, R any](rule string) flow.MapFunction[T, R] {
	rp, err := NewPolicy[T](rule)
	if err != nil {
		log.Fatalf("NewRiskPolicy returned error: %v", err)
	}

	return func(img T) R {
		buf, err := rp.Evaluate(img)
		if err != nil {
			log.Fatalf("Evaluate returned error: %v", err)
		}

		// Create a zero value of R to check its type
		var r R

		// Use reflection to check if R is a string type
		rType := reflect.TypeOf(r)
		if rType != nil && rType.Kind() == reflect.String {
			// If R is a string, convert buffer to string and return it
			strValue := buf.String() // Using buf.String() instead of string(buf.Bytes()) as per linting suggestion
			return any(strValue).(R)
		}

		// For non-string types, use standard unmarshaling
		if err := yaml.Unmarshal(buf.Bytes(), &r); err != nil {
			// Add line numbers to the buffer content for better error reporting
			numberedContent := numberOutput(buf)

			log.Printf("Error unmarshalling %s result: %v \nFailling yaml with line numbers:\n%s", rule, err, numberedContent)
			return r
		}
		return r
	}
}

func NewDebug() flow.MapFunction[map[string]interface{}, map[string]interface{}] {
	return func(data map[string]interface{}) map[string]interface{} {
		yamlContent, _ := toYaml(data)
		fmt.Printf("Debug:\n%s", yamlContent)
		return data
	}
}

func NewMapOfMaps() flow.MapFunction[any, map[string]interface{}] {
	return func(data any) map[string]interface{} {
		// Convert Image to map[string]interface{} using JSON marshaling
		imageJSON, err := json.Marshal(data)
		if err != nil {
			return nil
		}

		var result map[string]interface{}
		if err = json.Unmarshal(imageJSON, &result); err != nil {
			return nil
		}
		return result
	}
}
