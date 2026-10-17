package storage

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Jwz-git/Daygo/internal/domain"
)

const trainingDiskLimit int64 = 1 << 30
const trainingFileLimit = 32 << 20

var ErrTrainingExists = errors.New("training sample exists")
var ErrTrainingLimit = errors.New("training storage limit")
var ErrTrainingFormat = errors.New("training export contains reserved media markers")
var trainingID = regexp.MustCompile(`^[a-f0-9]{32}$`)

// TrainingFiles is owned and serialized by the training service of the writable
// host. It never opens a database. os.Root confines every user-selected ID.
type TrainingFiles struct{ directory string }

func NewTrainingFiles(directory string) *TrainingFiles { return &TrainingFiles{directory} }
func (s *Store) TrainingFiles() *TrainingFiles {
	return NewTrainingFiles(filepath.Join(filepath.Dir(s.path), "training"))
}
func (r *TrainingFiles) open() (*os.Root, error) {
	if err := os.MkdirAll(r.directory, 0700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(r.directory)
	if err != nil {
		return nil, err
	}
	for _, dir := range []string{"samples", "exports"} {
		if err := root.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			root.Close()
			return nil, err
		}
	}
	return root, nil
}
func trainingName(id string) (string, error) {
	if !trainingID.MatchString(id) {
		return "", errors.New("invalid sample id")
	}
	return "samples/" + id + ".json", nil
}
func randomTrainingID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (r *TrainingFiles) Group(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	root, err := r.open()
	if err != nil {
		return "", err
	}
	defer root.Close()
	b, err := root.ReadFile("source-group")
	if err == nil {
		if !trainingID.Match(b) {
			return "", errors.New("invalid source group")
		}
		return string(b), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	id := randomTrainingID()
	return id, root.WriteFile("source-group", []byte(id), 0600)
}
func (r *TrainingFiles) Get(ctx context.Context, id string) (domain.TrainingSample, error) {
	var s domain.TrainingSample
	name, err := trainingName(id)
	if err != nil {
		return s, err
	}
	if err = ctx.Err(); err != nil {
		return s, err
	}
	root, err := r.open()
	if err != nil {
		return s, err
	}
	defer root.Close()
	f, err := root.Open(name)
	if err != nil {
		return s, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, trainingFileLimit+1))
	if err != nil {
		return s, err
	}
	if len(data) > trainingFileLimit {
		return s, ErrTrainingLimit
	}
	if err = json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	if s.ID != id || s.Version != 1 {
		return s, errors.New("invalid sample envelope")
	}
	return s, nil
}
func (r *TrainingFiles) List(ctx context.Context) ([]domain.TrainingSummary, error) {
	root, err := r.open()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir, err := root.Open("samples")
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	out := []domain.TrainingSummary{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		s, err := r.Get(ctx, strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		out = append(out, domain.TrainingSummary{ID: s.ID, Purpose: s.Purpose, CreatedAt: s.CreatedAt, Model: s.RequestedModel, Verdict: s.Verdict, PrivacyApproved: s.PrivacyApproved, Revision: s.Revision})
	}
	return out, nil
}
func trainingUsage(root *os.Root) (int64, error) {
	var total int64
	for _, dir := range []string{"samples", "exports"} {
		f, err := root.Open(dir)
		if err != nil {
			return 0, err
		}
		entries, err := f.ReadDir(-1)
		f.Close()
		if err != nil {
			return 0, err
		}
		if dir == "samples" && len(entries) > 1000 {
			return 0, ErrTrainingLimit
		}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				return 0, err
			}
			total += info.Size()
		}
	}
	return total, nil
}
func (r *TrainingFiles) Save(ctx context.Context, s domain.TrainingSample, create bool) error {
	name, err := trainingName(s.ID)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if len(data) > trainingFileLimit {
		return ErrTrainingLimit
	}
	root, err := r.open()
	if err != nil {
		return err
	}
	defer root.Close()
	old, statErr := root.Stat(name)
	if create && statErr == nil {
		return ErrTrainingExists
	}
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	used, err := trainingUsage(root)
	if err != nil {
		return err
	}
	if old != nil {
		used -= old.Size()
	}
	if used+int64(len(data)) > trainingDiskLimit {
		return ErrTrainingLimit
	}
	tmp := "samples/" + randomTrainingID() + ".tmp"
	defer root.Remove(tmp)
	if err = root.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return root.Rename(tmp, name)
}
func (r *TrainingFiles) Delete(ctx context.Context, id string) error {
	name, err := trainingName(id)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	root, err := r.open()
	if err != nil {
		return err
	}
	defer root.Close()
	return root.Remove(name)
}

