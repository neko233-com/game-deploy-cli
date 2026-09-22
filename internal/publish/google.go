package publish

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const googlePublisherScope = "https://www.googleapis.com/auth/androidpublisher"

type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type googleAPI struct {
	client    *http.Client
	token     string
	baseURL   string
	userAgent string
}

func publishGooglePlay(ctx context.Context, opts Options) (Result, error) {
	result := Result{
		Provider: opts.Provider,
		Artifact: opts.Artifact,
		Track:    opts.Track,
	}
	if opts.Track == "" {
		result.Track = "internal"
	}
	if err := validateArtifact(opts.Artifact, ".aab", ".apk"); err != nil {
		return result, err
	}
	if opts.PackageName == "" {
		return result, errors.New("--package is required for google-play")
	}
	if !validTrack(result.Track) {
		return result, fmt.Errorf("unsupported Google Play track %q", result.Track)
	}
	if opts.ReleaseStatus == "" {
		opts.ReleaseStatus = "completed"
	}
	if !validReleaseStatus(opts.ReleaseStatus) {
		return result, fmt.Errorf("unsupported release status %q", opts.ReleaseStatus)
	}

	serviceAccountPath := configured(opts.GoogleServiceAccount, "GOOGLE_PLAY_SERVICE_ACCOUNT_FILE")
	if serviceAccountPath == "" {
		result.Warnings = append(result.Warnings, "GOOGLE_PLAY_SERVICE_ACCOUNT_FILE is not configured")
	}
	if opts.DryRun {
		result.Status = "dry-run"
		result.Message = fmt.Sprintf("would upload %s to Google Play %s track (%s)", filepath.Base(opts.Artifact), result.Track, providerPlatform())
		return result, nil
	}
	if !opts.Yes {
		return result, errors.New("publishing is a write operation; rerun with --yes or use --dry-run")
	}
	if serviceAccountPath == "" {
		return result, errors.New("Google Play credentials are missing; set --service-account or GOOGLE_PLAY_SERVICE_ACCOUNT_FILE")
	}
	if opts.ReleaseStatus != "draft" && opts.Track == "production" {
		result.Warnings = append(result.Warnings, "production track will be submitted with status "+opts.ReleaseStatus)
	}

	account, err := readServiceAccount(serviceAccountPath)
	if err != nil {
		return result, err
	}
	token, err := serviceAccountToken(ctx, opts.HTTPClient, account)
	if err != nil {
		return result, err
	}
	api := googleAPI{client: defaultHTTPClient(opts.HTTPClient), token: token, baseURL: "https://androidpublisher.googleapis.com/androidpublisher/v3", userAgent: "game-deploy-cli"}
	editID, err := api.createEdit(ctx, opts.PackageName)
	if err != nil {
		return result, err
	}
	versionCode, err := api.uploadArtifact(ctx, opts.PackageName, editID, opts.Artifact)
	if err != nil {
		return result, err
	}
	if err := api.updateTrack(ctx, opts.PackageName, editID, result.Track, opts.ReleaseStatus, versionCode); err != nil {
		return result, err
	}
	if err := api.commitEdit(ctx, opts.PackageName, editID); err != nil {
		return result, err
	}
	result.Status = "published"
	result.VersionCode = versionCode
	result.Message = "Google Play edit committed"
	return result, nil
}

func validTrack(track string) bool {
	switch track {
	case "internal", "alpha", "beta", "production":
		return true
	default:
		return false
	}
}

func validReleaseStatus(status string) bool {
	switch status {
	case "completed", "draft", "inProgress", "halted":
		return true
	default:
		return false
	}
}

func readServiceAccount(path string) (serviceAccount, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return serviceAccount{}, fmt.Errorf("read Google service account %q: %w", path, err)
	}
	var account serviceAccount
	if err := json.Unmarshal(data, &account); err != nil {
		return account, fmt.Errorf("parse Google service account %q: %w", path, err)
	}
	if account.ClientEmail == "" || account.PrivateKey == "" {
		return account, errors.New("Google service account must contain client_email and private_key")
	}
	if account.TokenURI == "" {
		account.TokenURI = "https://oauth2.googleapis.com/token"
	}
	return account, nil
}

