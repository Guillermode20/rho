// Hello Extension - Minimal custom tool example for rho.
//
// Demonstrates how to register a custom tool using the rho Go SDK.
// The "hello" tool greets a person by name.
//
// Build:  go build -o hello ./examples/extensions/hello/
// Deploy: cp hello ~/.rho/extensions/hello/
//
//	cp examples/extensions/hello/rho.toml ~/.rho/extensions/hello/
package main

import (
	"fmt"

	"github.com/earendil-works/rho/pkg/sdk"
)

func main() {
	ext := sdk.New("rho.hello")

	ext.Tool("hello", "A simple greeting tool. Given a name, returns a friendly greeting.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Name to greet",
				},
			},
			"required": []interface{}{"name"},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			name, _ := args["name"].(string)
			if name == "" {
				name = "World"
			}
			return fmt.Sprintf("Hello, %s!", name), false, nil
		},
	)

	ext.Run()
}
