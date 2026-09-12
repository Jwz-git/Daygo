//go:build windows

package secrets

import (
	"context"
	"errors"
	"runtime"
	"unsafe"

	"github.com/Jwz-git/Daygo/internal/platform"
	"golang.org/x/sys/windows"
)

const (
	credTypeGeneric            = 1
	credPersistLocalMachine    = 2
	maxCredentialBlobSize      = 5 * 512
	credentialManagerCallFlags = 0
)

var (
	advapi32        = windows.NewLazySystemDLL("advapi32.dll")
	procCredWriteW  = advapi32.NewProc("CredWriteW")
	procCredReadW   = advapi32.NewProc("CredReadW")
	procCredDeleteW = advapi32.NewProc("CredDeleteW")
	procCredFree    = advapi32.NewProc("CredFree")
)

// credential mirrors the Windows CREDENTIALW structure from wincred.h.
// Pointer-sized fields intentionally use pointers/uintptr so the layout is
// correct on both 32-bit and 64-bit Windows.
type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

var _ platform.Secrets = (*CredentialManager)(nil)

// CredentialManager stores provider API keys as generic credentials in the
// current Windows user's Credential Manager.
type CredentialManager struct{}

// New returns the Windows Credential Manager implementation of
// platform.Secrets.
func New() *CredentialManager { return &CredentialManager{} }

func (m *CredentialManager) Set(ctx context.Context, providerID, secret string) error {
	if err := validateWindowsRequest(ctx, providerID); err != nil {
		return err
	}

	secretBytes := []byte(secret)
	if len(secretBytes) > maxCredentialBlobSize {
		return &SecretsError{Code: SecretInvalidArgument, Provider: providerID}
	}

	target, err := windows.UTF16PtrFromString(ServiceName(providerID))
	if err != nil {
		return &SecretsError{Code: SecretInvalidArgument, Provider: providerID}
	}
	account, err := windows.UTF16PtrFromString(keychainAccount)
	if err != nil {
		return &SecretsError{Code: SecretNative, Provider: providerID}
	}

	var blob *byte
	if len(secretBytes) != 0 {
		blob = &secretBytes[0]
	}
	entry := credential{
		Type:               credTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(secretBytes)),
		CredentialBlob:     blob,
		Persist:            credPersistLocalMachine,
		UserName:           account,
	}

	result, _, _ := procCredWriteW.Call(
		uintptr(unsafe.Pointer(&entry)),
		credentialManagerCallFlags,
	)
	runtime.KeepAlive(entry)
	runtime.KeepAlive(secretBytes)
	if result == 0 {
		return &SecretsError{Code: SecretNative, Provider: providerID}
	}
	return nil
}

func (m *CredentialManager) Get(ctx context.Context, providerID string) (string, error) {
	if err := validateWindowsRequest(ctx, providerID); err != nil {
		return "", err
	}

	target, err := windows.UTF16PtrFromString(ServiceName(providerID))
	if err != nil {
		return "", &SecretsError{Code: SecretInvalidArgument, Provider: providerID}
	}

	var entry *credential
	result, _, callErr := procCredReadW.Call(
		uintptr(unsafe.Pointer(target)),
		credTypeGeneric,
		credentialManagerCallFlags,
		uintptr(unsafe.Pointer(&entry)),
	)
	runtime.KeepAlive(target)
	if result == 0 {
		if errors.Is(callErr, windows.ERROR_NOT_FOUND) {
			return "", errNotFound(providerID)
		}
		return "", &SecretsError{Code: SecretNative, Provider: providerID}
	}
	if entry == nil {
		return "", &SecretsError{Code: SecretNative, Provider: providerID}
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(entry)))

	size := int(entry.CredentialBlobSize)
	if size < 0 || size > maxCredentialBlobSize || (size > 0 && entry.CredentialBlob == nil) {
		return "", &SecretsError{Code: SecretNative, Provider: providerID}
	}
	if size == 0 {
		return "", nil
	}
	return string(unsafe.Slice(entry.CredentialBlob, size)), nil
}

func (m *CredentialManager) Delete(ctx context.Context, providerID string) error {
	if err := validateWindowsRequest(ctx, providerID); err != nil {
		return err
	}

	target, err := windows.UTF16PtrFromString(ServiceName(providerID))
	if err != nil {
		return &SecretsError{Code: SecretInvalidArgument, Provider: providerID}
	}
	result, _, callErr := procCredDeleteW.Call(
		uintptr(unsafe.Pointer(target)),
		credTypeGeneric,
		credentialManagerCallFlags,
	)
	runtime.KeepAlive(target)
	if result == 0 {
		if errors.Is(callErr, windows.ERROR_NOT_FOUND) {
			return errNotFound(providerID)
		}
		return &SecretsError{Code: SecretNative, Provider: providerID}
	}
	return nil
}

func validateWindowsRequest(ctx context.Context, providerID string) error {
	if err := validateProvider(providerID); err != nil {
		return err
	}
	if ctx == nil || ctx.Err() != nil {
		return &SecretsError{Code: SecretNative, Provider: providerID}
	}
	return nil
}
