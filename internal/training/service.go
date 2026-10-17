// Package training collects explicitly authorized local model examples. It
// never writes production timeline data or sends data to a network destination.
package training

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"runtime/debug"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
)

var ErrInvalid = errors.New("invalid training request")
var ErrConflict = errors.New("training sample revision conflict")

type Repository interface {
	Group(context.Context) (string, error)
	Save(context.Context, domain.TrainingSample, bool) error
	Get(context.Context, string) (domain.TrainingSample, error)
	List(context.Context) ([]domain.TrainingSummary, error)
	Delete(context.Context, string) error
	Export(context.Context, []domain.TrainingSample) (string, error)
}
type Status struct {
	Active   bool  `json:"active"`
	Until    int64 `json:"until"`
	Limit    int   `json:"limit"`
	Captured int   `json:"captured"`
	Skipped  int   `json:"skipped"`
	Failed   int   `json:"failed"`
}
type Service struct {
	mu       sync.Mutex
	files    sync.Mutex
	repo     Repository
	queue    chan ticket
	notify   func()
	status   Status
	session  string
	group    string
	Validate func(domain.TrainingSample, string) error
	seen     map[string]bool
	reserved int
	bytes    int
}
type ticket struct {
	sample  domain.TrainingSample
	session string
}

func New(repo Repository, notify func()) *Service {
	if notify == nil {
		notify = func() {}
	}
	return &Service{repo: repo, queue: make(chan ticket, 2), notify: notify}
}
func newID() string          { var b [16]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func codeVersion() string {
	if b, ok := debug.ReadBuildInfo(); ok {
		v := "development"
		for _, s := range b.Settings {
			if s.Key == "vcs.revision" {
				v = s.Value
			}
			if s.Key == "vcs.modified" && s.Value == "true" {
				v += "+modified"
			}
		}
		return v
	}
	return "development"
}
func (s *Service) Start(ctx context.Context, until int64, limit int, consent bool) error {
	if !consent || limit < 1 || limit > 100 || until <= time.Now().Unix() || until > time.Now().Add(8*time.Hour).Unix() {
		return ErrInvalid
	}
	s.files.Lock()
	group, err := s.repo.Group(ctx)
	s.files.Unlock()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.session = newID()
	s.group = group
	s.status = Status{Active: true, Until: until, Limit: limit}
	s.seen = map[string]bool{}
	s.reserved = 0
	s.bytes = 0
	s.mu.Unlock()
	s.notify()
	return nil
}
func (s *Service) Stop() {
	s.mu.Lock()
	s.status.Active = false
	for {
		select {
		case <-s.queue:
			s.status.Skipped++
		default:
			s.mu.Unlock()
			s.notify()
			return
		}
	}
}
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.status
	v.Active = v.Active && time.Now().Unix() < v.Until && s.reserved < v.Limit
	return v
}
func (s *Service) begin(req ai.Request, provider, model, protocol string) *ticket {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.status.Active || time.Now().Unix() >= s.status.Until || s.reserved >= s.status.Limit || (req.Purpose != ai.PurposeTranscribe && req.Purpose != ai.PurposeCards) || req.Output == nil || req.Training == nil {
		return nil
	}
	sample := domain.TrainingSample{Version: 1, Session: s.session, Group: s.group, CreatedAt: time.Now().Unix(), Purpose: string(req.Purpose), CodeVersion: codeVersion(), Provider: provider, RequestedModel: model, Protocol: protocol, Schema: append(json.RawMessage(nil), req.Output.Schema...), MaxImages: req.MaxImages, MaxOutputTokens: req.MaxOutputTokens, Verdict: "pending", Revision: 1}
	var prompt []byte
	size := 0
	for _, p := range req.Parts {
		part := domain.TrainingPart{Text: p.Text()}
		if p.Kind() == ai.PartImage {
			part.MIME = string(p.MediaType())
			part.Image = p.Bytes()
			part.SHA256 = digest(part.Image)
		}
		size += len(part.Image) + len(part.Text)
		prompt = append(prompt, part.Text...)
		sample.Parts = append(sample.Parts, part)
	}
	sample.Context = *req.Training
	sample.Context.Categories = append([]string(nil), req.Training.Categories...)
	sample.PromptHash = digest(prompt)
	sample.SchemaHash = digest(sample.Schema)
	identity, _ := json.Marshal(struct {
		Session, Purpose string
		Context          domain.TrainingContext
		Parts            []domain.TrainingPart
		Schema           json.RawMessage
	}{s.session, sample.Purpose, sample.Context, sample.Parts, sample.Schema})
	sample.ID = digest(identity)[:32]
	if s.seen[sample.ID] {
		return nil
	}
	if size > 24<<20 || s.bytes+size > 512<<20 {
		s.status.Skipped++
		return nil
	}
	s.bytes += size
	s.seen[sample.ID] = true
	s.reserved++
	return &ticket{sample: sample, session: s.session}
}
func (s *Service) finish(t *ticket, result ai.Result, err error) {
	if t == nil {
		return
	}
	s.mu.Lock()
	defer func() { s.mu.Unlock(); s.notify() }()
	if t.session != s.session {
		return
	}
	if err != nil {
		delete(s.seen, t.sample.ID)
		s.reserved--
		s.status.Skipped++
		return
	}
	if !s.status.Active {
		s.status.Skipped++
		return
	}
	if len(result.Text) > 1<<20 {
		s.status.Skipped++
		return
	}
	t.sample.ActualModel = result.Model
	t.sample.Candidate = result.Text
	t.sample.Label = result.Text
	select {
	case s.queue <- *t:
	default:
		s.status.Skipped++
	}
}
func (s *Service) Run(ctx context.Context) {
	defer s.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-s.queue:
			s.mu.Lock()
			valid := t.session == s.session && s.status.Active
			s.mu.Unlock()
			if !valid {
				continue
			}
			s.files.Lock()
			err := s.repo.Save(ctx, t.sample, true)
			s.files.Unlock()
			s.mu.Lock()
			if t.session == s.session {
				if err != nil {
					s.status.Failed++
				} else {
					s.status.Captured++
				}
			}
			s.mu.Unlock()
			s.notify()
		}
	}
}

