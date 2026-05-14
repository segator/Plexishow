//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

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

// Build compiles the binary
func Build(ctx context.Context) error {
	mg.Deps(Vet)
	fmt.Println("Building...")
	if err := os.MkdirAll("bin", 0755); err != nil {
		return err
	}
	ldflags := fmt.Sprintf("-ldflags=-s -w -X main.version=%s", version)
	return sh.RunV("go", "build", ldflags, "-o", "bin/"+binaryName, "./cmd/plexishow")
}

// Docker builds and pushes the Docker image
func Docker(ctx context.Context) error {
	mg.Deps(Build)
	fmt.Println("Building Docker image...")
	tag := fmt.Sprintf("%s:%s", imageName, version)
	if err := sh.RunV("docker", "build", "-t", tag, "-f", "Dockerfile", "."); err != nil {
		return err
	}
	fmt.Println("Pushing:", tag)
	return sh.RunV("docker", "push", tag)
}

// DockerGPU builds and pushes the GPU Docker image
func DockerGPU(ctx context.Context) error {
	mg.Deps(Build)
	fmt.Println("Building GPU Docker image...")
	tag := fmt.Sprintf("%s:%s-gpu", imageName, version)
	if err := sh.RunV("docker", "build", "-t", tag, "-f", "Dockerfile.gpu", "."); err != nil {
		return err
	}
	fmt.Println("Pushing:", tag)
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

// Sbom generates SBOM using Syft
func Sbom(ctx context.Context) error {
	mg.Deps(Build)
	fmt.Println("Generating SBOM...")
	out, err := sh.Output("syft", "file:bin/"+binaryName, "-o", "spdx-json")
	if err != nil {
		return fmt.Errorf("syft: %w", err)
	}
	return os.WriteFile("sbom.json", []byte(out), 0644)
}

// VulnScan scans the SBOM for vulnerabilities using Grype.
// Generates vulns.sarif for GitHub Security and prints a table summary.
func VulnScan(ctx context.Context) error {
	mg.Deps(Sbom)
	fmt.Println("Generating SARIF vulnerability report...")
	sarif, err := sh.Output("grype", "sbom:sbom.json", "-o", "sarif")
	if err != nil {
		return fmt.Errorf("grype sarif: %w", err)
	}
	if err := os.WriteFile("vulns.sarif", []byte(sarif), 0644); err != nil {
		return err
	}

	fmt.Println("Scanning for vulnerabilities...")
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
	for _, p := range []string{"bin", "sbom.json", "coverage.out", "vulns.sarif"} {
		if err := sh.Rm(p); err != nil {
			return err
		}
	}
	return nil
}

// All runs fmt, vet, test, build
func All(ctx context.Context) {
	mg.Deps(Fmt, Vet)
	mg.Deps(Test)
	mg.Deps(Build)
}
