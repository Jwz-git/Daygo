package settings_test

import (
	"context"
	"testing"

	"github.com/Jwz-git/Daygo/internal/settings"
	"github.com/Jwz-git/Daygo/internal/storage"
)

type countedSettingsRepo struct {
	*storage.SettingsRepo
	loads int
}

func (r *countedSettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	r.loads++
	return r.SettingsRepo.GetAll(ctx)
}

func TestSettingsSnapshotCache(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writer, err := storage.Open(ctx, storage.Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	reader, err := storage.OpenReadOnly(ctx, writer.Path(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()

	raw := writer.Settings()
	if err := raw.SetMany(ctx, map[string]string{
		settings.KeyPrivacyBlockedApplicationIDs: `["anonymous.example"]`,
		settings.KeyProvidersRouting:             `{"chain":[{"providerId":"fixture","model":"first"}]}`,
	}); err != nil {
		t.Fatal(err)
	}
	writerRepo := &countedSettingsRepo{SettingsRepo: raw}
	writeAccess := settings.New(writerRepo)
	readAccess := settings.New(reader.Settings())
	load := func(access *settings.Settings) settings.Snapshot {
		t.Helper()
		got, err := access.Load(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	first := load(writeAccess)
	first.BlockedApplicationIDs[0] = "mutated"
	first.ProvidersRouting.Chain[0].Model = "mutated"
	got := load(writeAccess)
	if writerRepo.loads != 1 || got.BlockedApplicationIDs[0] != "anonymous.example" || got.ProvidersRouting.Chain[0].Model != "first" {
		t.Fatalf("cache hit = %+v, repository reads = %d", got, writerRepo.loads)
	}
	if got := load(readAccess); got.Theme != settings.DefaultTheme {
		t.Fatalf("initial read-only theme = %q", got.Theme)
	}

	if err := raw.Set(ctx, settings.KeyAppearanceTheme, `"dark"`); err != nil {
		t.Fatal(err)
	}
	for _, access := range []*settings.Settings{writeAccess, readAccess} {
		if got := load(access); got.Theme != "dark" {
			t.Fatalf("after committed write theme = %q", got.Theme)
		}
	}
	if writerRepo.loads != 2 {
		t.Fatalf("committed write should require one new read, got %d total reads", writerRepo.loads)
	}
	if err := raw.Delete(ctx, settings.KeyAppearanceTheme); err != nil {
		t.Fatal(err)
	}
	if got := load(writeAccess); got.Theme != settings.DefaultTheme {
		t.Fatalf("after deleting theme = %q", got.Theme)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := writeAccess.Load(canceled); err == nil {
		t.Fatal("cache hit ignored context cancellation")
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := writeAccess.Load(ctx); err == nil {
		t.Fatal("cache hit returned settings from a closed store")
	}
}
