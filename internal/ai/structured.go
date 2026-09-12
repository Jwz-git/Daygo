package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func ParseStructuredOutput(text string, output OutputSchema) (json.RawMessage, error) {
	raw, err := extractJSON(text)
	if err != nil {
		return nil, NewError(ErrorInvalidOutput, "model output does not contain valid JSON", 0, err)
	}
	if err := ValidateJSON(raw, output); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), raw...), nil
}

// ValidateJSON checks an already-extracted JSON value against an output
// schema. It is the schema half of ParseStructuredOutput, exported so callers
// that hold raw JSON from elsewhere — chat tool arguments decoded from a
// model's envelope, for instance — validate against the same library and the
// same error kinds instead of growing a second validator.
func ValidateJSON(raw json.RawMessage, output OutputSchema) error {
	var schemaValue any
	if err := decodeJSON(output.Schema, &schemaValue); err != nil {
		return NewError(ErrorInvalidRequest, "output schema is invalid", 0, err)
	}
	compiler := jsonschema.NewCompiler()
	const schemaURL = "urn:daygo:output-schema"
	if err := compiler.AddResource(schemaURL, schemaValue); err != nil {
		return NewError(ErrorInvalidRequest, "output schema cannot be loaded", 0, err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return NewError(ErrorInvalidRequest, "output schema cannot be compiled", 0, err)
	}
	var value any
	if err := decodeJSON(raw, &value); err != nil {
		return NewError(ErrorInvalidOutput, "model output JSON is invalid", 0, err)
	}
	if err := schema.Validate(value); err != nil {
		return NewError(ErrorInvalidOutput, "model output does not match schema", 0, err)
	}
	return nil
}

func extractJSON(text string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```") {
		if newline := strings.IndexByte(trimmed, '\n'); newline >= 0 {
			trimmed = trimmed[newline+1:]
		}
		if fence := strings.LastIndex(trimmed, "```"); fence >= 0 {
			trimmed = trimmed[:fence]
		}
		trimmed = strings.TrimSpace(trimmed)
	}

	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != '{' && trimmed[i] != '[' {
			continue
		}
		candidate, ok := jsonCandidate(trimmed[i:])
		if !ok {
			continue
		}
		repaired := removeTrailingCommas(candidate)
		if json.Valid([]byte(repaired)) {
			return json.RawMessage(repaired), nil
		}
	}
	return nil, fmt.Errorf("no JSON value found")
}

func jsonCandidate(text string) (string, bool) {
	stack := make([]byte, 0, 8)
	inString := false
	escaped := false
	for i := 0; i < len(text); i++ {
		current := text[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch current {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch current {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, current)
		case '}', ']':
			if len(stack) == 0 || (current == '}' && stack[len(stack)-1] != '{') ||
				(current == ']' && stack[len(stack)-1] != '[') {
				return "", false
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return strings.TrimSpace(text[:i+1]), true
			}
		}
	}
	return "", false
}

func removeTrailingCommas(text string) string {
	var repaired strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(text); i++ {
		current := text[i]
		if inString {
			repaired.WriteByte(current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			repaired.WriteByte(current)
			continue
		}
		if current == ',' {
			next := i + 1
			for next < len(text) && (text[next] == ' ' || text[next] == '\n' || text[next] == '\r' || text[next] == '\t') {
				next++
			}
			if next < len(text) && (text[next] == '}' || text[next] == ']') {
				continue
			}
		}
		repaired.WriteByte(current)
	}
	return repaired.String()
}

func decodeJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("multiple JSON values")
	}
	return nil
}
