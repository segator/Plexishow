//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// targetArgs returns any arguments passed after the target name in os.Args.
// Mage receives: mage [flags] <target> [args...]
func targetArgs() []string {
	for i, a := range os.Args {
		if strings.EqualFold(a, "run") {
			if i+1 < len(os.Args) {
				return os.Args[i+1:]
			}
			break
		}
	}
	return nil
}

var (
	binaryName = "plexishow"
	imageName  = "ghcr.io/segator/plexishow"
	version    = os.Getenv("VERSION")
)

func init() {
	if version == "" {
		v, _ := sh.Output("git", "describe", "--tags", "--always", "--dirty")
		if v == "" {
			version = "dev"
		} else {
			version = v
		}
	}
}

// Fmt runs go fmt
func Fmt() error {
	fmt.Println("Running fmt...")
	return sh.RunV("go", "fmt", "./...")
}

// Vet runs go vet
func Vet() error {
	fmt.Println("Running vet...")
	return sh.RunV("go", "vet", "./...")
}

// Test runs unit tests with race detector, coverage report, and threshold gate
func Test(ctx context.Context) error {
	mg.Deps(Vet)
	fmt.Println("Running tests with coverage...")
	if err := sh.RunV("go", "test", "-race", "-coverprofile=coverage.out", "-covermode=atomic", "./..."); err != nil {
		return err
	}

	coverageOutput, err := sh.Output("go", "tool", "cover", "-func=coverage.out")
	if err != nil {
		return err
	}

	threshold := 40.0
	lines := strings.Split(strings.TrimSpace(coverageOutput), "\n")
	if len(lines) == 0 {
		return fmt.Errorf("could not parse coverage output")
	}
	lastLine := lines[len(lines)-1]
	var coverage float64
	_, err = fmt.Sscanf(lastLine, "total: (statements) %f%%", &coverage)
	if err != nil {
		return fmt.Errorf("failed to parse coverage from line %q: %w", lastLine, err)
	}

	fmt.Printf("Coverage: %.1f%% (threshold: %.1f%%)\n", coverage, threshold)
	if coverage < threshold {
		return fmt.Errorf("coverage %.1f%% is below threshold %.1f%%", coverage, threshold)
	}
	fmt.Println("Coverage gate passed!")
	return nil
}

// Bin groups binary-related targets.
type Bin mg.Namespace

// Build compiles the binary into bin/plexishow.
func (Bin) Build(ctx context.Context) error {
	mg.Deps(Vet, Generate{}.Placeholder)
	fmt.Println("Building binary...")
	if err := os.MkdirAll("bin", 0755); err != nil {
		return err
	}
	ldflags := fmt.Sprintf("-ldflags=-s -w -X main.version=%s", version)
	return sh.RunV("go", "build", ldflags, "-o", "bin/"+binaryName, "./cmd/plexishow")
}

// Build compiles the binary and builds the Docker image.
func Build(ctx context.Context) {
	mg.Deps(Bin{}.Build, Docker{}.Build)
}

// Docker groups Docker image targets.
type Docker mg.Namespace

// Build builds the Docker image (depends on bin:build).
func (Docker) Build(ctx context.Context) error {
	mg.Deps(Bin{}.Build)
	fmt.Println("Building Docker image...")
	tag := fmt.Sprintf("%s:%s", imageName, version)
	return sh.RunV("docker", "build", "-t", tag, "-f", "Dockerfile", ".")
}

// Publish pushes the Docker image to the registry.
func (Docker) Publish(ctx context.Context) error {
	mg.Deps(Docker{}.Build)
	fmt.Println("Publishing Docker image...")
	tag := fmt.Sprintf("%s:%s", imageName, version)
	return sh.RunV("docker", "push", tag)
}

// Release runs GoReleaser (local, needs git tags)
func Release() error {
	mg.Deps(Vet)
	fmt.Println("Releasing...")
	return sh.RunV("goreleaser", "release", "--clean")
}

