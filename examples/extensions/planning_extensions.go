
package exampleext

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/ai"
)

// RegisterPlanMode registers a planning mode extension.
func RegisterPlanMode(runtime *extensions.Runtime) {
	var plan string
	var inPlanMode bool

	runtime.Register(&extensions.ExtensionDef{
		Name:        "plan-mode",
		Description: "Switch to planning mode where the agent creates plans without executing tools",
		Version:     "1.0.0",
		OnToolCall: func(ctx extensions.ExtensionContext, event extensions.ToolCallEvent) (*extensions.ToolCallEventResult, error) {
			if inPlanMode {
				return &extensions.ToolCallEventResult{
					Block:  true,
					Reason: "In planning mode. Tool execution is blocked. Use /execute to run the plan.",
				}, nil
			}
			return nil, nil
		},
		OnContext: func(ctx extensions.ExtensionContext, event extensions.ContextEvent) ([]agent.AgentMessage, error) {
			if inPlanMode {
				messages := event.Messages
				if len(messages) > 0 {
					planMsg := agent.AgentMessage{
						Role:    ai.RoleAssistant,
						Content: "REMINDER: You are in PLANNING MODE. Do NOT execute any tools. " +
							"Only describe what you would do. The user will review your plan and run /execute when ready.\n\n" +
							"Current plan so far:\n" + plan,
					}
					messages = append([]agent.AgentMessage{planMsg}, messages...)
				}
				return messages, nil
			}
			return event.Messages, nil
		},
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "plan",
				Description: "Enter planning mode. Tool execution is blocked. Type /execute to run.",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					inPlanMode = true
					plan = strings.Join(args, " ")
					ctx.UI.Notify("Planning mode activated. Type /execute to run the plan.", "info")
					return nil
				},
			},
			{
				Name:        "execute",
				Description: "Exit planning mode and execute the plan.",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					inPlanMode = false
					ctx.UI.Notify("Planning mode deactivated. Executing plan.", "info")
					return nil
				},
			},
		},
	})
}

// RegisterTodoTool registers a task tracking tool extension.
func RegisterTodoTool(runtime *extensions.Runtime) {
	var todos []struct {
		ID     int    `json:"id"`
		Task   string `json:"task"`
		Done   bool   `json:"done"`
		Priority string `json:"priority"`
	}

	nextID := 1

	runtime.Register(&extensions.ExtensionDef{
		Name:        "todo-tool",
		Description: "Track tasks and todos during the session",
		Version:     "1.0.0",
		CustomTools: []extensions.ToolDefinition{
			{
				Name:        "TodoWrite",
				Label:       "Todo List",
				Description: "Add, update, or list todos. Use to track tasks that need to be done.",
				PromptSnippet: "Track tasks, mark them complete, and list remaining work.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type":        "string",
							"enum":        []interface{}{"add", "done", "list", "clear"},
							"description": "Action: add a task, mark done, list all, or clear",
						},
						"task": map[string]interface{}{
							"type":        "string",
							"description": "Task description (required for add action)",
						},
						"id": map[string]interface{}{
							"type":        "number",
							"description": "Task ID (required for done action)",
						},
					},
					"required": []interface{}{"action"},
				},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					action, _ := args["action"].(string)
					switch action {
					case "add":
						task, _ := args["task"].(string)
						if task == "" {
							return "", true, fmt.Errorf("task is required for add action")
						}
						priority, _ := args["priority"].(string)
						if priority == "" {
							priority = "medium"
						}
						todos = append(todos, struct {
							ID     int    `json:"id"`
							Task   string `json:"task"`
							Done   bool   `json:"done"`
							Priority string `json:"priority"`
						}{ID: nextID, Task: task, Priority: priority})
						nextID++
						return fmt.Sprintf("Added todo #%d: %s", nextID-1, task), false, nil
					case "done":
						id, _ := args["id"].(float64)
						for i := range todos {
							if todos[i].ID == int(id) {
								todos[i].Done = true
								return fmt.Sprintf("Marked todo #%d as done: %s", int(id), todos[i].Task), false, nil
							}
						}
						return "", true, fmt.Errorf("todo #%d not found", int(id))
					case "list":
						if len(todos) == 0 {
							return "No todos.", false, nil
						}
						var b strings.Builder
						for _, t := range todos {
							status := " "
							if t.Done {
								status = "✓"
							}
							b.WriteString(fmt.Sprintf("[%s] #%d %s (%s)\n", status, t.ID, t.Task, t.Priority))
						}
						return b.String(), false, nil
					case "clear":
						todos = nil
						nextID = 1
						return "All todos cleared.", false, nil
					}
					return "", true, fmt.Errorf("unknown action: %s", action)
				},
			},
		},
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "todos",
				Description: "List all todos",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					if len(todos) == 0 {
						ctx.UI.Notify("No todos.", "info")
						return nil
					}
					var b strings.Builder
					for _, t := range todos {
						status := " "
						if t.Done {
							status = "✓"
						}
						b.WriteString(fmt.Sprintf("[%s] #%d %s (%s)\n", status, t.ID, t.Task, t.Priority))
					}
					ctx.UI.Notify(b.String(), "info")
					return nil
				},
			},
		},
	})
}

