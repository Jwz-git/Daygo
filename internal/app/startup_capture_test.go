package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jwz-git/Daygo/internal/recordinglocation"
	"github.com/Jwz-git/Daygo/internal/storage"
)

func TestStartupCaptureServicesRequireOwnership(t *testing.T) {
	for _, mode := range []string{"read-only", "writer without capture lock", "owner"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			dir := t.TempDir()
			writer, err := storage.Open(ctx, storage.Options{Dir: dir, CaptureOwnerRequested: mode != "writer without capture lock"})
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = writer.Close() })
			store := writer
			if mode == "read-only" {
				store, err = storage.Open(ctx, storage.Options{Dir: dir, CaptureOwnerRequested: true})
				if err != nil {
					cancel()
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = store.Close() })
			}
			t.Cleanup(cancel)

			source := filepath.Join(dir, "old")
			target := filepath.Join(dir, "recordings")
			for _, root := range []string{source, target} {
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "fixture.jpg"), []byte("anonymous frame"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			state, err := json.Marshal(map[string]string{"source": source, "target": target, "phase": "committed"})
			if err != nil {
				t.Fatal(err)
			}
			if err := writer.Settings().SetMany(ctx, map[string]string{recordinglocation.MigrationKey: string(state)}); err != nil {
				t.Fatal(err)
			}

			b := NewBackend(nil, store)
			t.Cleanup(func() {
				cancel()
				b.stopTimelineTasks()
			})
			b.startCaptureOwnerServices(ctx, dir, target, nil, "windows")

			_, sourceErr := os.Stat(filepath.Join(source, "fixture.jpg"))
			if _, err := os.Stat(filepath.Join(target, "fixture.jpg")); err != nil {
				t.Fatal(err)
			}
			_, _, phase, err := recordinglocation.Pending(ctx, writer.Settings())
			if err != nil {
				t.Fatal(err)
			}
			if mode == "owner" {
				if !os.IsNotExist(sourceErr) || phase != "" || b.analysisService() == nil {
					t.Fatalf("owner startup: old file: %v, phase: %q, analysis started: %v", sourceErr, phase, b.analysisService() != nil)
				}
			} else if sourceErr != nil || phase != "committed" || b.analysisService() != nil {
				t.Fatalf("non-owner startup: old file: %v, phase: %q, analysis started: %v", sourceErr, phase, b.analysisService() != nil)
			}
		})
	}
}
