package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/neko233-com/game-deploy-cli/internal/build"
	"github.com/neko233-com/game-deploy-cli/internal/publish"
)

type Runner struct {
	Out    io.Writer
	ErrOut io.Writer
}

func (r Runner) Run(args []string) int {
	if r.Out == nil {
		r.Out = os.Stdout
	}
	if r.ErrOut == nil {
		r.ErrOut = os.Stderr
	}
	jsonOutput, args, err := parseGlobal(args)
	if err != nil {
		return r.fail(jsonOutput, err)
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		r.printUsage()
		return 0
	}
	switch args[0] {
	case "version":
		return r.writeValue(jsonOutput, map[string]string{
			"name":    "game-deploy",
			"version": build.Version,
			"go":      runtime.Version(),
		})
	case "doctor":
		return r.doctor(jsonOutput)
	case "auth":
		return r.auth(args[1:], jsonOutput)
	case "publish":
		return r.publish(args[1:], jsonOutput)
	default:
		return r.fail(jsonOutput, fmt.Errorf("unknown command %q", args[0]))
	}
}

func parseGlobal(args []string) (bool, []string, error) {
	jsonOutput := false
	for len(args) > 0 {
		switch args[0] {
		case "--json":
			jsonOutput = true
			args = args[1:]
		case "--version":
			return jsonOutput, []string{"version"}, nil
		default:
			return jsonOutput, args, nil
		}
	}
	return jsonOutput, args, nil
}

func (r Runner) doctor(jsonOutput bool) int {
	checks := map[string]any{
		"os":                      runtime.GOOS,
		"arch":                    runtime.GOARCH,
		"version":                 build.Version,
		"google_play_credentials": firstNonEmpty(os.Getenv("GOOGLE_PLAY_SERVICE_ACCOUNT_FILE"), "not configured"),
		"app_store_connect_key":   firstNonEmpty(os.Getenv("ASC_API_KEY_FILE"), "not configured"),
	}
	if _, err := exec.LookPath("xcrun"); err != nil {
		checks["xcrun"] = "missing (required for real iOS upload)"
	} else {
		checks["xcrun"] = "available"
	}
	return r.writeValue(jsonOutput, checks)
}

func (r Runner) auth(args []string, jsonOutput bool) int {
	if len(args) != 1 || args[0] != "status" {
		return r.fail(jsonOutput, errors.New("usage: game-deploy auth status"))
	}
	value := map[string]any{
		"google_play": map[string]any{
			"configured": os.Getenv("GOOGLE_PLAY_SERVICE_ACCOUNT_FILE") != "",
			"source":     "GOOGLE_PLAY_SERVICE_ACCOUNT_FILE",
		},
		"app_store_connect": map[string]any{
			"configured": os.Getenv("ASC_API_KEY_FILE") != "" && os.Getenv("ASC_KEY_ID") != "" && os.Getenv("ASC_ISSUER_ID") != "",
			"source":     []string{"ASC_API_KEY_FILE", "ASC_KEY_ID", "ASC_ISSUER_ID"},
		},
	}
	return r.writeValue(jsonOutput, value)
}

func (r Runner) publish(args []string, jsonOutput bool) int {
	if len(args) == 0 {
		return r.fail(jsonOutput, errors.New("usage: game-deploy publish <google-play|app-store-connect> [flags]"))
	}
	provider := args[0]
	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	artifact := fs.String("artifact", "", "path to .aab/.apk or .ipa")
	packageName := fs.String("package", "", "Android applicationId/package name")
	track := fs.String("track", "internal", "Google Play track: internal, alpha, beta, production")
	releaseStatus := fs.String("release-status", "completed", "Google Play release status")
	bundleID := fs.String("bundle-id", "", "iOS bundle identifier")
	serviceAccount := fs.String("service-account", "", "Google service account JSON file")
	apiKey := fs.String("api-key", "", "App Store Connect .p8 key file")
	keyID := fs.String("key-id", "", "App Store Connect API key ID")
	issuerID := fs.String("issuer-id", "", "App Store Connect issuer ID")
	yes := fs.Bool("yes", false, "confirm the external write")
	dryRun := fs.Bool("dry-run", false, "validate and print the plan without uploading")
	if err := fs.Parse(args[1:]); err != nil {
		return r.fail(jsonOutput, err)
	}
	if fs.NArg() != 0 {
		return r.fail(jsonOutput, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " ")))
	}
	result, err := publish.Run(context.Background(), publish.Options{
		Provider:             provider,
		Artifact:             *artifact,
		PackageName:          *packageName,
		Track:                *track,
		ReleaseStatus:        *releaseStatus,
		BundleID:             *bundleID,
		GoogleServiceAccount: *serviceAccount,
		AppleAPIKey:          *apiKey,
		AppleKeyID:           *keyID,
		AppleIssuerID:        *issuerID,
		Yes:                  *yes,
		DryRun:               *dryRun,
		JSON:                 jsonOutput,
		Output:               r.Out,
	})
	if err != nil {
		return r.fail(jsonOutput, err)
	}
	return r.writeValue(jsonOutput, result)
}

func (r Runner) writeValue(jsonOutput bool, value any) int {
	if jsonOutput {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return r.fail(true, err)
		}
		fmt.Fprintln(r.Out, string(data))
		return 0
	}
	switch typed := value.(type) {
	case map[string]string:
		for key, item := range typed {
			fmt.Fprintf(r.Out, "%s: %s\n", key, item)
		}
	case publish.Result:
		fmt.Fprintf(r.Out, "%s: %s\n", typed.Provider, typed.Status)
		if typed.Message != "" {
			fmt.Fprintln(r.Out, typed.Message)
		}
		for _, warning := range typed.Warnings {
			fmt.Fprintf(r.Out, "warning: %s\n", warning)
		}
		if len(typed.Command) > 0 {
			fmt.Fprintf(r.Out, "command: %s\n", strings.Join(typed.Command, " "))
		}
	default:
		data, _ := json.MarshalIndent(value, "", "  ")
		fmt.Fprintln(r.Out, string(data))
	}
	return 0
}

func (r Runner) fail(jsonOutput bool, err error) int {
	if jsonOutput {
		_ = r.writeValue(true, map[string]string{"error": err.Error()})
	} else {
		fmt.Fprintf(r.ErrOut, "error: %s\n", err)
	}
	return 1
}

func (r Runner) printUsage() {
	fmt.Fprintln(r.Out, `game-deploy - Agent-friendly game release CLI

Usage:
  game-deploy [--json] version
  game-deploy [--json] doctor
  game-deploy [--json] auth status
  game-deploy [--json] publish google-play --artifact game.aab --package com.example.game [--dry-run|--yes]
  game-deploy [--json] publish app-store-connect --artifact game.ipa --bundle-id com.example.game [--dry-run|--yes]

Publishing is a write operation. Use --dry-run to inspect a plan; use --yes to upload.
Credentials are read from files and environment variables, never printed.`)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
