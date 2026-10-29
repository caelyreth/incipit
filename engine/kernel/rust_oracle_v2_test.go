package kernel

import (
	"encoding/json"
	"os"
	"testing"
)

func TestRustOracleV2DocumentAuthorityEvidence(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../fixtures/rust-oracle/v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Fixture    string `json:"fixture"`
		RustCommit string `json:"rust_commit"`
		Cases      []struct {
			Variant string `json:"variant"`
			Debug   string `json:"debug"`
			Display string `json:"display"`
			Source  bool   `json:"source"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	var authority *struct {
		Fixture    string `json:"fixture"`
		RustCommit string `json:"rust_commit"`
		Cases      []struct {
			Variant string `json:"variant"`
			Debug   string `json:"debug"`
			Display string `json:"display"`
			Source  bool   `json:"source"`
		} `json:"cases"`
	}
	for index := range fixtures {
		if fixtures[index].Fixture == "kernel-document-authority-v2" {
			authority = &fixtures[index]
			break
		}
	}
	if authority == nil || len(authority.RustCommit) != 40 || len(authority.Cases) != 2 {
		t.Fatalf("kernel authority fixture = %#v", authority)
	}
	want := []struct {
		variant string
		debug   string
		display string
	}{
		{"UndeclaredDocumentRead", "UndeclaredDocumentRead", "undeclared read of the semantic document"},
		{"UndeclaredDocumentWrite", "UndeclaredDocumentWrite", "undeclared write of the semantic document"},
	}
	for index, expected := range want {
		got := authority.Cases[index]
		if got.Variant != expected.variant || got.Debug != expected.debug ||
			got.Display != expected.display || got.Source {
			t.Fatalf("case %d = %#v, want %#v with no source", index, got, expected)
		}
	}
}