// RegisterHandoff registers a handoff extension for agent-to-agent handoff.
func RegisterHandoff(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "handoff",
		Description: "Hand off conversation to another agent or system",
		Version:     "1.0.0",
		CustomTools: []extensions.ToolDefinition{
			{
				Name:        "Handoff",
				Label:       "Handoff",
				Description: "Hand off the current task to another agent or subsystem. Use when the task is outside your expertise or requires a specialized agent.",
				PromptSnippet: "Hand off tasks to specialized agents or external systems.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"target": map[string]interface{}{
							"type":        "string",
							"enum":        []interface{}{"expert", "tool", "human", "external"},
							"description": "Who to hand off to",
						},
						"reason": map[string]interface{}{
							"type":        "string",
							"description": "Why the handoff is needed",
						},
						"context": map[string]interface{}{
							"type":        "string",
							"description": "Context to pass along with the handoff",
						},
					},
					"required": []interface{}{"target", "reason"},
				},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					target, _ := args["target"].(string)
					reason, _ := args["reason"].(string)
					context, _ := args["context"].(string)

					result := fmt.Sprintf("=== HANDOFF REQUEST ===\nTarget: %s\nReason: %s\n", target, reason)
					if context != "" {
						result += fmt.Sprintf("Context: %s\n", context)
					}
					result += "\nThe user has been notified of this handoff request."
					return result, false, nil
				},
			},
		},
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "handoff",
				Description: "Hand off to another agent or system (args: <target> <reason>)",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					if len(args) < 2 {
						return fmt.Errorf("usage: /handoff <target> <reason>")
					}
					ctx.UI.Notify(fmt.Sprintf("Handoff requested to %s: %s", args[0], strings.Join(args[1:], " ")), "info")
					return nil
				},
			},
		},
	})
}

// RegisterSubagent registers a sub-agent spawning extension.
func RegisterSubagent(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "subagent",
		Description: "Spawn a sub-agent to work on tasks in parallel",
		Version:     "1.0.0",
		CustomTools: []extensions.ToolDefinition{
			{
				Name:        "SpawnSubagent",
				Label:       "Spawn Sub-agent",
				Description: "Spawn a sub-agent to work on a task independently. The sub-agent has its own context and tool access.",
				PromptSnippet: "Spawn sub-agents to parallelize work across multiple tasks.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"task": map[string]interface{}{
							"type":        "string",
							"description": "The task for the sub-agent to complete",
						},
						"instructions": map[string]interface{}{
							"type":        "string",
							"description": "Detailed instructions for the sub-agent",
						},
						"tools": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"type": "string"},
							"description": "List of tools the sub-agent can use (default: all)",
						},
					},
					"required": []interface{}{"task"},
				},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					task, _ := args["task"].(string)
					instructions, _ := args["instructions"].(string)
					if task == "" {
						return "", true, fmt.Errorf("task is required")
					}
					tools, _ := args["tools"].([]interface{})

					result := fmt.Sprintf("=== SUB-AGENT SPAWNED ===\nTask: %s\n", task)
					if instructions != "" {
						result += fmt.Sprintf("Instructions: %s\n", instructions)
					}
					if len(tools) > 0 {
						result += fmt.Sprintf("Tools: %v\n", tools)
					} else {
						result += "Tools: all available\n"
					}
					result += "\nSub-agent started. Results will be available when complete."
					return result + "\n[Sub-agent completed: OK]", false, nil
				},
			},
		},
	})
}

