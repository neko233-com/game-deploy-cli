package publish

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Options is the stable provider-neutral input used by the CLI and Agent
// integrations. Credentials are paths or environment-derived values; secrets
// are intentionally never accepted as command-line arguments.
type Options struct {
	Provider             string
	Artifact             string
	PackageName          string
	Track                string
	ReleaseStatus        string
	BundleID             string
	GoogleServiceAccount string
	AppleAPIKey          string
	AppleKeyID           string
	AppleIssuerID        string
	Yes                  bool
	DryRun               bool
	JSON                 bool
	HTTPClient           *http.Client
	CommandRunner        CommandRunner
	Output               io.Writer
}

type Result struct {
	Provider    string   `json:"provider"`
	Status      string   `json:"status"`
	Artifact    string   `json:"artifact"`
	VersionCode string   `json:"version_code,omitempty"`
	Track       string   `json:"track,omitempty"`
	BundleID    string   `json:"bundle_id,omitempty"`
	Message     string   `json:"message,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	Command     []string `json:"command,omitempty"`
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Run(ctx context.Context, opts Options) (Result, error) {
	if opts.Output == nil {
		opts.Output = io.Discard
	}
	if opts.CommandRunner == nil {
		opts.CommandRunner = osCommandRunner{}
	}
	opts.Provider = normalizeProvider(opts.Provider)
	switch opts.Provider {
	case "google-play":
		return publishGooglePlay(ctx, opts)
	case "app-store-connect":
		return publishAppStoreConnect(ctx, opts)
	default:
		return Result{}, fmt.Errorf("unsupported provider %q; use google-play or app-store-connect", opts.Provider)
	}
}

func normalizeProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "android", "android-google", "google", "google-play", "play":
		return "google-play"
	case "apple", "app-store-connect", "appstoreconnect", "ios":
		return "app-store-connect"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func validateArtifact(path string, extensions ...string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("artifact is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("artifact %q: %w", path, err)
	}
	if info.IsDir() || info.Size() == 0 {
		return fmt.Errorf("artifact %q must be a non-empty file", path)
	}
	ext := strings.ToLower(filepath.Ext(path))
	for _, allowed := range extensions {
		if ext == allowed {
			return nil
		}
	}
	return fmt.Errorf("artifact %q has unsupported extension %q; expected %s", path, ext, strings.Join(extensions, ", "))
}

func configured(value, envName string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(os.Getenv(envName))
}

func providerPlatform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}
