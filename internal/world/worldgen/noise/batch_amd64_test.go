//go:build amd64 && !purego

package noise

import (
	"os"
	"testing"
)

// TestNoiseWithoutAVX2 exercises runtime dispatch on an emulated or physical non-AVX2 processor.
func TestNoiseWithoutAVX2(t *testing.T) {
	if os.Getenv("VITIS_TEST_NO_AVX2") != "1" {
		t.Skip("requires a non-AVX2 processor and VITIS_TEST_NO_AVX2=1")
	}
	if hasAVX2 {
		t.Fatal("runtime selected AVX2 on a processor expected to lack it")
	}
	t.Run("exact_noise", TestNoiseBatchesMatchScalar)
}
