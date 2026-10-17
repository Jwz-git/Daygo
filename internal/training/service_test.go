package training

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

type fakeProvider struct{ calls int }

func (p *fakeProvider) Generate(_ context.Context, _ ai.Request) (ai.Result, error) {
	p.calls++
	return ai.Result{Text: `{"observations":[{"from_frame":0,"to_frame":0,"observation":"red","apps":[]}]}`, Model: "fixture-model"}, nil
}
func fixtureRequest(t *testing.T) ai.Request {
	t.Helper()
	image, err := ai.ImagePart(ai.MediaPNG, []byte("anonymous"))
	if err != nil {
		t.Fatal(err)
	}
	return ai.Request{Training: &domain.TrainingContext{}, Purpose: ai.PurposeTranscribe, Parts: []ai.Part{ai.TextPart("describe"), image}, Output: &ai.OutputSchema{Name: "fixture", Schema: json.RawMessage(`{"type":"object","required":["observations"],"properties":{"observations":{"type":"array"}}}`)}}
}
func TestConsentSnapshotReviewAndExport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := storage.NewTrainingFiles(t.TempDir())
	s := New(repo, nil)
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	defer func() { cancel(); <-done }()
	p := &fakeProvider{}
	wrapped := s.Wrap(p, "fixture-provider", "fixture-model", "openai")
	req := fixtureRequest(t)
	if _, err := wrapped.Generate(ctx, req); err != nil {
		t.Fatal(err)
	}
	if len(s.queue) != 0 {
		t.Fatal("disabled captured")
	}
	if err := s.Start(ctx, time.Now().Add(time.Hour).Unix(), 2, true); err != nil {
		t.Fatal(err)
	}
	if _, err := wrapped.Generate(ctx, req); err != nil {
		t.Fatal(err)
	}
	if _, err := wrapped.Generate(ctx, req); err != nil {
		t.Fatal(err)
	}
	var id string
	for deadline := time.Now().Add(time.Second * 3); time.Now().Before(deadline); {
		list, err := s.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) > 0 {
			id = list[0].ID
			break
		}
		time.Sleep(time.Millisecond * 5)
	}
	if id == "" {
		t.Fatal("sample not saved")
	}
	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Fatal("duplicate captured")
	}
	sample, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if string(sample.Parts[1].Image) != "anonymous" || sample.Verdict != "pending" {
		t.Fatal("snapshot mismatch")
	}
	if _, err := s.Export(ctx, []string{id}); err == nil {
		t.Fatal("unreviewed export")
	}
	if err := s.Review(ctx, id, 1, sample.Label, true, "accepted"); err != nil {
		t.Fatal(err)
	}
	if err := s.Review(ctx, id, 1, sample.Label, true, "accepted"); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale edit: %v", err)
	}
	if _, err := s.Export(ctx, []string{id}); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	if s.Status().Active {
		t.Fatal("stop")
	}
	restarted := New(repo, nil)
	if restarted.Status().Active {
		t.Fatal("consent persisted")
	}
}
func TestReviewRejectsOutOfRangeAndStopInvalidatesTicket(t *testing.T) {
	ctx := context.Background()
	s := New(storage.NewTrainingFiles(t.TempDir()), nil)
	if err := s.Start(ctx, time.Now().Add(time.Hour).Unix(), 1, false); err == nil {
		t.Fatal("missing consent")
	}
	_ = s.Start(ctx, time.Now().Add(time.Hour).Unix(), 1, true)
	ticket := s.begin(fixtureRequest(t), "p", "m", "openai")
	s.Stop()
	s.finish(ticket, ai.Result{Text: `{}`}, nil)
	if len(s.queue) != 0 {
		t.Fatal("in-flight capture survived stop")
	}
	sample := ticket.sample
	sample.Label = `{"observations":[{"from_frame":0,"to_frame":9}]}`
	if err := validateLabel(sample, sample.Label); err == nil {
		t.Fatal("out of range frame accepted")
	}
}

type failingRepo struct{ Repository }

func (f failingRepo) Save(context.Context, domain.TrainingSample, bool) error {
	return errors.New("fixture disk full")
}
func TestCollectionFailureDoesNotChangeProviderOutcome(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := New(failingRepo{storage.NewTrainingFiles(t.TempDir())}, nil)
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	defer func() { cancel(); <-done }()
	if err := s.Start(ctx, time.Now().Add(time.Hour).Unix(), 1, true); err != nil {
		t.Fatal(err)
	}
	p := &fakeProvider{}
	result, err := s.Wrap(p, "p", "m", "openai").Generate(ctx, fixtureRequest(t))
	if err != nil || result.Text == "" {
		t.Fatal("capture failure affected inference")
	}
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline) && s.Status().Failed == 0; {
		time.Sleep(time.Millisecond * 5)
	}
	if s.Status().Failed != 1 {
		t.Fatal("failed capture not counted")
	}
}
func TestTeacherInvalidatesApprovalsAndPreservesOriginal(t *testing.T) {
	ctx := context.Background()
	repo := storage.NewTrainingFiles(t.TempDir())
	s := New(repo, nil)
	sample := domain.TrainingSample{Version: 1, ID: "0123456789abcdef0123456789abcdef", Revision: 1, Candidate: "original", Label: "original", PrivacyApproved: true, Verdict: "accepted"}
	if err := repo.Save(ctx, sample, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Relabel(ctx, sample.ID, 1, domain.TrainingTeacher{Answer: `{"observations":[]}`, Model: "teacher"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, sample.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PrivacyApproved || got.Verdict != "pending" || got.Candidate != "original" || got.Revision != 2 || len(got.Teachers) != 1 {
		t.Fatal("teacher kept approval or lost history")
	}
	if err := s.Relabel(ctx, sample.ID, 1, domain.TrainingTeacher{Answer: `{}`}); !errors.Is(err, ErrConflict) {
		t.Fatal("stale teacher accepted")
	}
}
func TestCaptureDeadlineQuotaAndCorrectionExclusion(t *testing.T) {
	s := New(storage.NewTrainingFiles(t.TempDir()), nil)
	ctx := context.Background()
	if s.Start(ctx, time.Now().Add(9*time.Hour).Unix(), 1, true) == nil || s.Start(ctx, time.Now().Add(time.Hour).Unix(), 101, true) == nil {
		t.Fatal("unbounded session")
	}
	_ = s.Start(ctx, time.Now().Add(time.Hour).Unix(), 1, true)
	req := fixtureRequest(t)
	req.Training = nil
	if s.begin(req, "p", "m", "openai") != nil {
		t.Fatal("correction-only input captured")
	}
	req = fixtureRequest(t)
	if s.begin(req, "p", "m", "openai") == nil {
		t.Fatal("first capture")
	}
	req.Parts[0] = ai.TextPart("different")
	if s.begin(req, "p", "m", "openai") != nil {
		t.Fatal("limit exceeded")
	}
	s.mu.Lock()
	s.status.Until = time.Now().Add(-time.Second).Unix()
	s.reserved = 0
	s.mu.Unlock()
	if s.begin(req, "p", "m", "openai") != nil {
		t.Fatal("expired consent")
	}
}
