package app

import (
	"context"
	"strings"

	daygoai "github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
)

// ProviderModelsRequestDTO addresses either a saved provider (ProviderID
// non-empty — the secret then comes from the keychain) or an unsaved draft
// (Protocol/Endpoint/Secret — the TestProviderConnection pattern, the draft
// secret only lives in Go memory for this call).
type ProviderModelsRequestDTO struct {
	ProviderID string `json:"providerId"`
	Protocol   string `json:"protocol"`
	Endpoint   string `json:"endpoint"`
	Secret     string `json:"secret"`
}

// ProviderModelsResultDTO reports the listing outcome. Following the probe
// binding's idiom, a failed listing is a result, not an error: the model field
// stays editable and the message shows inline.
type ProviderModelsResultDTO struct {
	OK        bool     `json:"ok"`
	Models    []string `json:"models"`
	ErrorCode string   `json:"errorCode"`
	Message   string   `json:"message"`
}

// ListProviderModels fetches the model list for one provider (saved or draft).
// One request, 15-second deadline, no retry, no caching — the list is
// user-triggered and small; the decision to skip a cache is deliberate.
func (b *Backend) ListProviderModels(req ProviderModelsRequestDTO) (ProviderModelsResultDTO, error) {
	var protocol daygoai.Protocol
	var endpoint, secret string

	if id := strings.TrimSpace(req.ProviderID); id != "" {
		repo, err := b.providerStore()
		if err != nil {
			return ProviderModelsResultDTO{}, err
		}
		if err := b.providerSecrets(); err != nil {
			return ProviderModelsResultDTO{}, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), providersTimeout)
		defer cancel()
		row, err := repo.Get(ctx, id)
		if err != nil {
			return ProviderModelsResultDTO{}, mapStorageError("get provider", err)
		}
		stored, err := b.secrets.Get(ctx, id)
		if err != nil {
			if secrets.IsNotFound(err) {
				return ProviderModelsResultDTO{}, apperr.E(apperr.InvalidArgument, "no api key stored for this provider", nil)
			}
			return ProviderModelsResultDTO{}, apperr.E(apperr.NativeUnavailable, "reading the stored api key failed", nil)
		}
		protocol = daygoai.Protocol(row.Protocol)
		endpoint = row.Endpoint
		secret = stored
	} else {
		protocol = daygoai.Protocol(strings.TrimSpace(req.Protocol))
		endpoint = strings.TrimSpace(req.Endpoint)
		secret = strings.TrimSpace(req.Secret)
	}

	models, err := daygoai.ListModels(context.Background(), protocol, endpoint, secret)
	if err != nil {
		return ProviderModelsResultDTO{
			OK:        false,
			Models:    []string{},
			ErrorCode: string(daygoai.ErrorKindOf(err)),
			Message:   strings.TrimPrefix(err.Error(), "ai: "),
		}, nil
	}
	return ProviderModelsResultDTO{OK: true, Models: models}, nil
}
