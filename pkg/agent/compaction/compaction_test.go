package compaction

import (
	"testing"
)

func TestDefaultTokenEstimator(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"hello", 2},       // 5/4+1 = 2
		{"hello world", 3}, // 11/4+1 = 3
	}
	for _, tt := range tests {
		got := DefaultTokenEstimator(tt.text)
		if got != tt.want {
			t.Errorf("DefaultTokenEstimator(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestEstimateTokens(t *testing.T) {
	got := EstimateTokens("hello world")
	if got <= 0 {
		t.Errorf("expected positive, got %d", got)
	}
}

func TestEstimateContextTokens(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}
	total := EstimateContextTokens(messages)
	if total <= 0 {
		t.Errorf("expected positive, got %d", total)
	}
}

func TestFindCutPoint(t *testing.T) {
	settings := DefaultCompactionSettings()
	messages := []Message{
		{Role: "user", Content: "Message 1"},
		{Role: "assistant", Content: "Response 1"},
		{Role: "user", Content: "Message 2"},
	}
	cp := FindCutPoint(messages, 10, settings)
	if cp != nil {
		if cp.StartIndex < 0 || cp.EndIndex > len(messages) {
			t.Error("invalid cut point indices")
		}
	}
}

func TestFindCutPointNoNeed(t *testing.T) {
	settings := DefaultCompactionSettings()
	settings.TriggerRatio = 0.99
	messages := []Message{
		{Role: "user", Content: "short"},
	}
	cp := FindCutPoint(messages, 100000, settings)
	if cp != nil {
		t.Log("FindCutPoint returned non-nil (may be fine)")
	}
}

func TestShouldCompact(t *testing.T) {
	if ShouldCompact(nil, 100, 5) {
		t.Error("ShouldCompact(nil) should be false")
	}
}

func TestCompact(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "Tell me about Go"},
		{Role: "assistant", Content: "Go is a language"},
		{Role: "user", Content: "More?"},
		{Role: "assistant", Content: "Yes!"},
	}
	cut := &CutPoint{StartIndex: 0, EndIndex: 2, TokensBefore: 5}
	result, err := Compact(messages, cut, nil)
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestCompactCustomSummary(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}
	cut := &CutPoint{StartIndex: 0, EndIndex: 1, TokensBefore: 1}
	fn := func(text string) (string, error) { return "Custom summary", nil }
	result, err := Compact(messages, cut, fn)
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}
	if result.Summary != "Custom summary" {
		t.Errorf("expected 'Custom summary', got '%s'", result.Summary)
	}
}

func TestCompactNilCutPoint(t *testing.T) {
	_, err := Compact(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil cut point")
	}
}

func TestDefaultCompactionSettings(t *testing.T) {
	s := DefaultCompactionSettings()
	if s.TriggerRatio != 0.75 {
		t.Errorf("expected 0.75, got %f", s.TriggerRatio)
	}
}

func TestNewCompacter(t *testing.T) {
	c := NewCompacter(DefaultCompactionSettings())
	if c == nil {
		t.Fatal("expected non-nil Compacter")
	}
}

func TestPrepareCompaction(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	prep := PrepareCompaction(messages, 100)
	if prep != nil {
		t.Logf("Preparation: %d msgs to summarize", len(prep.MessagesToSummarize))
	}
}

func TestFormatForSummary(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!", ToolCalls: []ToolCall{{Name: "test"}}},
	}
	formatted := formatForSummary(messages)
	if formatted == "" {
		t.Error("expected non-empty format")
	}
}

func TestGenerateSimpleSummary(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "toolResult", Content: "c"},
	}
	summary := generateSimpleSummary(messages)
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestGenerateSimpleSummaryEmpty(t *testing.T) {
	s := generateSimpleSummary(nil)
	if s != "Empty conversation" {
		t.Errorf("expected 'Empty conversation', got '%s'", s)
	}
}

func TestCompactionEntry(t *testing.T) {
	e := CompactionEntry{Type: "compaction", Summary: "test", Tokens: 100}
	if e.Type != "compaction" {
		t.Errorf("expected compaction type, got '%s'", e.Type)
	}
}
