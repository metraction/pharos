package mappers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/flow"
	"gopkg.in/yaml.v3"
)

/*
Creates appender by reading file on creation moment.
*/
func NewAppendFile[T any](file string) flow.MapFunction[T, map[string]interface{}] {
	// Extract the filename without extension and directories
	baseName := filepath.Base(file)
	// Remove extension
	fileKey := strings.TrimSuffix(baseName, filepath.Ext(baseName))

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

	return func(data T) map[string]interface{} {
		// Create the result map with data and YAML content
		result := map[string]interface{}{
			"payload": data,
			"meta": map[string]interface{}{
				fileKey: yamlContent,
			},
		}
		return result
	}
}

func NewPureHbs[T any, R any](rule string) flow.MapFunction[T, R] {
	rp, err := NewPolicy[T](rule)
	if err != nil {
		log.Fatalf("NewRiskPolicy returned error: %v", err)
	}

	return func(data T) R {
		buf, err := rp.Evaluate(data)
		// Create a zero value of R to check its type
		var r R
		if err != nil {
			log.Printf("Evaluate returned error: %v", err)
			return r // Return zero value of R on error
		}

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

func NewDebug(config string) flow.MapFunction[map[string]interface{}, map[string]interface{}] {
	return func(data map[string]interface{}) map[string]interface{} {
		yamlContent, _ := toYaml(data)
		fmt.Printf("Debug %s:\n%s", config, yamlContent)
		return data
	}
}

func ToMap(data any) map[string]interface{} {
	// Convert any to map[string]interface{} using JSON marshaling
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

func ToWrappedResult(result model.PharosScanResult) WrappedResult {
	return WrappedResult{
		Result:  result,
		Context: ToMap(result),
	}
}

func ToUnWrappedResult(name string) func(result WrappedResult) model.PharosScanResult {
	logger := logging.NewLogger("info", "component", "pkg.mappers")

	return func(result WrappedResult) model.PharosScanResult {

		item := result.Result
		if len(item.Image.ContextRoots) == 0 {
			logger.Warn().Msg("No context roots found in scan result, I cannot add anything.")
			return item
		}
		if len(item.Image.ContextRoots) != 1 {
			logger.Warn().Msg("Wow, this should not happen either, only one context root is expected, but found multiple.")
			return item
		}
		item.Image.ContextRoots[0].Contexts = append(item.Image.ContextRoots[0].Contexts, model.Context{
			ContextRootKey: item.Image.ContextRoots[0].Key,
			ImageId:        item.Image.ImageId,
			Owner:          name,
			UpdatedAt:      time.Now(),
			Data:           result.Context,
		})
		logger.Debug().Str("ImageId", item.Image.ImageId).Str("urltocheck", "http://localhost:8080/api/pharosimagemeta/contexts/"+item.Image.ImageId).Msg("Context added")

		return item
	}
}
