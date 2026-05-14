package main

import "testing"

func TestVersionSet(t *testing.T) {
	if version == "" {
		t.Error("version should not be empty")
	}
}