// ReleaseSnapshot runs GoReleaser in snapshot mode
func ReleaseSnapshot() error {
	fmt.Println("Releasing snapshot...")
	return sh.RunV("goreleaser", "release", "--snapshot", "--clean")
}

// Security runs govulncheck, generates SBOM, and scans with Grype.
// Writes reports to security-reports/ for GitHub Security upload.
func Security(ctx context.Context) error {
	if err := os.MkdirAll("security-reports", 0755); err != nil {
		return err
	}

	// 1. govulncheck
	fmt.Println("Running govulncheck...")
	out, err := sh.Output("govulncheck", "-format=sarif", "./...")
	if err != nil {
		return fmt.Errorf("govulncheck: %w", err)
	}
	if err := os.WriteFile("security-reports/govulncheck.sarif", []byte(out), 0644); err != nil {
		return err
	}

	// 2. SBOM (depends on binary)
	mg.Deps(Bin.Build)
	fmt.Println("Generating SBOM...")
	sbomOut, err := sh.Output("syft", "file:bin/"+binaryName, "-o", "spdx-json")
	if err != nil {
		return fmt.Errorf("syft: %w", err)
	}
	if err := os.WriteFile("sbom.json", []byte(sbomOut), 0644); err != nil {
		return err
	}

	// 3. Grype SARIF
	fmt.Println("Scanning SBOM with Grype (SARIF)...")
	sarif, err := sh.Output("grype", "sbom:sbom.json", "-o", "sarif")
	if err != nil {
		return fmt.Errorf("grype sarif: %w", err)
	}
	if err := os.WriteFile("security-reports/grype.sarif", []byte(sarif), 0644); err != nil {
		return err
	}

	// 4. Grype table (for logs) with critical gate
	fmt.Println("Scanning SBOM with Grype (table)...")
	return sh.RunV("grype", "sbom:sbom.json", "-o", "table", "--fail-on", "critical")
}

// Lint runs golangci-lint
func Lint(ctx context.Context) error {
	fmt.Println("Running golangci-lint...")
	return sh.RunV("golangci-lint", "run", "--timeout=5m", "./...")
}

// Clean removes build artifacts
func Clean() error {
	fmt.Println("Cleaning...")
	for _, p := range []string{"bin", "sbom.json", "coverage.out", "security-reports"} {
		if err := sh.Rm(p); err != nil {
			return err
		}
	}
	return nil
}

// Run builds and runs the application locally, forwarding any extra arguments.
// Use `--` to pass flags to the binary:
//
//	mage run
//	mage run -- -help
//	mage run -- -m3u-url https://example.com/playlist.m3u
func Run(ctx context.Context) error {
	mg.Deps(Bin.Build)
	bin := "./bin/" + binaryName
	extra := targetArgs()
	fmt.Println("Running plexishow...")
	if len(extra) == 0 {
		return sh.RunV(bin)
	}
	return sh.RunV(bin, extra...)
}

// All runs fmt, vet, test, bin:build
func All(ctx context.Context) {
	mg.Deps(Fmt, Vet)
	mg.Deps(Test)
	mg.Deps(Bin.Build)
}

// Generate groups asset generation targets.
type Generate mg.Namespace

// Placeholder generates the Full HD Stereo countdown video in assets/placeholder.ts.
// Fails gracefully if ffmpeg is not installed.
func (Generate) Placeholder(ctx context.Context) error {
	fmt.Println("Checking for ffmpeg to generate assets...")
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		fmt.Println("WARNING: ffmpeg not found in PATH. Skipping placeholder video generation. The application will run without the loading placeholder feature.")
		return nil
	}
	fmt.Println("Generating placeholder video...")
	if err := os.MkdirAll("assets", 0755); err != nil {
		return err
	}
	return sh.RunV("./scripts/generate_placeholder.sh")
}
