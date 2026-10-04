package update

import "testing"

func TestGreaterUsesSemanticVersions(t *testing.T) {
	if !greater("v0.10.0", "v0.9.0") {
		t.Fatal("0.10.0 must be newer than 0.9.0")
	}
	if greater("v0.9.0", "v0.10.0") {
		t.Fatal("0.9.0 must not be newer than 0.10.0")
	}
	if greater("v1.0.0", "v1.0.0") {
		t.Fatal("same version must not be newer")
	}
}
