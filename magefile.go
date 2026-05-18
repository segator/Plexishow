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
	// Force static compilation (disable CGO) so that the binary is fully compatible
	// with Alpine's musl standard library and does not dynamically link to host's glibc.
	os.Setenv("CGO_ENABLED", "0")
	ldflags := fmt.Sprintf("-ldflags=-s -w -X main.version=%s", version)
	return sh.RunV("go", "build", ldflags, "-o", "bin/"+binaryName, "./cmd/plexishow")
}

// Build compiles the binary, builds the Docker image, and packages the Helm chart.
func Build(ctx context.Context) {
	mg.Deps(Bin{}.Build, Docker{}.Build, Helm{}.Build)
}

// Publish builds and publishes both the multi-arch Docker image and the Helm chart.
func Publish(ctx context.Context) {
	mg.Deps(Docker{}.Push, Helm{}.Publish)
}

// Docker groups Docker image targets.
type Docker mg.Namespace

// Build builds the Docker image (depends on bin:build).
func (Docker) Build(ctx context.Context) error {
	mg.Deps(Bin{}.Build)
	fmt.Println("Building Docker image...")
	tag := fmt.Sprintf("%s:%s", imageName, version)
	return sh.RunV("docker", "buildx", "build", "--load", "-t", tag, "-f", "Dockerfile", ".")
}

// Publish pushes the Docker image to the registry (image must already exist locally).
func (Docker) Publish(ctx context.Context) error {
	fmt.Println("Publishing Docker image...")
	tag := fmt.Sprintf("%s:%s", imageName, version)
	return sh.RunV("docker", "push", tag)
}

// Push builds and pushes a multi-arch Docker image directly.
// Relies on buildkit cache from a prior docker:build for speed.
func (Docker) Push(ctx context.Context) error {
	fmt.Println("Pushing multi-arch Docker image...")
	platforms := os.Getenv("PLATFORMS")
	if platforms == "" {
		platforms = "linux/amd64,linux/arm64"
	}
	tag := fmt.Sprintf("%s:%s", imageName, version)
	return sh.RunV("docker", "buildx", "build", "--platform", platforms, "--push", "-t", tag, "-f", "Dockerfile", ".")
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

// Helm groups Helm-related targets.
type Helm mg.Namespace

// Build packages the Helm chart into a .tgz file.
// Use the VERSION env var to override the chart and app version.
func (Helm) Build(ctx context.Context) error {
	fmt.Println("Packaging Helm chart...")
	
	// Chart version must comply strictly with SemVer (e.g. 0.0.0-dev or 1.0.0)
	chartVersion := version
	if strings.HasPrefix(chartVersion, "v") {
		chartVersion = chartVersion[1:] // Strip leading 'v'
	}
	if !strings.Contains(chartVersion, ".") {
		chartVersion = "0.0.0-" + chartVersion
	}
	
	// App version is free-form and matches the Docker image tag standard (e.g. dev or 1.0.0)
	appVersion := version
	if strings.HasPrefix(appVersion, "v") {
		appVersion = appVersion[1:] // Strip leading 'v' to match standard Docker release tags
	}

	return sh.RunV("helm", "package", "helm/plexishow", "--version", chartVersion, "--app-version", appVersion)
}

// Publish packages and pushes the Helm chart to GHCR as an OCI registry package.
// Relies on a prior helm registry login.
func (Helm) Publish(ctx context.Context) error {
	mg.Deps(Helm{}.Build)
	fmt.Println("Publishing Helm chart...")
	
	chartVersion := version
	if strings.HasPrefix(chartVersion, "v") {
		chartVersion = chartVersion[1:] // Strip leading 'v'
	}
	if !strings.Contains(chartVersion, ".") {
		chartVersion = "0.0.0-" + chartVersion
	}
	
	chartTar := fmt.Sprintf("plexishow-%s.tgz", chartVersion)
	defer os.Remove(chartTar) // Clean up the tgz file after pushing

	owner := os.Getenv("GITHUB_REPOSITORY_OWNER")
	if owner == "" {
		owner = "segator" // Fallback to original owner
	}

	ociURL := fmt.Sprintf("oci://ghcr.io/%s", strings.ToLower(owner))
	return sh.RunV("helm", "push", chartTar, ociURL)
}

