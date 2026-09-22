package publish

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func publishAppStoreConnect(ctx context.Context, opts Options) (Result, error) {
	result := Result{Provider: opts.Provider, Artifact: opts.Artifact, BundleID: opts.BundleID}
	if err := validateArtifact(opts.Artifact, ".ipa"); err != nil {
		return result, err
	}
	if opts.BundleID == "" {
		return result, errors.New("--bundle-id is required for app-store-connect")
	}
	keyPath := configured(opts.AppleAPIKey, "ASC_API_KEY_FILE")
	keyID := configured(opts.AppleKeyID, "ASC_KEY_ID")
	issuerID := configured(opts.AppleIssuerID, "ASC_ISSUER_ID")
	if keyPath == "" || keyID == "" || issuerID == "" {
		result.Warnings = append(result.Warnings, "App Store Connect requires ASC_API_KEY_FILE, ASC_KEY_ID and ASC_ISSUER_ID")
	}
	if opts.DryRun {
		result.Status = "dry-run"
		result.Message = fmt.Sprintf("would upload %s to App Store Connect (%s)", filepath.Base(opts.Artifact), providerPlatform())
		if runtime.GOOS != "darwin" {
			result.Warnings = append(result.Warnings, "IPA upload requires macOS Xcode Transporter; this host can only produce a plan")
		}
		if keyID != "" && issuerID != "" {
			result.Command = transporterCommand(opts.Artifact, keyID, issuerID)
		}
		return result, nil
	}
	if !opts.Yes {
		return result, errors.New("publishing is a write operation; rerun with --yes or use --dry-run")
	}
	if runtime.GOOS != "darwin" {
		return result, errors.New("App Store Connect IPA upload must run on macOS with Xcode Transporter")
	}
	if keyPath == "" || keyID == "" || issuerID == "" {
		return result, errors.New("App Store Connect credentials are missing; set --api-key/--key-id/--issuer-id or ASC_* environment variables")
	}
	if _, err := os.Stat(keyPath); err != nil {
		return result, fmt.Errorf("App Store Connect API key %q: %w", keyPath, err)
	}
	command := transporterCommand(opts.Artifact, keyID, issuerID)
	if err := opts.CommandRunner.Run(ctx, command[0], command[1:]...); err != nil {
		return result, fmt.Errorf("Transporter upload failed: %w", err)
	}
	result.Status = "published"
	result.Message = "App Store Connect Transporter upload completed"
	result.Command = command
	return result, nil
}

func transporterCommand(artifact, keyID, issuerID string) []string {
	return []string{"xcrun", "iTMSTransporter", "-m", "upload", "-assetFile", artifact, "-apiKey", keyID, "-apiIssuer", issuerID}
}
