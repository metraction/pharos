package mappers

import (
	"fmt"
	"os"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/metraction/pharos/internal/logging"
	"github.com/reugn/go-streams/flow"
)

var logger = logging.NewLogger("info", "component", "mappers")

func NewStarlet(rule string) (flow.MapFunction[map[string]interface{}, map[string]interface{}], error) {
	scriptBytes, err := os.ReadFile(rule)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to read script file")
		return nil, err
	}

	return func(data map[string]interface{}) map[string]interface{} {
		starlarkVal, err := dataconv.GoToStarlarkViaJSON(data)
		if err != nil {
			logger.Error().Err(err).Msg("Error converting data to Starlark")
			return data
		}

		// Convert the data map to a Starlark-compatible dictionary
		globals := starlet.StringAnyMap{
			"greet": func(name string) string {
				return fmt.Sprintf("Hello, %s!", name)
			},
			// Make the data available as a properly converted Starlark dictionary
			"payload": starlarkVal,
		}

		vm := starlet.NewWithNames(globals, []string{"random"}, nil)
		res, err := vm.RunScript(scriptBytes, nil)
		if err != nil {
			logger.Error().Err(err).Msg("Error executing script")
			return data
		}

		// If the script returned any values, merge them back into the data map
		if len(res) > 0 {
			logger.Info().Interface("script_result", res).Msg("Script execution result")
			for k, v := range res {
				data[k] = v
			}
		}

		return data
	}, nil
}
