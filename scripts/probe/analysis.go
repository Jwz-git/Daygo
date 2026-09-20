// Command probe-analysis runs the production analysis pipeline against the
// provider currently routed at the top of the configured chain. It reads a
// real batch, runs the production transcription + card-generation code paths
// (prompt, schema, retry and fallback included), but never writes the
// Daygo database or recordings.
//
// The name "probe-analysis" is on purpose: this tool is provider-agnostic.
// The old filename (probe-deepseek-vision.go) implied the script only knew
// about DeepSeek, but it actually consults settings.ProvidersRouting via
// internal/ai/factory.NewClient, so any provider the user has configured —
// OpenAI, Anthropic, DeepSeek, a local model — runs through the same code
// path. Misleading filenames breed stale mental models.
//
// Run from the repository root:
//
//	go run ./scripts/probe/analysis.go
//
// The script sends only after confirmation. API keys and model text are never
// printed unless explicitly requested.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
	"github.com/Jwz-git/Daygo/internal/analysis"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/settings"
	"github.com/Jwz-git/Daygo/internal/storage"
)

type providerConfig struct {
	id, name, protocol, endpoint, model string
}

type probeChainSource struct {
	store      *storage.Store
	selectedID string
	selected   string
}

// ImageCap reports the selected provider's configured image cap, or 0 for the
// default.
func (p probeChainSource) ImageCap(ctx context.Context) int {
	rows, err := p.store.Providers().List(ctx)
	if err != nil {
		return 0
	}
	for _, row := range rows {
		if row.ID == p.selectedID && row.MaxImages > 0 {
			return row.MaxImages
		}
	}
	return 0
}

type stagingMedia struct{ root string }

var inputReader = bufio.NewReader(os.Stdin)

func main() {
	ctx := context.Background()
	root, err := supportDir()
	if err != nil {
		fatal("startup: resolve Daygo support directory", err)
	}
	store, err := storage.Open(ctx, storage.Options{Dir: root})
	if err != nil {
		fatal("startup: open Daygo database", err)
	}
	defer func() { _ = store.Close() }()

	provider, err := configuredProvider(ctx, store)
	if err != nil {
		fatal("provider: load current provider and routing", err)
	}
	fmt.Printf("Provider: %s (%s)\nEndpoint: %s\nModel: %s\n", provider.name, provider.protocol, provider.endpoint, provider.model)
	fmt.Printf("Database mode: %s (read-only probe; no rows will change)\n", store.Mode())

	secret, err := secrets.New().Get(ctx, provider.id)
	if err != nil || strings.TrimSpace(secret) == "" {
		fmt.Println("No usable key was read from the keychain.")
		secret, err = readHidden("API key (input is hidden): ")
		if err != nil {
			fatal("credentials: read API key", err)
		}
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		fatal("credentials: validate API key", errors.New("API key is empty"))
	}

	batch, frames, err := chooseBatch(ctx, store)
	if err != nil {
		fatal("batch: select batch and frames", err)
	}
	language, err := outputLanguage(ctx, store)
	if err != nil {
		fatal("settings: load LLM output language", err)
	}
	fmt.Printf("LLM output language setting: %q (empty means model default)\n", language)
	if override, err := readLine("Output language override [Enter keeps setting, e.g. zh-CN]: "); err == nil {
		if override = strings.TrimSpace(override); override != "" {
			language = override
		}
	}
	fmt.Printf("Selected batch %d: %d frame(s), %s – %s, status=%s\n",
		batch.ID, len(frames), batch.Start.Local().Format(time.RFC3339), batch.End.Local().Format(time.RFC3339), batch.Status)
	if !askYesNo("Run production transcription and card generation now? [Y/n] ", true) {
		fmt.Println("Canceled; no request was sent.")
		return
	}

	chainSource := probeChainSource{store: store, selectedID: provider.id, selected: secret}
	service, err := analysis.New(analysis.Config{
		Store:      store.Analysis(),
		Cards:      store.Cards(),
		Categories: store.Categories(),
		Providers:  chainSource,
		Media:      stagingMedia{root: filepath.Join(root, "recordings")},
		Language:   func(context.Context) string { return language },
		Location:   store.Location(),
		Workers:    2,
	})
	if err != nil {
		fatal("analysis: build production service", err)
	}

	fmt.Println("Running the same internal transcription and card-generation functions as the backend...")
	result, err := service.Probe(ctx, frames)
	if err != nil {
		fatal("analysis: "+probeStage(err), probeDetail(err))
	}
	if len(result.Cards) != 1 {
		fatal("analysis: final card validation", fmt.Errorf("production pipeline returned %d card(s); expected one", len(result.Cards)))
	}
	fmt.Printf("PASS: %d observation(s), %d card(s)\n\n", len(result.Observations), len(result.Cards))
	printCard(result.Cards[0], len(result.Observations))
}

func supportDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Daygo"), nil
}

func configuredProvider(ctx context.Context, store *storage.Store) (providerConfig, error) {
	rows, err := store.Providers().List(ctx)
	if err != nil {
		return providerConfig{}, err
	}
	if len(rows) == 0 {
		return providerConfig{}, errors.New("no provider is configured")
	}
	snapshot, err := settings.New(store.Settings()).Load(ctx)
	if err != nil {
		return providerConfig{}, err
	}
	byID := make(map[string]storage.Provider, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	var selected storage.Provider
	var selectedModel string
	for _, slot := range snapshot.ProvidersRouting.Chain {
		if row, ok := byID[slot.ProviderID]; ok {
			selected = row
			selectedModel = slot.Model
			break
		}
	}
	if selected.ID == "" {
		selected = rows[0]
		fmt.Println("Warning: routing chain did not resolve; using the first provider row.")
	}
	if selectedModel == "" && len(selected.Models) > 0 {
		selectedModel = selected.Models[0]
	}
	return providerConfig{
		id: selected.ID, name: selected.DisplayName, protocol: selected.Protocol,
		endpoint: strings.TrimRight(selected.Endpoint, "/"), model: selectedModel,
	}, nil
}

func outputLanguage(ctx context.Context, store *storage.Store) (string, error) {
	snapshot, err := settings.New(store.Settings()).Load(ctx)
	if err != nil {
		return "", err
	}
	return snapshot.OutputLanguage, nil
}

func chooseBatch(ctx context.Context, store *storage.Store) (storage.Batch, []storage.AnalysisFrame, error) {
	batches, err := store.Analysis().BatchesInRange(ctx, time.Unix(0, 0), time.Now().Add(24*time.Hour))
	if err != nil {
		return storage.Batch{}, nil, err
	}
	if len(batches) == 0 {
		return storage.Batch{}, nil, errors.New("no analysis batch exists")
	}
	// Prefer the newest failed batch: it is the useful case when diagnosing a
	// retry failure. If none failed, use the newest batch of any status.
	selected := batches[len(batches)-1]
	for i := len(batches) - 1; i >= 0; i-- {
		if batches[i].Status == storage.BatchFailed || batches[i].Status == storage.BatchFailedEmpty {
			selected = batches[i]
			break
		}
	}
	start := len(batches) - 8
	if start < 0 {
		start = 0
	}
	fmt.Println("Recent batches:")
	for _, batch := range batches[start:] {
		fmt.Printf("  id=%d status=%s attempts=%d window=%s – %s\n", batch.ID, batch.Status, batch.Attempts,
			batch.Start.Local().Format("01-02 15:04"), batch.End.Local().Format("15:04"))
	}
	line, err := readLine(fmt.Sprintf("Batch id [default %d]: ", selected.ID))
	if err != nil {
		return storage.Batch{}, nil, err
	}
	if value := strings.TrimSpace(line); value != "" {
		id, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil {
			return storage.Batch{}, nil, fmt.Errorf("batch id %q is not an integer", value)
		}
		found := false
		for _, batch := range batches {
			if batch.ID == id {
				selected = batch
				found = true
				break
			}
		}
		if !found {
			return storage.Batch{}, nil, fmt.Errorf("batch %d was not found", id)
		}
	}
	frames, err := store.Analysis().FramesForBatch(ctx, selected.ID)
	if err != nil {
		return storage.Batch{}, nil, err
	}
	if len(frames) == 0 {
		return storage.Batch{}, nil, fmt.Errorf("batch %d has no frames", selected.ID)
	}
	return selected, frames, nil
}

func (p probeChainSource) AnalysisChain(ctx context.Context) (*ai.Chain, error) {
	rows, err := p.store.Providers().List(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, err := settings.New(p.store.Settings()).Load(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]storage.Provider, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	entries := make([]ai.ChainEntry, 0, len(snapshot.ProvidersRouting.Chain))
	for _, slot := range snapshot.ProvidersRouting.Chain {
		row, ok := byID[slot.ProviderID]
		if !ok {
			continue
		}
		model := slot.Model
		if model == "" && len(row.Models) > 0 {
			model = row.Models[0]
		}
		if model == "" {
			continue
		}
		secret, secretErr := secrets.New().Get(ctx, slot.ProviderID)
		if slot.ProviderID == p.selectedID && p.selected != "" {
			secret = p.selected
			secretErr = nil
		}
		if secretErr != nil && !secrets.IsNotFound(secretErr) {
			return nil, fmt.Errorf("provider %s keychain lookup: %w", slot.ProviderID, secretErr)
		}
		provider, err := factory.NewClient(nil, factory.Config{
			Protocol: ai.Protocol(row.Protocol), Endpoint: row.Endpoint,
			Model: model, Secret: secret,
		})
		if err != nil {
			continue
		}
		entries = append(entries, ai.ChainEntry{ID: row.ID + "\x1f" + model, Provider: ai.WithRetry(provider, ai.DefaultRetryPolicy())})
	}
	if len(entries) == 0 {
		return nil, ai.ErrNoProvider
	}
	return ai.NewChain(entries, 0), nil
}

func (m stagingMedia) FrameBytes(ctx context.Context, segmentPath string, frameIndex int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if frameIndex != 0 {
		return nil, fmt.Errorf("staging frames require frame_index 0, got %d", frameIndex)
	}
	path := filepath.Join(m.root, filepath.FromSlash(segmentPath))
	rel, err := filepath.Rel(m.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("frame path escapes recordings directory")
	}
	return os.ReadFile(path)
}

func probeStage(err error) string {
	var probeErr *analysis.ProbeError
	if errors.As(err, &probeErr) {
		return probeErr.Stage
	}
	return "unknown stage"
}

func probeDetail(err error) error {
	kind := ai.ErrorKindOf(err)
	status := ai.HTTPStatusOf(err)
	if kind != "" {
		if status != 0 {
			return fmt.Errorf("%s (HTTP %d): %w", kind, status, err)
		}
		return fmt.Errorf("%s: %w", kind, err)
	}
	return err
}

func printCard(card domain.CardShell, observations int) {
	fmt.Println("Final card (not written to database):")
	fmt.Printf("  time:          %s – %s\n", card.Start, card.End)
	fmt.Printf("  category:      %s\n", card.Category)
	fmt.Printf("  subcategory:   %s\n", card.Subcategory)
	fmt.Printf("  title:         %s\n", card.Title)
	fmt.Printf("  summary:       %s\n", card.Summary)
	fmt.Printf("  detailed:      %s\n", card.DetailedSummary)
	var metadata struct {
		AppSites *struct {
			Primary   *string `json:"primary"`
			Secondary *string `json:"secondary"`
		} `json:"appSites"`
		Distractions   []string        `json:"distractions"`
		ActivityPoints []activityPoint `json:"activityPoints"`
	}
	if err := json.Unmarshal([]byte(card.Metadata), &metadata); err == nil {
		var sites []string
		if metadata.AppSites != nil {
			for _, site := range []*string{metadata.AppSites.Primary, metadata.AppSites.Secondary} {
				if site != nil && *site != "" {
					sites = append(sites, *site)
				}
			}
		}
		fmt.Printf("  app/sites:     %s\n", strings.Join(sites, ", "))
		fmt.Printf("  distractions:  %s\n", strings.Join(metadata.Distractions, ", "))
		fmt.Printf("  activityPoints: %d\n", len(metadata.ActivityPoints))
		for _, point := range metadata.ActivityPoints {
			fmt.Printf("    - %s: %s\n", point.Time, point.Description)
		}
	}
	fmt.Printf("  source:         %d transcription observation(s)\n", observations)
}

type activityPoint struct {
	Time        string `json:"time"`
	Description string `json:"description"`
}

func readHidden(prompt string) (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return "", errors.New("API key input requires an interactive terminal")
	}
	fmt.Print(prompt)
	if err := exec.Command("stty", "-echo").Run(); err != nil {
		return "", errors.New("cannot disable terminal echo for API key input")
	}
	defer func() {
		_ = exec.Command("stty", "echo").Run()
		fmt.Println()
	}()
	return readLine("")
}

func readLine(prompt string) (string, error) {
	fmt.Print(prompt)
	return inputReader.ReadString('\n')
}

func askYesNo(prompt string, defaultYes bool) bool {
	line, err := readLine(prompt)
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return defaultYes
	}
}

func fatal(stage string, err error) {
	fmt.Fprintf(os.Stderr, "FAIL [%s]: %v\n", stage, err)
	os.Exit(1)
}
