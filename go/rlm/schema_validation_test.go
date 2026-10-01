package rlm

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func summarySchema() *JSONSchema {
	return &JSONSchema{
		Type:     "object",
		Required: []string{"summaryType", "title", "owner"},
		Properties: map[string]*JSONSchema{
			"summaryType": {Type: "string", Enum: []string{"brief", "detailed"}},
			"title":       {Type: "string"},
			"score":       {Type: "number"},
			"owner": {
				Type:     "object",
				Required: []string{"name", "email"},
				Properties: map[string]*JSONSchema{
					"name":  {Type: "string"},
					"email": {Type: "string"},
				},
			},
			"items": {
				Type: "array",
				Items: &JSONSchema{
					Type:       "object",
					Required:   []string{"id"},
					Properties: map[string]*JSONSchema{"id": {Type: "string"}},
				},
			},
		},
	}
}

func TestValidateAgainstSchema_ReportsAllIssues(t *testing.T) {
	data := map[string]interface{}{
		"owner": map[string]interface{}{"name": "Ada"},
		"score": "high",
		"items": []interface{}{map[string]interface{}{"id": "a"}, map[string]interface{}{}},
	}
	err := validateAgainstSchema(data, summarySchema())
	var verr *SchemaValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *SchemaValidationError, got %T: %v", err, err)
	}
	wantMissing := []string{"summaryType", "title", "items[1].id", "owner.email"}
	if fmt.Sprint(verr.Missing) != fmt.Sprint(wantMissing) {
		t.Errorf("Missing = %v, want %v", verr.Missing, wantMissing)
	}
	if len(verr.Invalid) != 1 || verr.Invalid[0] != "field score: expected number, got string" {
		t.Errorf("Invalid = %v", verr.Invalid)
	}
	for _, field := range wantMissing {
		if !strings.Contains(err.Error(), "missing required field: "+field) {
			t.Errorf("error %q does not mention %s", err.Error(), field)
		}
	}
	// Deterministic across runs (map iteration order must not leak into messages)
	for i := 0; i < 20; i++ {
		if again := validateAgainstSchema(data, summarySchema()); again.Error() != err.Error() {
			t.Fatalf("non-deterministic error: %q vs %q", again.Error(), err.Error())
		}
	}
}

func TestParseAndValidateJSON_WrappedResponseKeepsSpecificIssues(t *testing.T) {
	response := "Here is the summary:\n```json\n{\"title\": \"Q3\", \"owner\": {\"name\": \"Ada\", \"email\": \"a@x.io\"}}\n```\nLet me know!"
	_, err := parseAndValidateJSON(response, summarySchema())
	if err == nil || !strings.Contains(err.Error(), "missing required field: summaryType") {
		t.Fatalf("expected the specific missing field, got %v", err)
	}
}

func TestBuildValidationFeedback_ListsEveryMissingField(t *testing.T) {
	schema := summarySchema()
	err := validateAgainstSchema(map[string]interface{}{"owner": map[string]interface{}{}}, schema)
	feedback := buildValidationFeedback(err, schema, `{"owner": {}}`)

	for _, want := range []string{
		"These REQUIRED fields were not provided: summaryType, title, owner.name, owner.email",
		"- 'summaryType': string, one of: brief, detailed",
		"- 'owner.email': string",
	} {
		if !strings.Contains(feedback, want) {
			t.Errorf("feedback missing %q:\n%s", want, feedback)
		}
	}
}

func TestBuildValidationFeedback_InvalidJSON(t *testing.T) {
	_, err := parseAndValidateJSON(`{"summaryType": "brief",}`, summarySchema())
	if err == nil {
		t.Fatal("expected a parse error")
	}
	feedback := buildValidationFeedback(err, summarySchema(), `{"summaryType": "brief",}`)
	if !strings.Contains(feedback, "Your response was not valid JSON") {
		t.Errorf("feedback should explain the JSON error:\n%s", feedback)
	}
}

func TestSchemaAtPath(t *testing.T) {
	schema := summarySchema()
	if s := schemaAtPath(schema, "owner.email"); s == nil || s.Type != "string" {
		t.Errorf("owner.email = %+v", s)
	}
	if s := schemaAtPath(schema, "items[3].id"); s == nil || s.Type != "string" {
		t.Errorf("items[3].id = %+v", s)
	}
	if s := schemaAtPath(schema, "nope.x"); s != nil {
		t.Errorf("nope.x = %+v", s)
	}
}
