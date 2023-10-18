package stateful_proxy

import (
	"testing"
)

func TestTrue(t *testing.T) {
	if 1 != 2 {
		t.Fatalf("This test should not fail")
	}
}
