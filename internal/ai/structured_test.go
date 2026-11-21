package ai

import (
	"encoding/json"
	"testing"
)

func TestCompiledSchemaUsesContentsAndReusesValidation(t *testing.T) {
	text := OutputSchema{Name: "fixture", Schema: json.RawMessage(`{"type":"string"}`)}
	number := OutputSchema{Name: "fixture", Schema: json.RawMessage(`{"type":"number"}`)}
	first, err := compiledSchema(text)
	if err != nil {
		t.Fatal(err)
	}
	second, err := compiledSchema(text)
	if err != nil || first != second {
		t.Fatalf("identical schema was compiled again: %v", err)
	}
	if err := ValidateJSON(json.RawMessage("1"), number); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSON(json.RawMessage("1"), text); ErrorKindOf(err) != ErrorInvalidOutput {
		t.Fatalf("schemas with identical names must remain separate: %v", err)
	}
}

func TestStructuredResultUsesValidatedJSONWithTextFallback(t *testing.T) {
	schema := OutputSchema{Name: "fixture", Schema: json.RawMessage(`{"type":"object"}`)}
	raw := json.RawMessage(`{"fixture":true}`)
	for _, result := range []Result{{JSON: raw}, {Text: string(raw)}} {
		got, err := result.StructuredJSON(schema)
		if err != nil || string(got) != string(raw) {
			t.Fatalf("structured output = %s, err = %v", got, err)
		}
	}
}
