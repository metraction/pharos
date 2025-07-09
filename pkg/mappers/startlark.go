package mappers

import (
	"fmt"
	"log"

	"github.com/reugn/go-streams/flow"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func NewStarlark(rule string) flow.MapFunction[map[string]interface{}, map[string]interface{}] {
	thread := &starlark.Thread{Name: "my thread"}

	return func(item map[string]interface{}) map[string]interface{} {
		globals, err := starlark.ExecFile(thread, rule, nil, nil)
		if err != nil {
			log.Fatalf("Evaluate returned error: %v", err)
		}
		fibonacci := globals["fibonacci"]

		dict, err := toStringDict(item)
		if err != nil {
			log.Fatal(err)
		}
		st := starlarkstruct.FromStringDict(starlarkstruct.Default, dict)

		v, err := starlark.Call(thread, fibonacci, starlark.Tuple{st}, nil)
		if err != nil {
			log.Fatalf("Evaluate returned error: %v", err)
		}

		// Convert the Starlark result to a Go map
		result, err := starlarkValueToGo(v)
		if err != nil {
			log.Fatalf("Failed to convert Starlark result: %v", err)
		}

		// Ensure we have a map[string]interface{}
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			log.Fatalf("Expected map[string]interface{}, got %T", result)
		}

		return resultMap
	}
}

// Recursively converts map[string]interface{} to starlark.StringDict
func toStringDict(m map[string]interface{}) (starlark.StringDict, error) {
	dict := starlark.StringDict{}
	for k, v := range m {
		val, err := goToStarlarkValue(v)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", k, err)
		}
		dict[k] = val
	}
	return dict, nil
}

// Recursively converts Go values to starlark.Value
func goToStarlarkValue(v interface{}) (starlark.Value, error) {
	switch v := v.(type) {
	case string:
		return starlark.String(v), nil
	case int:
		return starlark.MakeInt(v), nil
	case int64:
		return starlark.MakeInt64(v), nil
	case float64:
		return starlark.Float(v), nil
	case bool:
		return starlark.Bool(v), nil
	case map[string]interface{}:
		d, err := toStringDict(v)
		if err != nil {
			return nil, err
		}
		return starlarkstruct.FromStringDict(starlarkstruct.Default, d), nil
	case []interface{}:
		elems := make([]starlark.Value, len(v))
		for i, elem := range v {
			sv, err := goToStarlarkValue(elem)
			if err != nil {
				return nil, fmt.Errorf("in slice at index %d: %w", i, err)
			}
			elems[i] = sv
		}
		return starlark.NewList(elems), nil
	default:
		return nil, fmt.Errorf("unsupported type %T", v)
	}
}

// starlarkValueToGo converts a Starlark value to a Go value
func starlarkValueToGo(v starlark.Value) (interface{}, error) {
	switch val := v.(type) {
	case starlark.String:
		return string(val), nil
	case starlark.Int:
		i, ok := val.Int64()
		if !ok {
			return nil, fmt.Errorf("int value out of range: %v", val)
		}
		return i, nil
	case starlark.Float:
		return float64(val), nil
	case starlark.Bool:
		return bool(val), nil
	case *starlark.List:
		result := make([]interface{}, val.Len())
		iter := val.Iterate()
		defer iter.Done()
		var x starlark.Value
		i := 0
		for iter.Next(&x) {
			item, err := starlarkValueToGo(x)
			if err != nil {
				return nil, err
			}
			result[i] = item
			i++
		}
		return result, nil
	case *starlark.Dict:
		result := make(map[string]interface{})
		for _, item := range val.Items() {
			key, ok := item[0].(starlark.String)
			if !ok {
				return nil, fmt.Errorf("dict key is not a string: %s", item[0].Type())
			}
			goVal, err := starlarkValueToGo(item[1])
			if err != nil {
				return nil, err
			}
			result[string(key)] = goVal
		}
		return result, nil
	case *starlarkstruct.Struct:
		return starlarkStructToMap(val)
	default:
		return nil, fmt.Errorf("unsupported starlark type: %s", v.Type())
	}
}

// starlarkStructToMap converts a Starlark struct to a Go map
func starlarkStructToMap(s *starlarkstruct.Struct) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	// Get all attributes from the struct
	attrs := s.AttrNames()
	for _, name := range attrs {
		val, err := s.Attr(name)
		if err != nil {
			return nil, fmt.Errorf("error getting attribute %s: %v", name, err)
		}
		
		goVal, err := starlarkValueToGo(val)
		if err != nil {
			return nil, err
		}
		
		result[name] = goVal
	}
	
	return result, nil
}
