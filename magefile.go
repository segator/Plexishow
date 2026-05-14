//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
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

func goCacheVolumes(client *dagger.Client) (*dagger.CacheVolume, *dagger.CacheVolume) {
	return client.CacheVolume("go-mod-cache"), client.CacheVolume("go-build-cache")
}

// goSrc returns only Go-relevant files (excludes README, helm, etc.)
func goSrc(client *dagger.Client) *dagger.Directory {
	return client.Host().Directory(".", dagger.HostDirectoryOpts{
		Include: []string{
			"go.mod",
			"go.sum",
			"cmd/**",
			"internal/**",
			"test/**",
		},
	})
}

func goContainer(client *dagger.Client, src *dagger.Directory) *dagger.Container {
	goCache, buildCache := goCacheVolumes(client)
	return client.Container().
		From("golang:1.25").
		WithMountedDirectory("/src", src).
		WithMountedCache("/go/pkg/mod", goCache).
		WithMountedCache("/root/.cache/go-build", buildCache).
		WithWorkdir("/src")
}

// Fmt runs go fmt (local, fast)
func Fmt() error {
	fmt.Println("Running fmt...")
	return sh.RunV("go", "fmt", "./...")
}

// Vet runs go vet (local, fast)
func Vet() error {
	fmt.Println("Running vet...")
	return sh.RunV("go", "vet", "./...")
}

// Test runs unit tests inside a Dagger container
func Test(ctx context.Context) error {
	mg.Deps(Vet)
	fmt.Println("Running tests in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	src := goSrc(client)
	golang := goContainer(client, src).
		WithExec([]string{"go", "mod", "download"}).
		WithExec([]string{"go", "test", "-v", "./..."})

	_, err = golang.Stdout(ctx)
	return err
}

// Build compiles the binary inside a Dagger container and exports it
func Build(ctx context.Context) error {
	mg.Deps(Vet)
	fmt.Println("Building in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	src := goSrc(client)
	ldflags := fmt.Sprintf("-ldflags=-s -w -X main.version=%s", version)

	golang := goContainer(client, src).
		WithEnvVariable("CGO_ENABLED", "0").
		WithExec([]string{"go", "mod", "download"}).
		WithExec([]string{"go", "build", ldflags, "-o", "bin/plexishow", "./cmd/plexishow"})

	_, err = golang.File("/src/bin/plexishow").Export(ctx, filepath.Join("bin", binaryName))
	return err
}

// Docker builds the Docker image inside Dagger and publishes it
func Docker(ctx context.Context) error {
	mg.Deps(Build)
	fmt.Println("Building Docker image in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	src := client.Host().Directory(".")
	cacheTag := "ghcr.io/segator/plexishow:buildcache"

	image := src.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Dockerfile: "Dockerfile",
	})

	addr, err := image.Publish(ctx, fmt.Sprintf("%s:%s", imageName, version))
	if err != nil {
		return err
	}
	fmt.Println("Published:", addr)

	_, err = image.Publish(ctx, cacheTag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to push cache: %v\n", err)
	}
	return nil
}

// DockerGPU builds the GPU Docker image inside Dagger
func DockerGPU(ctx context.Context) error {
	mg.Deps(Build)
	fmt.Println("Building GPU Docker image in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	src := client.Host().Directory(".")
	cacheTag := "ghcr.io/segator/plexishow:buildcache-gpu"

	image := src.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Dockerfile: "Dockerfile.gpu",
	})

	addr, err := image.Publish(ctx, fmt.Sprintf("%s:%s-gpu", imageName, version))
	if err != nil {
		return err
	}
	fmt.Println("Published:", addr)

	_, err = image.Publish(ctx, cacheTag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to push cache: %v\n", err)
	}
	return nil
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

// Sbom generates SBOM using Syft inside Dagger
func Sbom(ctx context.Context) error {
	mg.Deps(Build)
	fmt.Println("Generating SBOM in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	bin := client.Host().Directory("bin")

	syft := client.Container().
		From("golang:1.25").
		WithExec([]string{"sh", "-c",
			"curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin"},
		).
		WithMountedDirectory("/src", bin).
		WithWorkdir("/src").
		WithExec([]string{"syft", "file:plexishow", "-o", "spdx-json"})

	out, err := syft.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("syft: %w", err)
	}
	return os.WriteFile("sbom.json", []byte(out), 0644)
}

// VulnScan scans the SBOM for vulnerabilities using Grype inside Dagger
func VulnScan(ctx context.Context) error {
	mg.Deps(Sbom)
	fmt.Println("Scanning for vulnerabilities in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	sbom := client.Host().File("sbom.json")

	grype := client.Container().
		From("golang:1.25").
		WithExec([]string{"sh", "-c",
			"curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin"},
		).
		WithMountedFile("/sbom.json", sbom).
		WithWorkdir("/").
		WithExec([]string{"grype", "sbom:/sbom.json", "-o", "table", "--fail-on", "critical"})

	out, err := grype.Stdout(ctx)
	if err != nil {
		fmt.Println(out)
		return err
	}
	fmt.Println(out)
	return nil
}

// Cover runs tests with coverage report inside Dagger and enforces a minimum threshold
func Cover(ctx context.Context) error {
	fmt.Println("Running coverage in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	src := goSrc(client)
	golang := goContainer(client, src).
		WithExec([]string{"mkdir", "-p", "/output"}).
		WithExec([]string{"go", "mod", "download"}).
		WithExec([]string{"go", "test", "-race", "-coverprofile=/output/coverage.out", "-covermode=atomic", "./..."}).
		WithExec([]string{"go", "tool", "cover", "-func=/output/coverage.out", "-o", "/output/coverage.txt"})

	_, err = golang.File("/output/coverage.out").Export(ctx, "coverage.out")
	if err != nil {
		return err
	}

	threshold := 40.0
	coverageOutput, err := golang.File("/output/coverage.txt").Contents(ctx)
	if err != nil {
		return err
	}

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

// Lint runs golangci-lint inside a Dagger container
func Lint(ctx context.Context) error {
	fmt.Println("Running golangci-lint in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	src := goSrc(client)
	linter := goContainer(client, src).
		WithExec([]string{"go", "mod", "download"}).
		WithExec([]string{"go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"}).
		WithExec([]string{"golangci-lint", "run", "--timeout=5m", "./..."})

	out, err := linter.Stdout(ctx)
	if err != nil {
		fmt.Println(out)
		return err
	}
	fmt.Println(out)
	fmt.Println("Lint passed!")
	return nil
}

// All runs fmt, vet, test, build
func All(ctx context.Context) {
	mg.Deps(Fmt, Vet)
	mg.Deps(Test)
	mg.Deps(Build)
}
