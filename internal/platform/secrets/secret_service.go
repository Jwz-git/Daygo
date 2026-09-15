package secrets

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var _ platform.Secrets = (*SecretService)(nil)

// secretToolRunner is deliberately narrow so command construction, exit-code
// mapping and secret redaction can be tested without a desktop keyring.
type secretToolRunner interface {
	output(ctx context.Context, path string, args []string, stdin string) ([]byte, error)
}

type execSecretTool struct{}

func (execSecretTool) output(ctx context.Context, path string, args []string, stdin string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	// Do not attach stderr: a keyring implementation may include attributes or
	// other sensitive diagnostic material in it.
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// SecretService stores API keys through libsecret's secret-tool client. The
// secret is supplied on stdin and therefore never appears in argv or errors.
type SecretService struct {
	path   string
	runner secretToolRunner
}

func newSecretService(path string, runner secretToolRunner) *SecretService {
	return &SecretService{path: path, runner: runner}
}

func (s *SecretService) Set(ctx context.Context, providerID, secret string) error {
	if err := validateProvider(providerID); err != nil {
		return err
	}
	_, err := s.run(ctx, providerID, secret,
		"store", "--label=Daygo API key",
		"service", ServiceName(providerID), "account", keychainAccount)
	return err
}

func (s *SecretService) Get(ctx context.Context, providerID string) (string, error) {
	if err := validateProvider(providerID); err != nil {
		return "", err
	}
	out, err := s.run(ctx, providerID, "",
		"lookup", "service", ServiceName(providerID), "account", keychainAccount)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func (s *SecretService) Delete(ctx context.Context, providerID string) error {
	if err := validateProvider(providerID); err != nil {
		return err
	}
	_, err := s.run(ctx, providerID, "",
		"clear", "service", ServiceName(providerID), "account", keychainAccount)
	return err
}

func (s *SecretService) run(ctx context.Context, providerID, stdin string, args ...string) ([]byte, error) {
	if s == nil || s.path == "" || s.runner == nil {
		return nil, &SecretsError{Code: SecretUnsupported, Provider: providerID}
	}
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	out, err := s.runner.output(ctx, s.path, args, stdin)
	if err == nil {
		return out, nil
	}
	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 &&
		(args[0] == "lookup" || args[0] == "clear") {
		return nil, errNotFound(providerID)
	}
	return nil, &SecretsError{Code: SecretNative, Provider: providerID}
}
