package runtime

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	deviceIdentityVersion = 1
	deviceIdentityFile    = "device.json"
	deviceAuthFile        = "device-auth.json"
)

type gatewayDeviceIdentity struct {
	DeviceID      string
	PublicKeyPEM  string
	PrivateKeyPEM string
}

type storedDeviceIdentity struct {
	Version       int    `json:"version"`
	DeviceID      string `json:"deviceId"`
	PublicKeyPEM  string `json:"publicKeyPem"`
	PrivateKeyPEM string `json:"privateKeyPem"`
	CreatedAtMS   int64  `json:"createdAtMs,omitempty"`
}

type deviceAuthStore struct {
	Version  int                             `json:"version"`
	DeviceID string                          `json:"deviceId"`
	Tokens   map[string]deviceAuthTokenEntry `json:"tokens"`
}

type deviceAuthTokenEntry struct {
	Token       string   `json:"token"`
	Role        string   `json:"role"`
	Scopes      []string `json:"scopes,omitempty"`
	UpdatedAtMS int64    `json:"updatedAtMs"`
}

type deviceAuthPayloadParams struct {
	Version    string
	DeviceID   string
	ClientID   string
	ClientMode string
	Role       string
	Scopes     []string
	SignedAtMS int64
	Token      string
	Nonce      string
}

func resolveStateDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("OPENCLAW_STATE_DIR")); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv("CLAWDBOT_STATE_DIR")); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("resolve state dir: empty user home")
	}
	return filepath.Join(home, ".openclaw"), nil
}

func resolveIdentityDir() (string, error) {
	stateDir, err := resolveStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "identity"), nil
}

func identityFilePath() (string, error) {
	identityDir, err := resolveIdentityDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(identityDir, deviceIdentityFile), nil
}

func deviceAuthFilePath() (string, error) {
	identityDir, err := resolveIdentityDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(identityDir, deviceAuthFile), nil
}

func loadOrCreateDeviceIdentity() (*gatewayDeviceIdentity, error) {
	path, err := identityFilePath()
	if err != nil {
		return nil, err
	}

	if identity, ok := loadDeviceIdentity(path); ok {
		return identity, nil
	}

	identity, err := generateDeviceIdentity()
	if err != nil {
		return nil, err
	}
	if err := writeDeviceIdentity(path, identity); err != nil {
		return nil, err
	}
	return identity, nil
}

func loadDeviceIdentity(path string) (*gatewayDeviceIdentity, bool) {
	body, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, false
	}
	var stored storedDeviceIdentity
	if err := json.Unmarshal(body, &stored); err != nil {
		return nil, false
	}
	if stored.Version != deviceIdentityVersion {
		return nil, false
	}
	if strings.TrimSpace(stored.PublicKeyPEM) == "" || strings.TrimSpace(stored.PrivateKeyPEM) == "" {
		return nil, false
	}

	derivedID, err := deriveDeviceIDFromPublicKeyPEM(stored.PublicKeyPEM)
	if err != nil || strings.TrimSpace(derivedID) == "" {
		return nil, false
	}

	identity := &gatewayDeviceIdentity{
		DeviceID:      derivedID,
		PublicKeyPEM:  stored.PublicKeyPEM,
		PrivateKeyPEM: stored.PrivateKeyPEM,
	}

	if strings.TrimSpace(stored.DeviceID) != derivedID {
		_ = writeDeviceIdentity(path, identity)
	}
	return identity, true
}

func generateDeviceIdentity() (*gatewayDeviceIdentity, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 keypair: %w", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}

	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	if len(publicPEM) == 0 || len(privatePEM) == 0 {
		return nil, fmt.Errorf("encode keypair to pem")
	}

	deviceID, err := deriveDeviceIDFromPublicKeyPEM(string(publicPEM))
	if err != nil {
		return nil, err
	}

	return &gatewayDeviceIdentity{
		DeviceID:      deviceID,
		PublicKeyPEM:  string(publicPEM),
		PrivateKeyPEM: string(privatePEM),
	}, nil
}

func writeDeviceIdentity(path string, identity *gatewayDeviceIdentity) error {
	if identity == nil {
		return fmt.Errorf("device identity is nil")
	}
	if strings.TrimSpace(identity.DeviceID) == "" {
		return fmt.Errorf("device identity id is empty")
	}

	stored := storedDeviceIdentity{
		Version:       deviceIdentityVersion,
		DeviceID:      identity.DeviceID,
		PublicKeyPEM:  identity.PublicKeyPEM,
		PrivateKeyPEM: identity.PrivateKeyPEM,
		CreatedAtMS:   time.Now().UnixMilli(),
	}
	return writeJSONFile0600(path, stored)
}

