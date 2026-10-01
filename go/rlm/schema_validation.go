package rlm

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// SchemaValidationError lists every problem found when validating a value
// against a JSON schema, so retry feedback can name all of them at once.
//
// Field paths use dots for object keys and [i] for array items, e.g.
// "summaryType", "owner.email", "items[2].id".
type SchemaValidationError struct {
	// Missing holds the paths of required fields that were not provided.
	Missing []string
	// Invalid holds type mismatches, e.g. "field score: expected number, got string".
	Invalid []string
}

func (e *SchemaValidationError) Error() string {
	parts := make([]string, 0, e.issueCount())
	for _, field := range e.Missing {
		parts = append(parts, "missing required field: "+field)
	}
	parts = append(parts, e.Invalid...)
	return strings.Join(parts, "; ")
}

func (e *SchemaValidationError) issueCount() int {
	return len(e.Missing) + len(e.Invalid)
}

func (e *SchemaValidationError) orNil() error {
	if e.issueCount() == 0 {
		return nil
	}
	return e
}

func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

// validateAgainstSchema validates an object against a JSON schema, collecting
// every missing required field and type mismatch.
func validateAgainstSchema(data map[string]interface{}, schema *JSONSchema) error {
	if schema.Type != "object" {
		return nil // Only validate object types for now
	}
	verr := &SchemaValidationError{}
	collectObjectIssues(data, schema, "", verr)
	return verr.orNil()
}

// validateValue validates a value against a schema, collecting every issue.
func validateValue(value interface{}, schema *JSONSchema) error {
	verr := &SchemaValidationError{}
	collectValueIssues(value, schema, "", verr)
	return verr.orNil()
}

func collectObjectIssues(data map[string]interface{}, schema *JSONSchema, path string, verr *SchemaValidationError) {
	for _, required := range schema.Required {
		if _, exists := data[required]; !exists {
			verr.Missing = append(verr.Missing, joinPath(path, required))
		}
	}

	// Sorted for deterministic messages
	keys := make([]string, 0, len(schema.Properties))
	for key := range schema.Properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if value, exists := data[key]; exists {
			collectValueIssues(value, schema.Properties[key], joinPath(path, key), verr)
		}
	}
}

func collectValueIssues(value interface{}, schema *JSONSchema, path string, verr *SchemaValidationError) {
	if schema == nil || (value == nil && schema.Nullable) {
		return
	}

	mismatch := func(expected string) {
		msg := fmt.Sprintf("expected %s, got %T", expected, value)
		if path != "" {
			msg = fmt.Sprintf("field %s: %s", path, msg)
		}
		verr.Invalid = append(verr.Invalid, msg)
	}

	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			mismatch("string")
		}
	case "number", "integer":
		switch value.(type) {
		case float64, float32, int, int32, int64:
		default:
			mismatch("number")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			mismatch("boolean")
		}
	case "array":
		arr, ok := value.([]interface{})
		if !ok {
			mismatch("array")
			return
		}
		if schema.Items != nil {
			for i, item := range arr {
				collectValueIssues(item, schema.Items, path+"["+strconv.Itoa(i)+"]", verr)
			}
		}
	case "object":
		obj, ok := value.(map[string]interface{})
		if !ok {
			mismatch("object")
			return
		}
		collectObjectIssues(obj, schema, path, verr)
	}
}

// schemaAtPath returns the sub-schema for a field path produced by
// SchemaValidationError, or nil if the path does not exist in the schema.
func schemaAtPath(schema *JSONSchema, path string) *JSONSchema {
	current := schema
	for _, segment := range strings.Split(path, ".") {
		name := segment
		indexes := 0
		if i := strings.Index(segment, "["); i >= 0 {
			name = segment[:i]
			indexes = strings.Count(segment[i:], "[")
		}
		if current == nil || current.Properties == nil {
			return nil
		}
		current = current.Properties[name]
		for ; indexes > 0 && current != nil; indexes-- {
			current = current.Items
		}
	}
	return current
}

// writeFieldRequirements describes a missing field's schema for retry feedback.
func writeFieldRequirements(b *strings.Builder, fieldPath string, fieldSchema *JSONSchema) {
	if fieldSchema == nil {
		fmt.Fprintf(b, "- '%s'\n", fieldPath)
		return
	}
	fmt.Fprintf(b, "- '%s': %s", fieldPath, fieldSchema.Type)
	if len(fieldSchema.Enum) > 0 {
		fmt.Fprintf(b, ", one of: %s", strings.Join(fieldSchema.Enum, ", "))
	}
	b.WriteString("\n")

	if fieldSchema.Type == "object" && fieldSchema.Properties != nil {
		names := make([]string, 0, len(fieldSchema.Properties))
		for name := range fieldSchema.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			mark := ""
			if contains(fieldSchema.Required, name) {
				mark = " [REQUIRED]"
			}
			fmt.Fprintf(b, "    - %s: %s%s\n", name, fieldSchema.Properties[name].Type, mark)
		}
	}
	if fieldSchema.Type == "array" && fieldSchema.Items != nil {
		fmt.Fprintf(b, "    - array of: %s\n", fieldSchema.Items.Type)
	}
}
