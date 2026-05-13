//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

	src := client.Host().Directory(".")
	golang := client.Container().
		From("golang:1.22").
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
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

	src := client.Host().Directory(".")
	ldflags := fmt.Sprintf("-ldflags=-s -w -X main.version=%s", version)

	golang := client.Container().
		From("golang:1.22").
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
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

// Sbom generates SBOM using Syft inside Dagger
func Sbom(ctx context.Context) error {
	mg.Deps(Build)
	fmt.Println("Generating SBOM in Dagger...")
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	src := client.Host().Directory(".")

	syft := client.Container().
		From("anchore/syft:latest").
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
		WithExec([]string{"syft", "bin/plexishow", "-o", "spdx-json", "--file", "/output/sbom.json"})

	_, err = syft.File("/output/sbom.json").Export(ctx, "sbom.json")
	if err != nil {
		return err
	}
	fmt.Println("SBOM saved to sbom.json")
	return nil
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

	src := client.Host().Directory(".")

	grype := client.Container().
		From("anchore/grype:latest").
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
		WithExec([]string{"grype", "sbom:sbom.json", "-o", "table", "--fail-on", "critical"})

	out, err := grype.Stdout(ctx)
	if err != nil {
		fmt.Println(out)
		return err
	}
	fmt.Println(out)
	return nil
}

// All runs fmt, vet, test, build
func All(ctx context.Context) {
	mg.Deps(Fmt, Vet)
	mg.Deps(Test)
	mg.Deps(Build)
}
