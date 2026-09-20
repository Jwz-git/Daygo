package analysis

import (
	"testing"

	"github.com/Jwz-git/Daygo/internal/ai"
)

func TestCardsSchemaRejectsEmptyArray(t *testing.T) {
	if err := ai.ValidateJSON([]byte(`{"cards":[]}`), cardsOutput); err == nil {
		t.Fatal("empty cards array passed schema validation")
	}
}
