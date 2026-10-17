package storage

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jwz-git/Daygo/internal/domain"
)

func TestTrainingFilesBoundaries(t *testing.T) {
	root := t.TempDir()
	r := NewTrainingFiles(root)
	s := domain.TrainingSample{Version: 1, ID: "0123456789abcdef0123456789abcdef", Verdict: "pending", Revision: 1}
	if err := r.Save(context.Background(), s, true); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(context.Background(), s, true); !errors.Is(err, ErrTrainingExists) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := r.Get(context.Background(), "../escape"); err == nil {
		t.Fatal("path traversal accepted")
	}
	if _, err := r.Export(context.Background(), []domain.TrainingSample{s}); err == nil {
		t.Fatal("unapproved export")
	}
	s.PrivacyApproved = true
	s.Verdict = "accepted"
	s.Label = `{"observations":[]}`
	s.Parts = []domain.TrainingPart{{Text: "Describe"}, {MIME: "image/png", Image: []byte("anonymous fixture")}}
	path, err := r.Export(context.Background(), []domain.TrainingSample{s})
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	found := map[string]bool{}
	for _, f := range z.File {
		found[f.Name] = true
		if f.Name == "train.json" {
			reader, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			var records []struct {
				Image         []string `json:"image"`
				Conversations []struct {
					Value string `json:"value"`
				} `json:"conversations"`
			}
			if err := json.Unmarshal(raw, &records); err != nil {
				t.Fatal(err)
			}
			if len(records) != 1 || len(records[0].Image) != 1 || strings.Count(records[0].Conversations[0].Value, "<image>") != 1 || records[0].Conversations[1].Value != s.Label {
				t.Fatal("broken training pair")
			}
		}

	}
	if !found["train.json"] || !found["samples/"+s.ID+".json"] || !found["images/"+s.ID+"-1.png"] {
		t.Fatal(found)
	}
	s.Parts[0].Text = "literal <image>"
	if _, err := r.Export(context.Background(), []domain.TrainingSample{s}); !errors.Is(err, ErrTrainingFormat) {
		t.Fatal("ambiguous media marker exported")
	}
	if err := r.Delete(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Get(context.Background(), s.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	// A symlink must not escape the bounded file store.
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "samples", "abcdefabcdefabcdefabcdefabcdefab.json")); err == nil {
		if _, err := r.Get(context.Background(), "abcdefabcdefabcdefabcdefabcdefab"); err == nil {
			t.Fatal("symlink escape")
		}
	}
	if !json.Valid([]byte(s.Label)) {
		t.Fatal("fixture")
	}
}
