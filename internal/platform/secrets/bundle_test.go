package secrets

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type boundedSecrets struct {
	*Fake
	fail bool
}

func (s *boundedSecrets) Set(ctx context.Context, id, value string) error {
	if len(value) > 2560 {
		return errors.New("credential too large")
	}
	if s.fail && strings.Contains(id, ".chunk.") {
		return errors.New("fixture write failure")
	}
	return s.Fake.Set(ctx, id, value)
}
func TestBundleRotationPreservesOldOnFailure(t *testing.T) {
	ctx := context.Background()
	s := &boundedSecrets{Fake: NewFake()}
	v := NewBundleStore(s)
	old := strings.Repeat("old", 4000)
	next := strings.Repeat("new", 4000)
	if err := v.Set(ctx, "fixture", old); err != nil {
		t.Fatal(err)
	}
	s.fail = true
	if err := v.Set(ctx, "fixture", next); err == nil {
		t.Fatal("expected write failure")
	}
	got, err := v.Get(ctx, "fixture")
	if err != nil || got != old {
		t.Fatal("failed rotation lost previous bundle")
	}
	s.fail = false
	if err := v.Set(ctx, "fixture", next); err != nil {
		t.Fatal(err)
	}
	got, err = v.Get(ctx, "fixture")
	if err != nil || got != next {
		t.Fatal("rotation not committed")
	}
	if err := v.Delete(ctx, "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get(ctx, "fixture"); !IsNotFound(err) {
		t.Fatal("bundle not deleted")
	}
}
