package mappers

import (
	"fmt"
	"log"
	"reflect"

	"github.com/Masterminds/semver/v3"
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
	fmt.Println("Creating Yaegi mapper for rule:", rule)
	return func(item map[string]interface{}) map[string]interface{} {
		// Create a new Yaegi interpreter
		i := interp.New(interp.Options{})

		// Use the standard library
		i.Use(stdlib.Symbols)

		// Define the semverConstraint function as a global variable
		_, err := i.Eval("var semverConstraint func(string, string) bool")
		if err != nil {
			log.Fatalf("Failed to define semverConstraint variable: %v", err)
		}
		
		// Set the function value
		semverConstraintVal, err := i.Eval("semverConstraint")
		if err != nil {
			log.Fatalf("Failed to get semverConstraint variable: %v", err)
		}
		semverConstraintVal.Set(reflect.ValueOf(semverConstraint))

		// Execute the Yaegi script first
		_, err = i.EvalPath(rule)
		if err != nil {
			log.Fatalf("Failed to execute Yaegi script: %v", err)
		}

		// Get the enrich function
		enrichFunc, err := i.Eval("enrich")
		if err != nil {
			log.Fatalf("Function 'enrich' not found in Yaegi script: %v", err)
		}

		// Call the enrich function with the payload
		// We need to convert the Go map to reflect.Value for the function call
		payloadValue := reflect.ValueOf(item)
		args := []reflect.Value{payloadValue}
		results := enrichFunc.Call(args)

		if len(results) == 0 {
			log.Fatalf("Enrich function returned no results")
		}

		// Convert the result back to map[string]interface{}
		result := results[0].Interface()
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			log.Fatalf("Expected map[string]interface{}, got %T", result)
		}

		return resultMap
	}
}