type observed struct {
	s                         *Service
	p                         ai.Provider
	provider, model, protocol string
	filter                    func(ai.Result) ai.Result
}

func (s *Service) Wrap(p ai.Provider, provider, model, protocol string, filters ...func(ai.Result) ai.Result) ai.Provider {
	var filter func(ai.Result) ai.Result
	if len(filters) > 0 {
		filter = filters[0]
	}
	return observed{s: s, p: p, provider: provider, model: model, protocol: protocol, filter: filter}
}
func (p observed) Generate(ctx context.Context, req ai.Request) (ai.Result, error) {
	t := p.s.begin(req, p.provider, p.model, p.protocol)
	result, err := p.p.Generate(ctx, req)
	snapshot := result
	if t != nil && p.filter != nil {
		snapshot = p.filter(snapshot)
	}
	p.s.finish(t, snapshot, err)
	return result, err
}
func (s *Service) List(ctx context.Context) ([]domain.TrainingSummary, error) {
	s.files.Lock()
	defer s.files.Unlock()
	return s.repo.List(ctx)
}
func (s *Service) Get(ctx context.Context, id string) (domain.TrainingSample, error) {
	s.files.Lock()
	defer s.files.Unlock()
	return s.repo.Get(ctx, id)
}
func (s *Service) Delete(ctx context.Context, id string) error {
	s.files.Lock()
	err := s.repo.Delete(ctx, id)
	s.files.Unlock()
	if err == nil {
		s.notify()
	}
	return err
}
func validateLabel(sample domain.TrainingSample, label string) error {
	if len(label) > 1<<20 || ai.ValidateJSON(json.RawMessage(label), ai.OutputSchema{Name: "training", Schema: sample.Schema, Strict: true}) != nil {
		return ErrInvalid
	}
	if sample.Purpose == string(ai.PurposeTranscribe) {
		var v struct {
			Observations []struct {
				From int `json:"from_frame"`
				To   int `json:"to_frame"`
			}
		}
		if json.Unmarshal([]byte(label), &v) != nil {
			return ErrInvalid
		}
		images := 0
		for _, p := range sample.Parts {
			if len(p.Image) > 0 {
				images++
			}
		}
		for _, o := range v.Observations {
			if o.From < 0 || o.To < o.From || o.To >= images {
				return ErrInvalid
			}
		}
	}
	return nil
}
func (s *Service) Review(ctx context.Context, id string, revision int, label string, privacy bool, verdict string) error {
	if len(label) > 1<<20 {
		return ErrInvalid
	}
	if verdict != "pending" && verdict != "accepted" && verdict != "rejected" {
		return ErrInvalid
	}
	s.files.Lock()
	sample, err := s.repo.Get(ctx, id)
	if err == nil && sample.Revision != revision {
		err = ErrConflict
	}
	if err == nil && verdict == "accepted" {
		err = validateLabel(sample, label)
		if err == nil && s.Validate != nil {
			err = s.Validate(sample, label)
		}
	}
	if err == nil {
		sample.Label = label
		sample.Verdict = verdict
		sample.PrivacyApproved = privacy
		sample.Revision++
		err = s.repo.Save(ctx, sample, false)
	}
	s.files.Unlock()
	if err == nil {
		s.notify()
	}
	return err
}
func (s *Service) Export(ctx context.Context, ids []string) (string, error) {
	if len(ids) == 0 || len(ids) > 100 {
		return "", ErrInvalid
	}
	s.files.Lock()
	defer s.files.Unlock()
	samples := []domain.TrainingSample{}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return "", ErrInvalid
		}
		seen[id] = true
		sample, err := s.repo.Get(ctx, id)
		if err != nil {
			return "", err
		}
		if !sample.PrivacyApproved || sample.Verdict != "accepted" {
			return "", ErrInvalid
		}
		if err := validateLabel(sample, sample.Label); err != nil {
			return "", err
		}
		if s.Validate != nil {
			if err := s.Validate(sample, sample.Label); err != nil {
				return "", err
			}
		}
		samples = append(samples, sample)
	}
	return s.repo.Export(ctx, samples)
}

