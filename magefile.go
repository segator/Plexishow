//go:build mage
// +build mage

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	// Enable Dagger registry cache for ALL Dagger operations.
	if os.Getenv("_EXPERIMENTAL_DAGGER_CACHE_CONFIG") == "" {
		os.Setenv("_EXPERIMENTAL_DAGGER_CACHE_CONFIG",
			fmt.Sprintf("type=registry,ref=%s:dagger-cache,mode=max", imageName))
	}
	// Pass GHCR creds to Dagger engine so BuildKit can push/pull the cache.
	// Precedence: DAGGER_REGISTRY_AUTH_ghcr.io env > GHCR_USER+GHCR_TOKEN > docker config.json.
	if os.Getenv("DAGGER_REGISTRY_AUTH_ghcr.io") == "" {
		user, token := os.Getenv("GHCR_USER"), os.Getenv("GHCR_TOKEN")
		if user == "" || token == "" {
			user, token = readDockerConfig("ghcr.io")
		}
		if user != "" && token != "" {
			authJSON := fmt.Sprintf(`{"username":"%s","password":"%s"}`, user, token)
			os.Setenv("DAGGER_REGISTRY_AUTH_ghcr.io", authJSON)
		}
	}
}

// readDockerConfig reads the auth entry for a registry from ~/.docker/config.json.
// Returns username and password if found.
func readDockerConfig(host string) (string, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	cfgPath := filepath.Join(home, ".docker", "config.json")
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", ""
	}
	var cfg struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return "", ""
	}
	entry, ok := cfg.Auths[host]
	if !ok || entry.Auth == "" {
		return "", ""
	}
	decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
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
			".golangci.yml",
		},
	})
}

// miseContainer returns a container with all tools from mise.toml installed.
// This is the single source of truth for all tooling inside Dagger.
func miseContainer(client *dagger.Client) *dagger.Container {
	miseToml := client.Host().File("mise.toml")
	miseCache := client.CacheVolume("mise-cache")
	goCache, buildCache := goCacheVolumes(client)

	return client.Container().
		From("alpine:3.19").
		WithExec([]string{"apk", "add", "--no-cache", "bash", "curl", "git", "gcc", "musl-dev", "libc-dev"}).
		WithExec([]string{"sh", "-c", "curl -sSfL https://mise.run | sh"}).
		WithEnvVariable("PATH", "/root/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		WithMountedFile("/mise.toml", miseToml).
		WithMountedCache("/root/.local/share/mise", miseCache).
		WithMountedCache("/go/pkg/mod", goCache).
		WithMountedCache("/root/.cache/go-build", buildCache).
		WithEnvVariable("MISE_TRUSTED_CONFIG_PATHS", "/mise.toml").
		WithExec([]string{"mise", "install", "-y"})
}

// miseExec wraps a command with "mise exec --" so mise-managed tools are in PATH
func miseExec(cmd ...string) []string {
	return append([]string{"mise", "exec", "--"}, cmd...)
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
	_, err = miseContainer(client).
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
		WithExec(miseExec("go", "mod", "download")).
		WithExec(miseExec("go", "test", "-v", "./...")).
		Stdout(ctx)
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

	golang := miseContainer(client).
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
		WithEnvVariable("CGO_ENABLED", "0").
		WithExec(miseExec("go", "mod", "download")).
		WithExec(miseExec("go", "build", ldflags, "-o", "bin/plexishow", "./cmd/plexishow"))

	_, err = golang.File("/src/bin/plexishow").Export(ctx, filepath.Join("bin", binaryName))
	return err
}

// Docker builds the Docker image inside Dagger.
func Docker(ctx context.Context) error {
	mg.Deps(Build)
	fmt.Println("Building Docker image in Dagger...")

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	src := client.Host().Directory(".")
	image := src.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Dockerfile: "Dockerfile",
	})

	addr, err := image.Publish(ctx, fmt.Sprintf("%s:%s", imageName, version))
	if err != nil {
		return err
	}
	fmt.Println("Published:", addr)
	return nil
}

// DockerGPU builds the GPU Docker image inside Dagger.
func DockerGPU(ctx context.Context) error {
	mg.Deps(Build)
	fmt.Println("Building GPU Docker image in Dagger...")

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	src := client.Host().Directory(".")
	image := src.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Dockerfile: "Dockerfile.gpu",
	})

	addr, err := image.Publish(ctx, fmt.Sprintf("%s:%s-gpu", imageName, version))
	if err != nil {
		return err
	}
	fmt.Println("Published:", addr)
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

// Sbom generates SBOM using Syft inside Dagger (via mise)
func Sbom(ctx context.Context) error {
	mg.Deps(Build)
	fmt.Println("Generating SBOM in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	bin := client.Host().Directory("bin")

	out, err := miseContainer(client).
		WithMountedDirectory("/src", bin).
		WithWorkdir("/src").
		WithExec(miseExec("syft", "file:plexishow", "-o", "spdx-json")).
		Stdout(ctx)
	if err != nil {
		return fmt.Errorf("syft: %w", err)
	}
	return os.WriteFile("sbom.json", []byte(out), 0644)
}

// VulnScan scans the SBOM for vulnerabilities using Grype inside Dagger (via mise)
func VulnScan(ctx context.Context) error {
	mg.Deps(Sbom)
	fmt.Println("Scanning for vulnerabilities in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	sbomFile := client.Host().File("sbom.json")

	out, err := miseContainer(client).
		WithMountedFile("/sbom.json", sbomFile).
		WithWorkdir("/").
		WithExec(miseExec("grype", "sbom:/sbom.json", "-o", "table", "--fail-on", "critical")).
		Stdout(ctx)
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
	golang := miseContainer(client).
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
		WithEnvVariable("CGO_ENABLED", "1").
		WithExec([]string{"mkdir", "-p", "/output"}).
		WithExec(miseExec("go", "mod", "download")).
		WithExec(miseExec("go", "test", "-race", "-coverprofile=/output/coverage.out", "-covermode=atomic", "./...")).
		WithExec(miseExec("go", "tool", "cover", "-func=/output/coverage.out", "-o", "/output/coverage.txt"))

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

// Lint runs golangci-lint inside a Dagger container.
// Uses go install (not mise binary) to ensure Go version compatibility.
func Lint(ctx context.Context) error {
	fmt.Println("Running golangci-lint in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	src := goSrc(client)
	out, err := miseContainer(client).
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
		WithExec(miseExec("go", "mod", "download")).
		WithExec(miseExec("go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@latest")).
		WithExec(miseExec("golangci-lint", "run", "--timeout=5m", "./...")).
		Stdout(ctx)
	if err != nil {
		fmt.Println(out)
		return err
	}
	fmt.Println(out)
	fmt.Println("Lint passed!")
	return nil
}

// Clean removes build artifacts
func Clean() error {
	fmt.Println("Cleaning...")
	for _, p := range []string{"bin", "sbom.json", "coverage.out"} {
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
