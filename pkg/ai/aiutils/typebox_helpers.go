package aiutils

import (
	"encoding/json"
	"fmt"
	"strings"
)

type SchemaValidationError struct {
	Path    string      `json:"path"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

type SchemaValidator struct {
	schema map[string]interface{}
}

func NewSchemaValidator(schema map[string]interface{}) *SchemaValidator {
	return &SchemaValidator{schema: schema}
}

func (sv *SchemaValidator) Validate(data interface{}) (bool, []SchemaValidationError) {
	var errs []SchemaValidationError
	s := sv.schema
	if s == nil {
		return true, nil
	}
	if t, ok := s["type"].(string); ok {
		jt := typeOf(data)
		if jt != t && t != "any" {
			errs = append(errs, SchemaValidationError{Path: "", Message: fmt.Sprintf("expected %q got %q", t, jt), Value: data})
		}
	}
	if req, ok := s["required"].([]interface{}); ok {
		if obj, ok := data.(map[string]interface{}); ok {
			for _, r := range req {
				if rs, ok := r.(string); ok {
					if _, exists := obj[rs]; !exists {
						errs = append(errs, SchemaValidationError{Path: "/" + rs, Message: fmt.Sprintf("missing %q", rs)})
					}
				}
			}
		}
	}
	return len(errs) == 0, errs
}

func ValidateToolArguments(args map[string]interface{}, schema map[string]interface{}) (bool, string) {
	if schema == nil {
		return true, ""
	}
	v := NewSchemaValidator(schema)
	ok, errs := v.Validate(args)
	if !ok && len(errs) > 0 {
		return false, errs[0].Message
	}
	return true, ""
}

type TSchema map[string]interface{}

func String() TSchema     { return TSchema{"type": "string"} }
func Number() TSchema     { return TSchema{"type": "number"} }
func Boolean() TSchema    { return TSchema{"type": "boolean"} }
func Integer() TSchema    { return TSchema{"type": "integer"} }

func Array(items TSchema) TSchema {
	return TSchema{"type": "array", "items": items}
}

func Object(props map[string]TSchema) TSchema {
	p := make(map[string]interface{})
	for k, v := range props {
		p[k] = v
	}
	return TSchema{"type": "object", "properties": p}
}

func Optional(s TSchema) TSchema {
	s["optional"] = true
	return s
}

func Enum(vals ...string) TSchema {
	items := make([]interface{}, len(vals))
	for i, v := range vals {
		items[i] = v
	}
	return TSchema{"type": "string", "enum": items}
}

func typeOf(data interface{}) string {
	if data == nil {
		return "null"
	}
	switch data.(type) {
	case bool:
		return "boolean"
	case float64, float32, int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case json.Number:
		return "number"
	}
	return fmt.Sprintf("%T", data)
}

func DerefSchema(schema map[string]interface{}, defs map[string]map[string]interface{}) map[string]interface{} {
	if ref, ok := schema["$ref"].(string); ok {
		rp := strings.TrimPrefix(ref, "#/$defs/")
		if d, ok := defs[rp]; ok {
			return d
		}
	}
	return schema
}
