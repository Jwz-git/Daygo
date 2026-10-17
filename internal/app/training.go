package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
	"github.com/Jwz-git/Daygo/internal/analysis"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/training"
)

// DTOs deliberately expose no source paths, endpoint, or credential.
type TrainingStartDTO struct {
	Minutes int  `json:"minutes"`
	Limit   int  `json:"limit"`
	Consent bool `json:"consent"`
}
type TrainingReviewDTO struct {
	ID              string `json:"id"`
	Revision        int    `json:"revision"`
	Label           string `json:"label"`
	PrivacyApproved bool   `json:"privacyApproved"`
	Verdict         string `json:"verdict"`
}
type TrainingTeacherDTO struct {
	ID         string `json:"id"`
	Revision   int    `json:"revision"`
	ProviderID string `json:"providerId"`
	Model      string `json:"model"`
	Consent    bool   `json:"consent"`
}
type TrainingSampleDTO struct {
	ID              string                   `json:"id"`
	Revision        int                      `json:"revision"`
	Purpose         string                   `json:"purpose"`
	Model           string                   `json:"model"`
	Prompt          string                   `json:"prompt"`
	Schema          string                   `json:"schema"`
	Images          []string                 `json:"images"`
	Candidate       string                   `json:"candidate"`
	Teachers        []domain.TrainingTeacher `json:"teachers"`
	Label           string                   `json:"label"`
	Verdict         string                   `json:"verdict"`
	PrivacyApproved bool                     `json:"privacyApproved"`
	Group           string                   `json:"group"`
}

func (b *Backend) trainingSnapshot() *training.Service {
	b.trainingMu.Lock()
	defer b.trainingMu.Unlock()
	return b.training
}
func (b *Backend) ensureTraining() *training.Service {
	b.trainingMu.Lock()
	defer b.trainingMu.Unlock()
	if b.training == nil {
		b.training = training.New(b.store().TrainingFiles(), func() { b.emitter.Emit(EventTrainingUpdated, struct{}{}) })
		b.training.Validate = analysis.ValidateTrainingLabel
	}
	return b.training
}
func (b *Backend) trainingAccess() (*training.Service, error) {
	if writable, owner := b.instanceOwnership(); !writable || !owner {
		return nil, apperr.E(apperr.NotCaptureOwner, "training requires the writable capture owner", nil)
	}
	if b.store() == nil {
		return nil, apperr.E(apperr.DatabaseError, "training storage unavailable", nil)
	}
	return b.ensureTraining(), nil
}
func trainingError(err error) error {
	if err == nil {
		return nil
	}
	code := apperr.InvalidArgument
	if errors.Is(err, training.ErrConflict) {
		code = apperr.Conflict
	}
	if errors.Is(err, os.ErrNotExist) {
		code = apperr.NotFound
	}
	return apperr.E(code, "training operation failed; check input, approval, revision and storage quota", nil)
}
func (b *Backend) trainingContext() (context.Context, context.CancelFunc) {
	b.windowCtxMu.Lock()
	ctx := b.windowCtx
	b.windowCtxMu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, 5*time.Minute)
}
func (b *Backend) GetTrainingStatus() (training.Status, error) {
	s, err := b.trainingAccess()
	if err != nil {
		return training.Status{}, err
	}
	return s.Status(), nil
}
func (b *Backend) StartTrainingCapture(req TrainingStartDTO) error {
	s, err := b.trainingAccess()
	if err != nil {
		return err
	}
	if req.Minutes < 1 || req.Minutes > 480 {
		return trainingError(training.ErrInvalid)
	}
	ctx, cancel := b.trainingContext()
	defer cancel()
	return trainingError(s.Start(ctx, time.Now().Add(time.Duration(req.Minutes)*time.Minute).Unix(), req.Limit, req.Consent))
}
func (b *Backend) StopTrainingCapture() error {
	s, err := b.trainingAccess()
	if err != nil {
		return err
	}
	s.Stop()
	return nil
}
func (b *Backend) ListTrainingSamples() ([]domain.TrainingSummary, error) {
	s, err := b.trainingAccess()
	if err != nil {
		return nil, err
	}
	ctx, cancel := b.trainingContext()
	defer cancel()
	list, err := s.List(ctx)
	return list, trainingError(err)
}
func (b *Backend) GetTrainingSample(id string) (TrainingSampleDTO, error) {
	s, err := b.trainingAccess()
	if err != nil {
		return TrainingSampleDTO{}, err
	}
	ctx, cancel := b.trainingContext()
	defer cancel()
	sample, err := s.Get(ctx, id)
	if err != nil {
		return TrainingSampleDTO{}, trainingError(err)
	}
	dto := TrainingSampleDTO{ID: sample.ID, Revision: sample.Revision, Purpose: sample.Purpose, Model: sample.RequestedModel, Schema: string(sample.Schema), Candidate: sample.Candidate, Label: sample.Label, Verdict: sample.Verdict, PrivacyApproved: sample.PrivacyApproved, Group: sample.Group, Teachers: sample.Teachers, Images: []string{}}
	for _, p := range sample.Parts {
		if len(p.Image) > 0 {
			dto.Images = append(dto.Images, "data:"+p.MIME+";base64,"+base64.StdEncoding.EncodeToString(p.Image))
		} else {
			dto.Prompt += p.Text + "\n"
		}
	}
	return dto, nil
}
func (b *Backend) ReviewTrainingSample(req TrainingReviewDTO) error {
	s, err := b.trainingAccess()
	if err != nil {
		return err
	}
	ctx, cancel := b.trainingContext()
	defer cancel()
	return trainingError(s.Review(ctx, req.ID, req.Revision, req.Label, req.PrivacyApproved, req.Verdict))
}
func (b *Backend) DeleteTrainingSample(id string) error {
	s, err := b.trainingAccess()
	if err != nil {
		return err
	}
	ctx, cancel := b.trainingContext()
	defer cancel()
	return trainingError(s.Delete(ctx, id))
}
func (b *Backend) ExportTrainingSamples(ids []string) (string, error) {
	s, err := b.trainingAccess()
	if err != nil {
		return "", err
	}
	ctx, cancel := b.trainingContext()
	defer cancel()
	path, err := s.Export(ctx, ids)
	return path, trainingError(err)
}