// Export writes a complete atomic ZIP. It retains the source group and original
// request for provenance and adds Qwen-compatible image/conversations records.
func (r *TrainingFiles) Export(ctx context.Context, samples []domain.TrainingSample) (string, error) {
	if len(samples) == 0 {
		return "", errors.New("empty training export")
	}
	var estimate int64
	for _, s := range samples {
		if !s.PrivacyApproved || s.Verdict != "accepted" || !json.Valid([]byte(s.Label)) {
			return "", errors.New("unapproved training sample")
		}
		if _, err := trainingName(s.ID); err != nil {
			return "", err
		}
		for _, part := range s.Parts {
			if strings.Contains(part.Text, "<image>") || strings.Contains(part.Text, "<video>") {
				return "", ErrTrainingFormat
			}
			if len(part.Image) > 0 && part.MIME != "image/png" && part.MIME != "image/jpeg" {
				return "", ErrTrainingFormat
			}
		}
		if strings.Contains(s.Label, "<image>") || strings.Contains(s.Label, "<video>") {
			return "", ErrTrainingFormat
		}
		b, _ := json.Marshal(s)
		estimate += int64(len(b)) * 3
	}
	root, err := r.open()
	if err != nil {
		return "", err
	}
	defer root.Close()
	used, err := trainingUsage(root)
	if err != nil {
		return "", err
	}
	if used+estimate > trainingDiskLimit {
		return "", ErrTrainingLimit
	}
	id := randomTrainingID()
	tmp := "exports/" + id + ".tmp"
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer root.Remove(tmp)
	z := zip.NewWriter(f)
	defer f.Close()
	write := func(name string, data []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		w, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	records := []any{}
	for _, s := range samples {
		images := []string{}
		var prompt strings.Builder
		for i, p := range s.Parts {
			if len(p.Image) > 0 {
				ext := "jpg"
				if p.MIME == "image/png" {
					ext = "png"
				}
				name := fmt.Sprintf("images/%s-%d.%s", s.ID, i, ext)
				if err := write(name, p.Image); err != nil {
					return "", err
				}
				images = append(images, name)
				prompt.WriteString("<image>\n")
			} else {
				prompt.WriteString(p.Text)
				prompt.WriteByte('\n')
			}
		}
		prompt.WriteString("\nRequired output JSON schema:\n")
		prompt.Write(s.Schema)
		record := map[string]any{"id": s.ID, "group_id": s.Group, "conversations": []map[string]string{{"from": "human", "value": prompt.String()}, {"from": "gpt", "value": s.Label}}}
		if len(images) > 0 {
			record["image"] = images
		}
		records = append(records, record)
		data, _ := json.Marshal(s)
		if err := write("samples/"+s.ID+".json", data); err != nil {
			return "", err
		}
	}
	data, _ := json.Marshal(records)
	if err := write("train.json", data); err != nil {
		return "", err
	}
	if err := write("README.txt", []byte("Local reviewed data. Do not upload automatically. Split entire group_id values, never individual rows. images and train.json use relative paths. samples retains original candidate and input; review all content before sharing.\n")); err != nil {
		return "", err
	}
	if err := z.Close(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	name := "exports/" + id + ".zip"
	if err := root.Rename(tmp, name); err != nil {
		return "", err
	}
	return filepath.Join(r.directory, filepath.FromSlash(name)), nil
}
