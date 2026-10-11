//go:build !amd64 || purego

package noise

const hasAVX2 = false

// improvedBatchAVX2 supplies the unreachable architecture fallback without foreign code.
func improvedBatchAVX2(input *improvedBatchInput, result *[4]float64) {
	for lane := range result {
		var values [8]float64
		for corner := range values {
			x, y, z := input.x, input.y[lane], input.z
			if corner&1 != 0 {
				x--
			}
			if corner&2 != 0 {
				y--
			}
			if corner&4 != 0 {
				z--
			}
			index := input.gradients[corner][lane]
			values[corner] = dot(gradients[index], x, y, z)
		}
		result[lane] = lerp3(input.smoothX, smoothstep(input.fractionY[lane]), input.smoothZ, &values)
	}
}
