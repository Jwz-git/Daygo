package anthropic

import (
	"bytes"
	"encoding/json"
	"strings"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
)

// outputSchema adapts only the wire schema. Generate validates the response
// against the original schema, including constraints Anthropic cannot enforce.
// https://platform.claude.com/docs/en/build-with-claude/structured-outputs
func outputSchema(raw json.RawMessage) (map[string]any, error) {
	var schema map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&schema); err != nil || schema == nil {
		return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "output schema must be a JSON object", 0, err)
	}
	if err := adaptSchema(schema); err != nil {
		return nil, daygoai.NewError(daygoai.ErrorInvalidRequest, "cannot adapt output schema", 0, err)
	}
	return schema, nil
}

func adaptSchema(value any) error {
	node, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	constraints := make(map[string]any)
	for _, key := range []string{"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf", "minLength", "maxLength", "maxItems", "uniqueItems"} {
		if constraint, present := node[key]; present {
			constraints[key] = constraint
			delete(node, key)
		}
	}
	if number, ok := node["minItems"].(json.Number); ok {
		if count, err := number.Int64(); err == nil && count > 1 {
			constraints["minItems"] = number
			delete(node, "minItems")
		}
	}
	if len(constraints) > 0 {
		encoded, err := json.Marshal(constraints)
		if err != nil {
			return err
		}
		description, _ := node["description"].(string)
		if description != "" {
			description += "\n"
		}
		node["description"] = description + "Local validation constraints: " + string(encoded) + "."
	}
	// Walk schema positions, never property names, defaults, enums or examples.
	for _, key := range []string{"properties", "$defs", "definitions", "patternProperties", "dependentSchemas"} {
		if children, ok := node[key].(map[string]any); ok {
			for _, child := range children {
				if err := adaptSchema(child); err != nil {
					return err
				}
			}
		}
	}
	for _, key := range []string{"items", "additionalProperties", "contains", "not", "if", "then", "else"} {
		if err := adaptSchema(node[key]); err != nil {
			return err
		}
	}
	for _, key := range []string{"anyOf", "allOf", "oneOf", "prefixItems"} {
		if children, ok := node[key].([]any); ok {
			for _, child := range children {
				if err := adaptSchema(child); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Inspect only the error envelope for classification; provider prose can echo
// private inputs and is never returned or retained in an error chain.
func rejectsStructuredOutput(raw string) bool {
	if len(raw) > 4096 {
		return false
	}
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return false
	}
	message := strings.ToLower(envelope.Error.Message)
	for _, keyword := range []string{"output_config", "output_format", "json_schema", "structured output"} {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}