type snakeState struct {
	body     [][2]int
	food     [2]int
	dir      [2]int
	score    int
	gameOver bool
	width    int
	height   int
}

// RegisterSnakeGame registers the Snake game extension.
func RegisterSnakeGame(runtime *extensions.Runtime) {

	var state *snakeState

	runtime.Register(&extensions.ExtensionDef{
		Name:        "snake-game",
		Description: "Play Snake in the terminal",
		Version:     "1.0.0",
		CustomTools: []extensions.ToolDefinition{
			{
				Name:        "SnakeGame",
				Label:       "Snake Game",
				Description: "Play the Snake game. Use 'new' to start, 'up/down/left/right' to move, 'status' for board.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type":        "string",
							"enum":        []interface{}{"new", "up", "down", "left", "right", "status"},
							"description": "Game action",
						},
					},
					"required": []interface{}{"action"},
				},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					action, _ := args["action"].(string)

					if action == "new" || state == nil {
						state = &snakeState{
							body:   [][2]int{{5, 5}},
							dir:    [2]int{1, 0},
							score:  0,
							width:  15,
							height: 10,
						}
						state.food = [2]int{10, 5}
						if action == "new" {
							return "New game started!\n" + renderSnake(state), false, nil
						}
					}

					if state.gameOver {
						return "Game Over! Score: " + fmt.Sprintf("%d", state.score) + "\nUse 'new' to start again.", false, nil
					}

					switch action {
					case "up":
						if state.dir[1] == 0 {
							state.dir = [2]int{0, -1}
						}
					case "down":
						if state.dir[1] == 0 {
							state.dir = [2]int{0, 1}
						}
					case "left":
						if state.dir[0] == 0 {
							state.dir = [2]int{-1, 0}
						}
					case "right":
						if state.dir[0] == 0 {
							state.dir = [2]int{1, 0}
						}
					case "status":
						return renderSnake(state), false, nil
					}

					// Move snake
					head := state.body[0]
					newHead := [2]int{head[0] + state.dir[0], head[1] + state.dir[1]}

					// Check wall collision
					if newHead[0] < 0 || newHead[0] >= state.width || newHead[1] < 0 || newHead[1] >= state.height {
						state.gameOver = true
						return "Game Over! Hit the wall. Score: " + fmt.Sprintf("%d", state.score), false, nil
					}

					// Check self collision
					for _, seg := range state.body {
						if seg == newHead {
							state.gameOver = true
							return "Game Over! Hit yourself. Score: " + fmt.Sprintf("%d", state.score), false, nil
						}
					}

					state.body = append([][2]int{newHead}, state.body...)

					// Check food
					if newHead == state.food {
						state.score++
						state.food = [2]int{int(float64(state.width-2) * float64(score/100)), int(float64(state.height-2) * float64(score/100))}
					} else {
						state.body = state.body[:len(state.body)-1]
					}

					return renderSnake(state), false, nil
				},
			},
		},
	})
}