// OpenTrainingExports opens only the store-owned export directory.
func (b *Backend) OpenTrainingExports() error {
	_, err := b.trainingAccess()
	if err != nil {
		return err
	}
	ctx, cancel := b.trainingContext()
	defer cancel()
	return trainingError(platform.OpenFolder(ctx, filepath.Join(filepath.Dir(b.store().Path()), "training", "exports")))
}
func (b *Backend) RelabelTrainingSample(req TrainingTeacherDTO) error {
	s, err := b.trainingAccess()
	if err != nil {
		return err
	}
	if !req.Consent || strings.TrimSpace(req.Model) == "" || req.ProviderID == "" {
		return trainingError(training.ErrInvalid)
	}
	ctx, cancel := b.trainingContext()
	defer cancel()
	sample, err := s.Get(ctx, req.ID)
	if err != nil {
		return trainingError(err)
	}
	if len(sample.Teachers) >= 4 {
		return trainingError(training.ErrInvalid)
	}
	if sample.Revision != req.Revision {
		return trainingError(training.ErrConflict)
	}
	repo, err := b.providerStore()
	if err != nil {
		return err
	}
	row, err := repo.Get(ctx, req.ProviderID)
	if err != nil {
		return trainingError(err)
	}
	model, err := resolveTestModel(row.Models, req.Model)
	if err != nil {
		return err
	}
	if err = b.providerSecrets(); err != nil {
		return err
	}
	secret, err := b.secrets.Get(ctx, row.ID)
	if err != nil && !secrets.IsNotFound(err) {
		return apperr.E(apperr.NativeUnavailable, "provider credential unavailable", nil)
	}
	provider, err := factory.NewClient(nil, factory.Config{Protocol: ai.Protocol(row.Protocol), Endpoint: row.Endpoint, Model: model, Secret: secret, UserAgent: row.UserAgent})
	if err != nil {
		return apperr.E(apperr.ProviderFailed, "teacher unavailable", nil)
	}
	request, err := training.RequestForSample(sample)
	if err != nil {
		return trainingError(err)
	}
	request.MaxImages = ai.ClampMaxImages(row.MaxImages)
	if err = request.Validate(); err != nil {
		return trainingError(err)
	}
	provider = playgroundResponseFilter{provider: provider, secret: secret}
	result, err := provider.Generate(ctx, request)
	if err != nil {
		return apperr.E(apperr.ProviderFailed, "teacher request failed", nil)
	}
	// The candidate stays pending even when its JSON parses; only human review approves it.
	if !json.Valid([]byte(result.Text)) {
		return trainingError(training.ErrInvalid)
	}
	return trainingError(s.Relabel(ctx, req.ID, req.Revision, domain.TrainingTeacher{Provider: row.ID, Model: model, ActualModel: result.Model, Protocol: row.Protocol, Answer: result.Text, CreatedAt: time.Now().Unix()}))
}