func serviceAccountToken(ctx context.Context, client *http.Client, account serviceAccount) (string, error) {
	key, err := parsePrivateKey(account.PrivateKey)
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	header := encodeJWTPart(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims := encodeJWTPart(map[string]any{
		"iss":   account.ClientEmail,
		"scope": googlePublisherScope,
		"aud":   account.TokenURI,
		"iat":   now,
		"exp":   now + 3600,
	})
	unsigned := header + "." + claims
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign Google service account JWT: %w", err)
	}
	assertion := unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, account.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request Google OAuth token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("Google OAuth token returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &token); err != nil || token.AccessToken == "" {
		return "", errors.New("Google OAuth response did not contain access_token")
	}
	return token.AccessToken, nil
}

func parsePrivateKey(value string) (*rsa.PrivateKey, error) {
	block, _ := pemDecode([]byte(value))
	if block == nil {
		return nil, errors.New("Google private_key is not valid PEM")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(block); err == nil {
		return key, nil
	}
	return nil, errors.New("Google private_key is not a supported RSA private key")
}

func encodeJWTPart(value any) string {
	data, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(data)
}

func defaultHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: 10 * time.Minute}
}

// pemDecode is a small indirection to make private-key parsing easy to test
// without making the rest of the publisher depend on a custom PEM type.
func pemDecode(data []byte) ([]byte, []byte) {
	block, rest := pem.Decode(data)
	if block == nil {
		return nil, rest
	}
	return block.Bytes, rest
}

func (api googleAPI) request(ctx context.Context, method, endpoint string, body io.Reader, contentType string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, api.baseURL+endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+api.token)
	req.Header.Set("User-Agent", api.userAgent)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Google Play API %s returned %s: %s", endpoint, resp.Status, strings.TrimSpace(string(data)))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode Google Play response: %w", err)
		}
	}
	return nil
}

func (api googleAPI) createEdit(ctx context.Context, packageName string) (string, error) {
	var response struct {
		ID string `json:"id"`
	}
	endpoint := "/applications/" + url.PathEscape(packageName) + "/edits"
	if err := api.request(ctx, http.MethodPost, endpoint, bytes.NewReader([]byte("{}")), "application/json", &response); err != nil {
		return "", err
	}
	if response.ID == "" {
		return "", errors.New("Google Play create edit response did not contain id")
	}
	return response.ID, nil
}

func (api googleAPI) uploadArtifact(ctx context.Context, packageName, editID, artifact string) (string, error) {
	data, err := os.Open(artifact)
	if err != nil {
		return "", fmt.Errorf("open artifact: %w", err)
	}
	defer data.Close()
	ext := strings.ToLower(filepath.Ext(artifact))
	resource := "bundles"
	if ext == ".apk" {
		resource = "apks"
	}
	endpoint := "/applications/" + url.PathEscape(packageName) + "/edits/" + url.PathEscape(editID) + "/" + resource + "?uploadType=media"
	var response struct {
		VersionCode int64 `json:"versionCode"`
	}
	if err := api.request(ctx, http.MethodPost, endpoint, data, "application/octet-stream", &response); err != nil {
		return "", err
	}
	if response.VersionCode == 0 {
		return "", errors.New("Google Play upload response did not contain versionCode")
	}
	return fmt.Sprintf("%d", response.VersionCode), nil
}

func (api googleAPI) updateTrack(ctx context.Context, packageName, editID, track, status, versionCode string) error {
	body, _ := json.Marshal(map[string]any{
		"releases": []map[string]any{{
			"versionCodes": []string{versionCode},
			"status":       status,
		}},
	})
	endpoint := "/applications/" + url.PathEscape(packageName) + "/edits/" + url.PathEscape(editID) + "/tracks/" + url.PathEscape(track)
	return api.request(ctx, http.MethodPut, endpoint, bytes.NewReader(body), "application/json", nil)
}

func (api googleAPI) commitEdit(ctx context.Context, packageName, editID string) error {
	endpoint := "/applications/" + url.PathEscape(packageName) + "/edits/" + url.PathEscape(editID) + ":commit"
	return api.request(ctx, http.MethodPost, endpoint, bytes.NewReader([]byte("{}")), "application/json", nil)
}
