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
	Model       string `json:"model"`
	HasSecret   bool   `json:"hasSecret"`
}

// ProviderInputDTO is the create/update payload. Secret is the one field not
// stored in the database: an empty string keeps the existing key (clearing is
// DeleteProviderSecret), a non-empty one replaces it in the keychain.
type ProviderInputDTO struct {
	DisplayName string `json:"displayName"`
	Protocol    string `json:"protocol"`
	Endpoint    string `json:"endpoint"`
	Model       string `json:"model"`
	Secret      string `json:"secret"`
}

// ProviderRoutingDTO is the ordered routing chain: Chain[0] is the primary,
// the rest are fallbacks (decisions/providers-fallback-chain).
type ProviderRoutingDTO struct {
	Chain []string `json:"chain"`
}
