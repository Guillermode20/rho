// Bookmark Extension
//
// Bookmark important messages in the conversation for later reference.
// Allows naming bookmarks and jumping back to them.
//
// Build:  go build -o bookmark ./examples/extensions/bookmark/
// Deploy: cp bookmark ~/.rho/extensions/bookmark/
//         cp examples/extensions/bookmark/rho.toml ~/.rho/extensions/bookmark/
package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/sdk"
)

type bookmark struct {
	ID        string
	Name      string
	Message   string
	CreatedAt time.Time
}

var (
	bookmarks   []bookmark
	bookmarksMu sync.Mutex
	nextID      int
)

func main() {
	ext := sdk.New("rho.bookmark")

	ext.Command("bookmark", func(ctx sdk.Context, args []string) error {
		name := strings.Join(args, " ")
		if name == "" {
			name = fmt.Sprintf("Bookmark #%d", nextID+1)
		}

		bookmarksMu.Lock()
		nextID++
		id := fmt.Sprintf("bm_%d", nextID)
		bm := bookmark{
			ID:        id,
			Name:      name,
			Message:   fmt.Sprintf("Bookmark: %s", name),
			CreatedAt: time.Now(),
		}
		bookmarks = append(bookmarks, bm)
		bookmarksMu.Unlock()

		ctx.Notify(fmt.Sprintf("🔖 Bookmark '%s' created (id: %s)", name, id), "success")
		return nil
	})

	ext.Command("bookmarks", func(ctx sdk.Context, args []string) error {
		bookmarksMu.Lock()
		bms := make([]bookmark, len(bookmarks))
		copy(bms, bookmarks)
		bookmarksMu.Unlock()

		if len(bms) == 0 {
			ctx.Notify("No bookmarks yet. Use /bookmark <name> to create one.", "info")
			return nil
		}

		sort.Slice(bms, func(i, j int) bool {
			return bms[i].CreatedAt.After(bms[j].CreatedAt)
		})

		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("🔖 %d bookmark(s):\n", len(bms)))
		for i, bm := range bms {
			msg.WriteString(fmt.Sprintf("  %d. [%s] %s (%s)\n",
				i+1, bm.ID, bm.Name, bm.CreatedAt.Format("15:04:05")))
		}
		ctx.Notify(msg.String(), "info")
		return nil
	})

	ext.Command("goto-bookmark", func(ctx sdk.Context, args []string) error {
		if len(args) == 0 {
			ctx.Notify("Usage: /goto-bookmark <bookmark-id>", "info")
			return nil
		}

		id := args[0]
		bookmarksMu.Lock()
		var found *bookmark
		for i, bm := range bookmarks {
			if bm.ID == id || fmt.Sprintf("%d", i+1) == id {
				found = &bookmarks[i]
				break
			}
		}
		bookmarksMu.Unlock()

		if found == nil {
			ctx.Notify(fmt.Sprintf("Bookmark '%s' not found. Use /bookmarks to list them.", id), "error")
			return nil
		}

		ctx.Notify(fmt.Sprintf("📍 Jumped to bookmark: %s (%s)", found.Name, found.ID), "success")
		return nil
	})

	ext.Tool("add_bookmark", "Bookmark the current point in the conversation with a descriptive name. "+
		"Bookmarks can be listed with /bookmarks and navigated to with /goto-bookmark.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "A descriptive name for the bookmark",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Optional detailed description of what was happening",
				},
			},
			"required": []interface{}{"name"},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			name, _ := args["name"].(string)
			description, _ := args["description"].(string)

			if name == "" {
				return "", true, fmt.Errorf("name is required")
			}

			bookmarksMu.Lock()
			nextID++
			id := fmt.Sprintf("bm_%d", nextID)
			msg := name
			if description != "" {
				msg = fmt.Sprintf("%s: %s", name, description)
			}
			bm := bookmark{
				ID:        id,
				Name:      name,
				Message:   msg,
				CreatedAt: time.Now(),
			}
			bookmarks = append(bookmarks, bm)
			bookmarksMu.Unlock()

			return fmt.Sprintf("🔖 Bookmark '%s' created (id: %s). Use /goto-bookmark %s to return.", name, id, id), false, nil
		},
	)

	ext.Tool("list_bookmarks", "List all bookmarks in the conversation.",
		map[string]interface{}{
			"type":     "object",
			"properties": map[string]interface{}{},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			bookmarksMu.Lock()
			bms := make([]bookmark, len(bookmarks))
			copy(bms, bookmarks)
			bookmarksMu.Unlock()

			if len(bms) == 0 {
				return "No bookmarks yet.", false, nil
			}

			var result strings.Builder
			result.WriteString(fmt.Sprintf("🔖 %d bookmark(s):\n\n", len(bms)))
			for i, bm := range bms {
				result.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, bm.ID, bm.Name))
			}
			return result.String(), false, nil
		},
	)

	ext.Run()
}