// Relabel keeps prior candidates and invalidates both approvals. The revision
// check prevents a slow teacher request from overwriting a concurrent review.
func (s *Service) Relabel(ctx context.Context, id string, revision int, teacher domain.TrainingTeacher) error {
	if len(teacher.Answer) > 1<<20 {
		return ErrInvalid
	}
	s.files.Lock()
	sample, err := s.repo.Get(ctx, id)
	if err == nil && sample.Revision != revision {
		err = ErrConflict
	}
	if err == nil && len(sample.Teachers) >= 4 {
		err = ErrInvalid
	}
	if err == nil {
		sample.Teachers = append(sample.Teachers, teacher)
		sample.Label = teacher.Answer
		sample.PrivacyApproved = false
		sample.Verdict = "pending"
		sample.Revision++
		err = s.repo.Save(ctx, sample, false)
	}
	s.files.Unlock()
	if err == nil {
		s.notify()
	}
	return err
}

func RequestForSample(sample domain.TrainingSample) (ai.Request, error) {
	req := ai.Request{Purpose: ai.Purpose(sample.Purpose), Output: &ai.OutputSchema{Name: "daygo_training", Schema: sample.Schema, Strict: true}, MaxImages: sample.MaxImages, MaxOutputTokens: sample.MaxOutputTokens}
	for _, p := range sample.Parts {
		if len(p.Image) > 0 {
			part, err := ai.ImagePart(ai.MediaType(p.MIME), p.Image)
			if err != nil {
				return req, err
			}
			req.Parts = append(req.Parts, part)
		} else {
			req.Parts = append(req.Parts, ai.TextPart(p.Text))
		}
	}
	return req, req.Validate()
}
