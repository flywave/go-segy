package segy

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenFileMinimal(t *testing.T) {
	segy, err := Open("testdata/test.segy")
	if err != nil {
		t.Skip("test data not found:", err)
	}
	assert.NotNil(t, segy)
	assert.Greater(t, len(segy.Traces), 0)
	t.Logf("Traces: %d", len(segy.Traces))
	t.Logf("Samples per trace: %d", segy.BinaryHeader.SamplesPerTrace)
	t.Logf("Sample format: %d", segy.BinaryHeader.SampleFormat)
	t.Logf("Text header: %s", segy.TextualHeaderAsText()[:80])
}

func TestOpenFileE5(t *testing.T) {
	segy, err := Open("testdata/E5_MIG_DMO_FINAL.sgy")
	if err != nil {
		t.Skip("E5 test data not found:", err)
	}
	assert.NotNil(t, segy)
	assert.Greater(t, len(segy.Traces), 0)

	json, err := segy.JSON(true)
	assert.NoError(t, err)
	if len(json) > 200 {
		t.Logf("SEG-Y summary:\n%s", json[:200])
	} else {
		t.Logf("SEG-Y summary:\n%s", json)
	}

	first := segy.Traces[0]
	t.Logf("First trace: inline=%d, crossline=%d, samples=%d",
		first.Header.Inline, first.Header.Crossline, len(first.Samples))

	sampleMin, sampleMax := float32(1e10), float32(-1e10)
	for _, tr := range segy.Traces {
		for _, v := range tr.Samples {
			if v < sampleMin {
				sampleMin = v
			}
			if v > sampleMax {
				sampleMax = v
			}
		}
	}
	t.Logf("Amplitude range: [%.2f, %.2f]", sampleMin, sampleMax)
}

func TestOpenFileInlineCrossline(t *testing.T) {
	segy, err := Open("testdata/E5_MIG_DMO_FINAL.sgy")
	if err != nil {
		t.Skip("test data not found:", err)
	}

	ilCount := segy.InlineCount()
	xlCount := segy.CrosslineCount()
	t.Logf("Inlines: %d, Crosslines: %d", ilCount, xlCount)

	trByIL := make(map[int32]int)
	for _, tr := range segy.Traces {
		trByIL[tr.Header.Inline]++
	}
	for il, count := range trByIL {
		t.Logf("  Inline %d: %d traces", il, count)
		break // just show first
	}
	assert.Greater(t, ilCount, 0)
}

func TestOpenFileSampleFormats(t *testing.T) {
	segy, err := Open("testdata/E5_MIG_DMO_FINAL.sgy")
	if err != nil {
		t.Skip("test data not found:", err)
	}

	assert.Equal(t, FormatIBM, segy.BinaryHeader.SampleFormat)
	assert.Equal(t, int16(1000), segy.BinaryHeader.SampleInterval)
	assert.Equal(t, binary.BigEndian, segy.ByteOrder)

	f, err := os.Open("testdata/E5_MIG_DMO_FINAL.sgy")
	if err != nil {
		t.Skip()
	}
	defer f.Close()
	segy2, err := Read(f)
	assert.NoError(t, err)
	assert.Equal(t, len(segy.Traces), len(segy2.Traces))
}
