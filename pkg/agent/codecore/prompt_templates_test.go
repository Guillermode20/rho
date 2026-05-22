package codecore

import (
	"testing"
)

func TestFormatTemplateWithArgsUnresolved(t *testing.T) {
	tpl := "Hello $1 $2 and ${@:2} end"
	vars := map[string]string{}
	args := []string{"world"}

	result := FormatTemplateWithArgs(tpl, vars, args)
	expected := "Hello world  and  end"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