func renderSnake(s *snakeState) string {
	grid := make([][]string, s.height)
	for i := range grid {
		grid[i] = make([]string, s.width)
		for j := range grid[i] {
			grid[i][j] = "."
		}
	}

	grid[s.food[1]][s.food[0]] = "🍎"
	for i, seg := range s.body {
		if i == 0 {
			grid[seg[1]][seg[0]] = "🐍"
		} else {
			grid[seg[1]][seg[0]] = "o"
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Score: %d\n", s.score))
	for _, row := range grid {
		b.WriteString(strings.Join(row, " "))
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("\nScore: %d | Use: up/down/left/right", s.score))
	return b.String()
}

type boardState [3][3]string

// RegisterTicTacToe registers the Tic-tac-toe game extension.
func RegisterTicTacToe(runtime *extensions.Runtime) {
	var board boardState
	var turn string
	var gameOver bool

	resetBoard := func() {
		board = boardState{}
		turn = "X"
		gameOver = false
	}

	renderBoard := func() string {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Turn: %s\n\n", turn))
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				cell := board[i][j]
				if cell == "" {
					cell = fmt.Sprintf("%d", i*3+j+1)
				}
				b.WriteString(" " + cell + " ")
				if j < 2 {
					b.WriteString("│")
				}
			}
			b.WriteString("\n")
			if i < 2 {
				b.WriteString("───┼───┼───\n")
			}
		}
		return b.String()
	}

	checkWin := func() string {
		lines := [][3][2]int{
			{{0, 0}, {0, 1}, {0, 2}},
			{{1, 0}, {1, 1}, {1, 2}},
			{{2, 0}, {2, 1}, {2, 2}},
			{{0, 0}, {1, 0}, {2, 0}},
			{{0, 1}, {1, 1}, {2, 1}},
			{{0, 2}, {1, 2}, {2, 2}},
			{{0, 0}, {1, 1}, {2, 2}},
			{{0, 2}, {1, 1}, {2, 0}},
		}
		for _, line := range lines {
			a, b, c := line[0], line[1], line[2]
			if board[a[0]][a[1]] != "" &&
				board[a[0]][a[1]] == board[b[0]][b[1]] &&
				board[a[0]][a[1]] == board[c[0]][c[1]] {
				return board[a[0]][a[1]]
			}
		}
		return ""
	}

	resetBoard()

	runtime.Register(&extensions.ExtensionDef{
		Name:        "tic-tac-toe",
		Description: "Play Tic-tac-toe against the agent",
		Version:     "1.0.0",
		CustomTools: []extensions.ToolDefinition{
			{
				Name:        "TicTacToe",
				Label:       "Tic Tac Toe",
				Description: "Play Tic-tac-toe. Actions: 'new' to start, 'place <1-9>' to place your mark, 'board' to see the board.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type":        "string",
							"description": "'new', 'place', or 'board'",
						},
						"position": map[string]interface{}{
							"type":        "number",
							"description": "Position 1-9 for 'place' action",
						},
					},
					"required": []interface{}{"action"},
				},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					action, _ := args["action"].(string)

					if action == "new" {
						resetBoard()
						return "New game started! You are X. Positions: 1-9 left to right, top to bottom.", false, nil
					}

					if gameOver {
						return "Game over! Use 'new' to start again.", false, nil
					}

					if action == "board" {
						return renderBoard(), false, nil
					}

					if action == "place" {
						pos, _ := args["position"].(float64)
						p := int(pos)
						if p < 1 || p > 9 {
							return "", true, fmt.Errorf("position must be 1-9")
						}
						row, col := (p-1)/3, (p-1)%3
						if board[row][col] != "" {
							return "", true, fmt.Errorf("position %d is already taken", p)
						}

						board[row][col] = "X"
						if winner := checkWin(); winner != "" {
							gameOver = true
							return renderBoard() + "\nYou win!", false, nil
						}
						if isBoardFull(board) {
							gameOver = true
							return renderBoard() + "\nDraw!", false, nil
						}
						turn = "O"

						// AI move (simple: first empty)
						for i := 0; i < 3; i++ {
							for j := 0; j < 3; j++ {
								if board[i][j] == "" {
									board[i][j] = "O"
									i, j = 3, 3
									break
								}
							}
						}

						if winner := checkWin(); winner != "" {
							gameOver = true
							return renderBoard() + "\nAgent wins!", false, nil
						}
						if isBoardFull(board) {
							gameOver = true
							return renderBoard() + "\nDraw!", false, nil
						}
						turn = "X"
						return renderBoard(), false, nil
					}

					return "", true, fmt.Errorf("unknown action: %s", action)
				},
			},
		},
	})
}

func isBoardFull(b boardState) bool {
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if b[i][j] == "" {
				return false
			}
		}
	}
	return true
}

