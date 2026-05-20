package main

import (
	"fmt"

	"github.com/earendil-works/rho/pkg/sdk"
)

func main() {
	ext := sdk.New("rho.hello")

	// Register hello.greet tool
	ext.Tool("hello.greet", "Greet the user", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name of the user to greet",
			},
		},
		"required": []string{"name"},
	}, func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
		name, _ := args["name"].(string)
		if name == "" {
			name = "World"
		}
		msg := fmt.Sprintf("Hello, %s! Greetings from the out-of-process extension system!", name)
		return msg, false, nil
	})

	// Register /hello-test slash command
	ext.Command("hello-test", func(ctx sdk.Context, args []string) error {
		ctx.Notify("Starting interactive test from extension...", "info")

		// Test Confirm
		ok, err := ctx.Confirm("Hello Test", "Do you want to continue with the interactive test?")
		if err != nil {
			return err
		}
		if !ok {
			ctx.Notify("Test aborted by user", "info")
			return nil
		}

		// Test Select
		opt, err := ctx.Select("Choose an Option", []string{"Option A", "Option B", "Option C"})
		if err != nil {
			return err
		}

		// Test Input
		txt, err := ctx.Input("Enter message", "Type something here...")
		if err != nil {
			return err
		}

		ctx.Notify(fmt.Sprintf("You selected: %s, and entered: %s", opt, txt), "success")
		return nil
	})

	ext.Run()
}
