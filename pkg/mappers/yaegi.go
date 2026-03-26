package mappers

import (
	"fmt"
	"reflect"

	"github.com/Masterminds/semver/v3"
	"github.com/metraction/pharos/internal/logging"
	"github.com/reugn/go-streams/flow"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// semverConstraint is a wrapper function for Yaegi to use semver constraint checking
func semverConstraint(version, constraint string) bool {
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false
	}
	v, err := semver.NewVersion(version)
	if err != nil {
		return false
	}
	return c.Check(v)
}

func NewYaegi(rule string) flow.MapFunction[map[string]interface{}, map[string]interface{}] {
	logger := logging.NewLogger("info", "component", "Yaegi")
	fmt.Println("Creating Yaegi mapper for rule:", rule)
	return func(item map[string]interface{}) map[string]interface{} {
		// Create a new Yaegi interpreter
		i := interp.New(interp.Options{})

		// Use the standard library
		i.Use(stdlib.Symbols)

		// Define the semverConstraint function as a global variable
		_, err := i.Eval("var semverConstraint func(string, string) bool")
		if err != nil {
			errorString := "Failed to define semverConstraint variable"
			logger.Error().Err(err).Msg(errorString)
			return map[string]interface{}{
				"error": errorString,
			}
		}

		// Set the function value
		semverConstraintVal, err := i.Eval("semverConstraint")
		if err != nil {
			errorString := "Failed to get semverConstraint variable"
			logger.Error().Err(err).Msg(errorString)
			return map[string]interface{}{
				"error": errorString,
			}
		}
		semverConstraintVal.Set(reflect.ValueOf(semverConstraint))

		// Execute the Yaegi script first
		_, err = i.EvalPath(rule)
		if err != nil {
			errorString := "Failed to execute Yaegi script"
			logger.Error().Err(err).Msg(errorString)
			return map[string]interface{}{
				"error": errorString,
			}
		}

		// Get the enrich function
		enrichFunc, err := i.Eval("enrich")
		if err != nil {
			errorString := "Function 'enrich' not found in Yaegi script"
			logger.Error().Err(err).Msg(errorString)
			return map[string]interface{}{
				"error": errorString,
			}
		}

		// Call the enrich function with the payload
		// We need to convert the Go map to reflect.Value for the function call
		payloadValue := reflect.ValueOf(item)
		args := []reflect.Value{payloadValue}
		results := enrichFunc.Call(args)

		if len(results) == 0 {
			errorString := "Enrich function returned no results"
			logger.Error().Msg(errorString)
			return map[string]interface{}{
				"error": errorString,
			}
		}

		// Convert the result back to map[string]interface{}
		result := results[0].Interface()
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			errorString := "Expected map[string]interface{}, got different type"
			logger.Error().Msg(errorString)
			return map[string]interface{}{
				"error": errorString,
			}
		}

		return resultMap
	}
}
