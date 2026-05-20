package codecui

import (
	"strings"
	"testing"
)

func TestToolExecutionPending(t *testing.T) {
	tc := NewToolExecutionComponent("call_1", "read_file", map[string]interface{}{"path": "/tmp/test.txt"})
	lines := tc.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected render output")
	}
	if !strings.Contains(lines[0], "read_file") {
		t.Fatalf("header should contain tool name: %s", lines[0])
	}
	if !strings.Contains(lines[0], "○") {
		t.Fatalf("pending state should show ○: %s", lines[0])
	}
}

func TestToolExecutionRunning(t *testing.T) {
	tc := NewToolExecutionComponent("call_1", "bash", map[string]interface{}{"cmd": "ls"})
	tc.SetRunning()
	tc.Tick()
	lines := tc.Render(80)
	if !strings.Contains(lines[0], "◌") {
		t.Fatalf("running state should show ◌: %s", lines[0])
	}
}

func TestToolExecutionCompleted(t *testing.T) {
	tc := NewToolExecutionComponent("call_1", "read_file", map[string]interface{}{})
	tc.SetOutput("file contents\nline 2\nline 3", false)
	lines := tc.Render(80)
	if !strings.Contains(lines[0], "●") {
		t.Fatalf("completed state should show ●: %s", lines[0])
	}
	if len(lines) < 2 {
		t.Fatal("expected multiple lines for expanded output")
	}
}

func TestToolExecutionFailed(t *testing.T) {
	tc := NewToolExecutionComponent("call_1", "bash", map[string]interface{}{})
	tc.SetOutput("permission denied", true)
	lines := tc.Render(80)
	if !strings.Contains(lines[0], "●") && !strings.Contains(lines[1], "permission denied") {
		t.Fatalf("failed state should show error: %#v", lines)
	}
}

func TestToolExecutionBlocked(t *testing.T) {
	tc := NewToolExecutionComponent("call_1", "write_file", nil)
	tc.SetBlocked("extension denied")
	lines := tc.Render(80)
	if !strings.Contains(lines[0], "⊘") {
		t.Fatalf("blocked state should show ⊘: %s", lines[0])
	}
}

func TestToolExecutionToggleExpand(t *testing.T) {
	tc := NewToolExecutionComponent("call_1", "read_file", map[string]interface{}{})
	tc.SetOutput("output", false)
	// Initially expanded
	if !tc.isExpanded {
		t.Fatal("should start expanded")
	}
	expandedLines := tc.Render(80)
	if len(expandedLines) <= 1 {
		t.Fatal("expanded should show output")
	}

	// Toggle to collapsed
	tc.ToggleExpanded()
	if tc.isExpanded {
		t.Fatal("should be collapsed after toggle")
	}
	collapsedLines := tc.Render(80)
	if len(collapsedLines) != 1 {
		t.Fatalf("collapsed should only show header, got %d lines", len(collapsedLines))
	}
}

func TestToolExecutionArgsDisplay(t *testing.T) {
	tc := NewToolExecutionComponent("call_1", "search", map[string]interface{}{"query": "hello world", "limit": 10})
	lines := tc.Render(80)
	if !strings.Contains(lines[0], "query") || !strings.Contains(lines[0], "limit") {
		t.Fatalf("header should show args: %s", lines[0])
	}
}

func TestToolExecutionEmptyOutput(t *testing.T) {
	tc := NewToolExecutionComponent("call_1", "noop", nil)
	tc.SetOutput("", false)
	lines := tc.Render(80)
	if len(lines) < 2 {
		t.Fatal("expected at least 2 lines for completed with empty output")
	}
}

func TestToolExecutionGroup(t *testing.T) {
	g := NewToolExecutionGroup()
	tc1 := NewToolExecutionComponent("call_1", "read", nil)
	tc2 := NewToolExecutionComponent("call_2", "write", nil)
	g.AddExecution(tc1)
	g.AddExecution(tc2)

	lines := g.Render(80)
	// Each pending tool renders header + waiting line = 2 lines each
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	// Both tool names should be visible
	combined := strings.Join(lines, " ")
	if !strings.Contains(combined, "read") || !strings.Contains(combined, "write") {
		t.Fatalf("expected both tool names: %s", combined)
	}
}

func TestToolExecutionGroupClear(t *testing.T) {
	g := NewToolExecutionGroup()
	g.AddExecution(NewToolExecutionComponent("call_1", "read", nil))
	g.Clear()
	lines := g.Render(80)
	if len(lines) != 0 {
		t.Fatalf("expected empty after clear, got %d lines", len(lines))
	}
}

func TestToolExecutionGroupGetExecution(t *testing.T) {
	g := NewToolExecutionGroup()
	tc := NewToolExecutionComponent("call_1", "read", nil)
	g.AddExecution(tc)

	got := g.GetExecution("call_1")
	if got == nil {
		t.Fatal("expected to find execution by id")
	}
	if got.toolName != "read" {
		t.Fatalf("expected 'read', got %q", got.toolName)
	}
}

func TestBashExecutionCommand(t *testing.T) {
	be := NewBashExecution("ls -la")
	lines := be.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected render output")
	}
	found := false
	for _, l := range lines {
		if strings.Contains(l, "ls -la") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("bash execution should show command: %#v", lines)
	}
}

func TestBashExecutionCompleted(t *testing.T) {
	be := NewBashExecution("echo hello")
	be.SetCompleted(0, "hello\nworld\n", false)
	lines := be.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected render output")
	}
}

func TestBashExecutionExitCode(t *testing.T) {
	be := NewBashExecution("false")
	be.SetCompleted(1, "", false)
	lines := be.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected render output")
	}
}

func TestBashExecutionRunning(t *testing.T) {
	be := NewBashExecution("sleep 10")
	be.SetRunning(true)
	lines := be.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected render output for running state")
	}
}

func TestBashExecutionTruncated(t *testing.T) {
	be := NewBashExecution("cat largefile")
	be.SetCompleted(0, "lots of output", true)
	lines := be.Render(80)
	combined := strings.Join(lines, " ")
	if !strings.Contains(combined, "truncat") && !strings.Contains(combined, "more") {
		t.Fatalf("truncated execution should mention truncation: %s", combined)
	}
}
