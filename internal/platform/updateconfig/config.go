package updateconfig

// These are public release parameters. The private signing key is held in the
// macOS keychain and the SPARKLE_ED25519_PRIVATE_KEY GitHub Actions secret.
const (
	FeedURL          = "https://github.com/Jwz-git/Daygo/releases/download/updates/appcast.xml"
	Ed25519PublicKey = "06+8of/d5uuNRZP3K7PPYQ8yCYG9BFdjwJ9P2PMj7Po="
)
