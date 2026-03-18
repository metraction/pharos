package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"

	"github.com/spf13/cobra"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

var edPayloadFile string
var edScriptFile string

var enricherDebuggerCmd = &cobra.Command{
	Use:   "enricher-debugger",
	Short: "Debug an enricher script against a JSON payload",
	Long:  `Load a JSON payload file and run an enricher script (.star or .yaegi) against it, printing the result.`,
	Run: func(cmd *cobra.Command, args []string) {
		if edPayloadFile == "" || edScriptFile == "" {
			fmt.Fprintln(os.Stderr, "error: --payload and --script are required")
			cmd.Usage()
			os.Exit(1)
		}

		m, err := edLoadJSON(edPayloadFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		var result map[string]any
		switch filepath.Ext(edScriptFile) {
		case ".star":
			result = edRunStar(edScriptFile, m)
		case ".yaegi":
			result = edRunYaegi(edScriptFile, m)
		default:
			fmt.Fprintf(os.Stderr, "unsupported script type: %s\n", filepath.Ext(edScriptFile))
			os.Exit(1)
		}

		fmt.Println("\nEnrich result:")
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error marshalling result: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(b))
	},
}

func edLoadJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	return result, nil
}

func edRunYaegi(scriptPath string, payload map[string]any) map[string]any {
	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols)

	_, err := i.EvalPath(scriptPath)
	if err != nil {
		log.Fatalf("Failed to execute yaegi script: %v", err)
	}

	enrichFunc, err := i.Eval("enrich")
	if err != nil {
		log.Fatalf("Function 'enrich' not found in yaegi script: %v", err)
	}

	results := enrichFunc.Call([]reflect.Value{reflect.ValueOf(payload)})
	if len(results) == 0 {
		log.Fatalf("enrich returned no results")
	}

	resultMap, ok := results[0].Interface().(map[string]interface{})
	if !ok {
		log.Fatalf("Expected map[string]interface{}, got %T", results[0].Interface())
	}
	return resultMap
}

func edRunStar(scriptPath string, payload map[string]any) map[string]any {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		log.Fatalf("Failed to read starlark script: %v", err)
	}

	thread := &starlark.Thread{
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}

	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, scriptPath, data, nil)
	if err != nil {
		log.Fatalf("Failed to execute starlark script: %v", err)
	}

	enrichFunc, ok := globals["enrich"]
	if !ok {
		log.Fatalf("Function 'enrich' not found in starlark script")
	}

	arg := edMapToStarlark(payload)
	val, err := starlark.Call(thread, enrichFunc, starlark.Tuple{arg}, nil)
	if err != nil {
		log.Fatalf("enrich call failed: %v", err)
	}

	result, ok := edStarlarkToMap(val)
	if !ok {
		log.Fatalf("enrich did not return a dict")
	}
	return result
}

func edMapToStarlark(m map[string]any) *starlarkstruct.Struct {
	sd := make(starlark.StringDict, len(m))
	for k, v := range m {
		sd[k] = edAnyToStarlark(v)
	}
	return starlarkstruct.FromStringDict(starlark.String("struct"), sd)
}

func edAnyToStarlark(v any) starlark.Value {
	if v == nil {
		return starlark.None
	}
	switch val := v.(type) {
	case bool:
		return starlark.Bool(val)
	case float64:
		return starlark.Float(val)
	case string:
		return starlark.String(val)
	case map[string]any:
		return edMapToStarlark(val)
	case []any:
		elems := make([]starlark.Value, len(val))
		for i, e := range val {
			elems[i] = edAnyToStarlark(e)
		}
		return starlark.NewList(elems)
	default:
		return starlark.String(fmt.Sprintf("%v", val))
	}
}

func edStarlarkToMap(v starlark.Value) (map[string]any, bool) {
	switch val := v.(type) {
	case *starlark.Dict:
		result := make(map[string]any, val.Len())
		for _, kv := range val.Items() {
			key, ok := starlark.AsString(kv[0])
			if !ok {
				continue
			}
			result[key] = edStarlarkToAny(kv[1])
		}
		return result, true
	case *starlarkstruct.Struct:
		result := make(map[string]any)
		for _, name := range val.AttrNames() {
			attr, err := val.Attr(name)
			if err != nil || attr == nil {
				continue
			}
			result[name] = edStarlarkToAny(attr)
		}
		return result, true
	}
	return nil, false
}

func edStarlarkToAny(v starlark.Value) any {
	switch val := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(val)
	case starlark.Int:
		n, _ := val.Int64()
		return n
	case starlark.Float:
		return float64(val)
	case starlark.String:
		return string(val)
	case *starlark.Dict:
		m, _ := edStarlarkToMap(val)
		return m
	case *starlarkstruct.Struct:
		m, _ := edStarlarkToMap(val)
		return m
	case *starlark.List:
		elems := make([]any, val.Len())
		for i := range elems {
			elems[i] = edStarlarkToAny(val.Index(i))
		}
		return elems
	default:
		return v.String()
	}
}

func init() {
	rootCmd.AddCommand(enricherDebuggerCmd)
	enricherDebuggerCmd.Flags().StringVarP(&edPayloadFile, "payload", "p", "", "Path to the JSON payload file (required)")
	enricherDebuggerCmd.Flags().StringVarP(&edScriptFile, "script", "s", "", "Path to the enricher script (.star or .yaegi) (required)")
}
