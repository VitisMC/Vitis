package network_test

import (
	"bytes"
	"compress/zlib"
	"testing"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// BenchmarkChunkCompression measures outbound framing for representative chunk data.
func BenchmarkChunkCompression(b *testing.B) {
	c := section.NewChunk(0, 0, section.OverworldSections, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			height := 48 + (x*17+z*13)%40
			for y := -64; y < height; y++ {
				state := genblock.StoneDefaultState
				if y >= height-3 {
					state = genblock.DirtDefaultState
				}
				if y > 8 && y < 48 && (x*19+y*7+z*11)%23 == 0 {
					state = 0
				}
				c.SetBlock(x, y, z, state)
			}
			c.SetBlock(x, height, z, genblock.GrassBlockDefaultState)
		}
	}
	payload := c.EncodePacketPayload()
	b.Logf("chunk payload bytes: %d", len(payload))
	defaultFrame, err := network.CompressFrame(payload, 256)
	if err != nil {
		b.Fatal(err)
	}
	var fastFrame bytes.Buffer
	fastWriter, err := zlib.NewWriterLevel(&fastFrame, zlib.BestSpeed)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := fastWriter.Write(payload); err != nil {
		b.Fatal(err)
	}
	if err := fastWriter.Close(); err != nil {
		b.Fatal(err)
	}
	b.Logf("compressed bytes: current=%d fast=%d", len(defaultFrame), fastFrame.Len())
	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := network.CompressFrame(payload, 256); err != nil {
				b.Fatal(err)
			}
		}
	})
	for _, tc := range []struct {
		name  string
		level int
	}{{"fast", zlib.BestSpeed}, {"default", zlib.DefaultCompression}} {
		b.Run(tc.name, func(b *testing.B) {
			var output bytes.Buffer
			w, err := zlib.NewWriterLevel(&output, tc.level)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				output.Reset()
				w.Reset(&output)
				_, err = w.Write(payload)
				if err != nil {
					b.Fatal(err)
				}
				if err = w.Close(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