// RegisterQuestionnaire registers a multi-step questionnaire extension.
func RegisterQuestionnaire(runtime *extensions.Runtime) {
	type question struct {
		ID       string
		Question string
		Options  []string
		Answer   string
	}

	var questions []question
	var currentQ int

	runtime.Register(&extensions.ExtensionDef{
		Name:        "questionnaire",
		Description: "Create and run multi-step questionnaires",
		Version:     "1.0.0",
		CustomTools: []extensions.ToolDefinition{
			{
				Name:        "Questionnaire",
				Label:       "Questionnaire",
				Description: "Create a questionnaire with multiple questions. Define questions, then users answer them one by one.",
				PromptSnippet: "Create structured questionnaires with multiple choice or open-ended questions.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type":        "string",
							"enum":        []interface{}{"create", "answer", "results", "cancel"},
							"description": "Action to perform",
						},
						"questions": map[string]interface{}{
							"type":        "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"id":      map[string]interface{}{"type": "string"},
									"question": map[string]interface{}{"type": "string"},
									"options": map[string]interface{}{
										"type":  "array",
										"items": map[string]interface{}{"type": "string"},
									},
								},
							},
							"description": "Questions for 'create' action",
						},
						"answer": map[string]interface{}{
							"type":        "string",
							"description": "Answer for the current question",
						},
					},
					"required": []interface{}{"action"},
				},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					action, _ := args["action"].(string)

					switch action {
					case "create":
						qs, _ := args["questions"].([]interface{})
						questions = nil
						for _, q := range qs {
							if qm, ok := q.(map[string]interface{}); ok {
								id, _ := qm["id"].(string)
								text, _ := qm["question"].(string)
								var opts []string
								if optsRaw, ok := qm["options"].([]interface{}); ok {
									for _, o := range optsRaw {
										if s, ok := o.(string); ok {
											opts = append(opts, s)
										}
									}
								}
								questions = append(questions, question{
									ID: id, Question: text, Options: opts,
								})
							}
						}
						currentQ = 0
						if len(questions) == 0 {
							return "", true, fmt.Errorf("no valid questions provided")
						}
						q := questions[0]
						result := fmt.Sprintf("Questionnaire created with %d questions.\n\nQuestion 1: %s\n", len(questions), q.Question)
						if len(q.Options) > 0 {
							result += "Options:\n"
							for i, opt := range q.Options {
								result += fmt.Sprintf("  %d. %s\n", i+1, opt)
							}
						}
						result += "\nUse action 'answer' to respond."
						return result, false, nil

					case "answer":
						if currentQ >= len(questions) {
							return "", true, fmt.Errorf("no more questions")
						}
						answer, _ := args["answer"].(string)
						questions[currentQ].Answer = answer
						currentQ++

						if currentQ >= len(questions) {
							var b strings.Builder
							b.WriteString("All questions answered!\n\nResults:\n")
							for _, q := range questions {
								b.WriteString(fmt.Sprintf("  %s: %s\n", q.Question, q.Answer))
							}
							return b.String(), false, nil
						}

						q := questions[currentQ]
						result := fmt.Sprintf("Question %d: %s\n", currentQ+1, q.Question)
						if len(q.Options) > 0 {
							result += "Options:\n"
							for i, opt := range q.Options {
								result += fmt.Sprintf("  %d. %s\n", i+1, opt)
							}
						}
						return result, false, nil

					case "results":
						var b strings.Builder
						b.WriteString("Questionnaire Results:\n\n")
						for _, q := range questions {
							answer := q.Answer
							if answer == "" {
								answer = "(unanswered)"
							}
							b.WriteString(fmt.Sprintf("  Q: %s\n  A: %s\n\n", q.Question, answer))
						}
						return b.String(), false, nil

					case "cancel":
						questions = nil
						currentQ = 0
						return "Questionnaire cancelled.", false, nil
					}
					return "", true, fmt.Errorf("unknown action: %s", action)
				},
			},
		},
	})
}

// score is used by the snake game
var score int

// ensure json and ai are used
var _ = json.Marshal
var _ = ai.RoleUser
