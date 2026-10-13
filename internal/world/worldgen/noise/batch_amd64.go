//go:build amd64 && !purego

package noise

// hasAVX2 checks both processor instructions and operating-system vector state support.
var hasAVX2 = detectAVX2()

// detectAVX2 prevents vector execution on unsupported processors and operating systems.
func detectAVX2() bool {
	maximum, _, _, _ := noiseCPUID(0, 0)
	if maximum < 7 {
		return false
	}
	_, _, features, _ := noiseCPUID(1, 0)
	if features&(1<<27) == 0 || features&(1<<28) == 0 {
		return false
	}
	state, _ := noiseXGETBV()
	if state&6 != 6 {
		return false
	}
	_, extended, _, _ := noiseCPUID(7, 0)
	return extended&(1<<5) != 0
}

// noiseCPUID returns architectural feature bits.
func noiseCPUID(leaf, subleaf uint32) (a, b, c, d uint32)

// noiseXGETBV returns the enabled operating-system vector state.
func noiseXGETBV() (a, d uint32)

// improvedBatchAVX2 preserves scalar dot products and x/y/z interpolation without FMA.
//
//go:noescape
func improvedBatchAVX2(input *improvedBatchInput, result *[4]float64)
