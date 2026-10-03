package anthropic

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOutputSchemaPreservesStructureAndMovesUnsupportedConstraints(t *testing.T) {
	// A property named minimum is data, not a schema keyword. Defaults and enum
	// values must not be walked as schemas either.
	input := []byte(`{
		"type":"object","additionalProperties":false,
		"properties":{
			"minimum":{"type":"integer","minimum":0,"maximum":10,"description":"Frame"},
			"text":{"type":"string","minLength":1,"maxLength":5,"pattern":"^[a-z]+$","enum":["abc"]},
			"array":{"type":"array","minItems":2,"maxItems":3,"uniqueItems":true,"items":{"type":"number","exclusiveMinimum":0,"exclusiveMaximum":10,"multipleOf":0.5}},
			"cards":{"type":"array","minItems":1,"items":{"$ref":"#/$defs/item"}},
			"choice":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]}
		},
		"$defs":{"item":{"type":"object","additionalProperties":false,"properties":{"frame":{"type":"integer","minimum":0}},"required":["frame"]}},
		"required":["minimum","text","array","cards","choice"],
		"default":{"minimum":2}
	}`)
	want := []byte(`{
		"type":"object","additionalProperties":false,
		"properties":{
			"minimum":{"type":"integer","description":"Frame\nLocal validation constraints: {\"maximum\":10,\"minimum\":0}."},
			"text":{"type":"string","pattern":"^[a-z]+$","enum":["abc"],"description":"Local validation constraints: {\"maxLength\":5,\"minLength\":1}."},
			"array":{"type":"array","description":"Local validation constraints: {\"maxItems\":3,\"minItems\":2,\"uniqueItems\":true}.","items":{"type":"number","description":"Local validation constraints: {\"exclusiveMaximum\":10,\"exclusiveMinimum\":0,\"multipleOf\":0.5}."}},
			"cards":{"type":"array","minItems":1,"items":{"$ref":"#/$defs/item"}},
			"choice":{"anyOf":[{"type":"integer","description":"Local validation constraints: {\"minimum\":0}."},{"type":"null"}]}
		},
		"$defs":{"item":{"type":"object","additionalProperties":false,"properties":{"frame":{"type":"integer","description":"Local validation constraints: {\"minimum\":0}."}},"required":["frame"]}},
		"required":["minimum","text","array","cards","choice"],
		"default":{"minimum":2}
	}`)
	got, err := outputSchema(input)
	if err != nil {
		t.Fatal(err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var gotValue, wantValue any
	if err := json.Unmarshal(gotJSON, &gotValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("schema = %s, want %s", gotJSON, want)
	}
}
