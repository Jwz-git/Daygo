package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
)

func seedTraining(t *testing.T, b *Backend) domain.TrainingSample {
	t.Helper()
	s := domain.TrainingSample{Version: 1, ID: "0123456789abcdef0123456789abcdef", Revision: 1, Purpose: "transcribe", Group: "anonymous", Candidate: `{"observations":[]}`, Label: `{"observations":[]}`, Verdict: "pending", Schema: json.RawMessage(`{"type":"object","properties":{"observations":{"type":"array"}},"required":["observations"]}`), Parts: []domain.TrainingPart{{Text: "Anonymous fixture"}}}
	if err := b.store().TrainingFiles().Save(context.Background(), s, true); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestTrainingBindingOwnershipAndPathBoundary(t *testing.T) {
	b, _, _ := backendWithStoreAndSecrets(t)
	_, err := b.GetTrainingStatus()
	requireCode(t, err, apperr.NotCaptureOwner)
	requireCode(t, b.StartTrainingCapture(TrainingStartDTO{Minutes: 1, Limit: 1, Consent: true}), apperr.NotCaptureOwner)
	b = playgroundBackend(t)
	_, err = b.GetTrainingSample("../private")
	requireCode(t, err, apperr.InvalidArgument)
	requireCode(t, b.StartTrainingCapture(TrainingStartDTO{Minutes: 1, Limit: 1}), apperr.InvalidArgument)
	if err = b.StartTrainingCapture(TrainingStartDTO{Minutes: 1, Limit: 1, Consent: true}); err != nil {
		t.Fatal(err)
	}
	if err = b.StopTrainingCapture(); err != nil {
		t.Fatal(err)
	}
	status, err := b.GetTrainingStatus()
	if err != nil || status.Active {
		t.Fatal("stop failed")
	}
	sample := seedTraining(t, b)
	_, err = b.ExportTrainingSamples([]string{sample.ID})
	requireCode(t, err, apperr.InvalidArgument)
	if err = b.DeleteTrainingSample(sample.ID); err != nil {
		t.Fatal(err)
	}
	_, err = b.GetTrainingSample(sample.ID)
	requireCode(t, err, apperr.NotFound)
}
func TestTrainingTeacherRequiresConsentAndUsesSingleSelectedProvider(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Error("missing authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"teacher-fixture","choices":[{"finish_reason":"stop","message":{"content":"{\"observations\":[]}"}}]}`)
	}))
	defer server.Close()
	b := playgroundBackend(t)
	s := seedTraining(t, b)
	input := validProviderInput()
	input.Endpoint = server.URL
	input.Secret = "fixture-secret"
	id, err := b.AddProvider(input)
	if err != nil {
		t.Fatal(err)
	}
	req := TrainingTeacherDTO{ID: s.ID, Revision: 1, ProviderID: id, Model: "fixture-model"}
	requireCode(t, b.RelabelTrainingSample(req), apperr.InvalidArgument)
	if calls.Load() != 0 {
		t.Fatal("unconsented network call")
	}
	req.Consent = true
	missingModel := req
	missingModel.Model = ""
	requireCode(t, b.RelabelTrainingSample(missingModel), apperr.InvalidArgument)
	if calls.Load() != 0 {
		t.Fatal("implicit teacher model selected")
	}
	if err = b.RelabelTrainingSample(req); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("unexpected retry")
	}
	dto, err := b.GetTrainingSample(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(dto)
	if strings.Contains(string(wire), "fixture-secret") || dto.PrivacyApproved || dto.Verdict != "pending" || len(dto.Teachers) != 1 || dto.Revision != 2 {
		t.Fatal("unsafe teacher result")
	}
	requireCode(t, b.RelabelTrainingSample(req), apperr.Conflict)
	if calls.Load() != 1 {
		t.Fatal("stale request sent")
	}
}
