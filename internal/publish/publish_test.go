package publish

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDryRunGooglePlayDoesNotNeedCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.aab")
	if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), Options{
		Provider:    "android-google",
		Artifact:    path,
		PackageName: "com.example.game",
		Track:       "internal",
		DryRun:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "dry-run" || result.Provider != "google-play" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAppStoreConnectDryRunWarnsOnNonMacOS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.ipa")
	if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), Options{
		Provider:      "ios",
		Artifact:      path,
		BundleID:      "com.example.game",
		DryRun:        true,
		AppleKeyID:    "key",
		AppleIssuerID: "issuer",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "dry-run" || len(result.Command) == 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