func readDeviceAuthStore(path string) (*deviceAuthStore, error) {
	body, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var store deviceAuthStore
	if err := json.Unmarshal(body, &store); err != nil {
		return nil, nil
	}
	if store.Version != deviceIdentityVersion {
		return nil, nil
	}
	if strings.TrimSpace(store.DeviceID) == "" {
		return nil, nil
	}
	if store.Tokens == nil {
		store.Tokens = map[string]deviceAuthTokenEntry{}
	}
	return &store, nil
}

func loadDeviceAuthToken(deviceID, role string) (*deviceAuthTokenEntry, error) {
	role = strings.TrimSpace(role)
	deviceID = strings.TrimSpace(deviceID)
	if role == "" || deviceID == "" {
		return nil, nil
	}
	path, err := deviceAuthFilePath()
	if err != nil {
		return nil, err
	}
	store, err := readDeviceAuthStore(path)
	if err != nil || store == nil {
		return nil, err
	}
	if strings.TrimSpace(store.DeviceID) != deviceID {
		return nil, nil
	}
	entry, ok := store.Tokens[role]
	if !ok {
		return nil, nil
	}
	if strings.TrimSpace(entry.Token) == "" {
		return nil, nil
	}
	return &entry, nil
}

func storeDeviceAuthToken(deviceID, role, token string, scopes []string) error {
	role = strings.TrimSpace(role)
	deviceID = strings.TrimSpace(deviceID)
	token = strings.TrimSpace(token)
	if role == "" || deviceID == "" || token == "" {
		return nil
	}

	path, err := deviceAuthFilePath()
	if err != nil {
		return err
	}
	store, err := readDeviceAuthStore(path)
	if err != nil {
		return err
	}
	if store == nil || strings.TrimSpace(store.DeviceID) != deviceID {
		store = &deviceAuthStore{
			Version:  deviceIdentityVersion,
			DeviceID: deviceID,
			Tokens:   map[string]deviceAuthTokenEntry{},
		}
	}
	store.Tokens[role] = deviceAuthTokenEntry{
		Token:       token,
		Role:        role,
		Scopes:      normalizeScopes(scopes),
		UpdatedAtMS: time.Now().UnixMilli(),
	}
	return writeJSONFile0600(path, store)
}

func clearDeviceAuthToken(deviceID, role string) error {
	role = strings.TrimSpace(role)
	deviceID = strings.TrimSpace(deviceID)
	if role == "" || deviceID == "" {
		return nil
	}
	path, err := deviceAuthFilePath()
	if err != nil {
		return err
	}
	store, err := readDeviceAuthStore(path)
	if err != nil || store == nil {
		return err
	}
	if strings.TrimSpace(store.DeviceID) != deviceID {
		return nil
	}
	if _, ok := store.Tokens[role]; !ok {
		return nil
	}
	delete(store.Tokens, role)
	return writeJSONFile0600(path, store)
}

func writeJSONFile0600(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')

	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(cleanPath), err)
	}
	if err := os.WriteFile(cleanPath, body, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", cleanPath, err)
	}
	_ = os.Chmod(cleanPath, 0o600)
	return nil
}

func buildDeviceAuthPayload(params deviceAuthPayloadParams) string {
	version := strings.TrimSpace(params.Version)
	if version == "" {
		if strings.TrimSpace(params.Nonce) != "" {
			version = "v2"
		} else {
			version = "v1"
		}
	}

	token := strings.TrimSpace(params.Token)
	scopes := strings.Join(params.Scopes, ",")
	parts := []string{
		version,
		params.DeviceID,
		params.ClientID,
		params.ClientMode,
		params.Role,
		scopes,
		fmt.Sprintf("%d", params.SignedAtMS),
		token,
	}
	if version == "v2" {
		parts = append(parts, strings.TrimSpace(params.Nonce))
	}
	return strings.Join(parts, "|")
}

func signDevicePayload(privateKeyPEM string, payload string) (string, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("decode private key pem")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return "", fmt.Errorf("private key is not ed25519")
	}
	signature := ed25519.Sign(privateKey, []byte(payload))
	return base64.RawURLEncoding.EncodeToString(signature), nil
}

func publicKeyRawBase64URLFromPEM(publicKeyPEM string) (string, error) {
	raw, err := derivePublicKeyRawFromPEM(publicKeyPEM)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func deriveDeviceIDFromPublicKeyPEM(publicKeyPEM string) (string, error) {
	raw, err := derivePublicKeyRawFromPEM(publicKeyPEM)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func derivePublicKeyRawFromPEM(publicKeyPEM string) ([]byte, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("decode public key pem")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not ed25519")
	}
	copyBytes := make([]byte, len(publicKey))
	copy(copyBytes, publicKey)
	return copyBytes, nil
}

func normalizeScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		normalized := strings.TrimSpace(scope)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}
