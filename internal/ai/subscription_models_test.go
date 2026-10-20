package ai

import "testing"

func TestSubscriptionCatalogKeepsVisibleSlugsInServerOrder(t *testing.T) {
	models, err := parseSubscriptionModels([]byte(`{"models":[{"slug":"z","display_name":"Z","visibility":"list"},{"slug":"hidden","visibility":"hide"},{"slug":"a","visibility":"list"},{"slug":"z","visibility":"list"}]}`))
	if err != nil || len(models) != 2 || models[0] != "z" || models[1] != "a" {
		t.Fatalf("models=%v err=%v", models, err)
	}
}
