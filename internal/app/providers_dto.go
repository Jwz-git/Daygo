package app

// Provider DTOs mirror docs/05 §5.5.2. Secret handling follows the fixed rules
// (docs/07 §7.3): a DTO never carries a secret out, and ProviderInputDTO.Secret
// is write-only with "" meaning "keep the stored key".

// ProviderDTO is one configured provider. HasSecret is derived from the
// keychain on read; the value itself never crosses this boundary.
type ProviderDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Protocol    string `json:"protocol"`
	Endpoint    string `json:"endpoint"`
	// Models is the ordered list of models configured under this provider's one
	// endpoint and key (decisions/providers-multi-model). Models[0] is the
	// default a routing entry with an empty model resolves to.
	Models []string `json:"models"`
	// MaxImages caps the image parts of one request to this provider; 0 means
	// the built-in default. Low-limit gateways need it lowered, and the
	// recognition enhancement (one frame → five images) may need it raised.
	MaxImages int  `json:"maxImages"`
	HasSecret bool `json:"hasSecret"`
}

// ProviderInputDTO is the create/update payload. Secret is the one field not
// stored in the database: an empty string keeps the existing key (clearing is
// DeleteProviderSecret), a non-empty one replaces it in the keychain.
type ProviderInputDTO struct {
	DisplayName string   `json:"displayName"`
	Protocol    string   `json:"protocol"`
	Endpoint    string   `json:"endpoint"`
	Models      []string `json:"models"`
	MaxImages   int      `json:"maxImages"`
	Secret      string   `json:"secret"`
}

// ProviderRoutingEntryDTO is one (provider, model) pair in the chain. Model ""
// resolves to the provider's first configured model
// (decisions/providers-multi-model).
type ProviderRoutingEntryDTO struct {
	ProviderID string `json:"providerId"`
	Model      string `json:"model"`
}

// ProviderRoutingDTO is the ordered routing chain: Chain[0] is the primary,
// the rest are fallbacks (decisions/providers-fallback-chain). Each entry is a
// (provider, model) pair, so one provider can appear under several models.
type ProviderRoutingDTO struct {
	Chain []ProviderRoutingEntryDTO `json:"chain"`
}
