package segy

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func makeTestSEGY() *SEGYFile {
	th := TextualHeader{}
	copy(th.Raw[:], []byte("TEST SEG-Y FILE"))

	bh := BinaryHeader{
		SamplesPerTrace: 10,
		SampleInterval:  4,
		SampleFormat:    FormatIEEE,
		ByteOrder:       "big",
		TracesPerEnsemble: 1,
	}

	var traces []Trace
	for il := int32(1); il <= 3; il++ {
		for xl := int32(1); xl <= 3; xl++ {
			var thBuf [TraceHeaderSize]byte
			bo := binary.BigEndian
			bo.PutUint32(thBuf[InlineHeaderPos:], uint32(il))
			bo.PutUint32(thBuf[CrosslineHeaderPos:], uint32(xl))

			samples := make([]float32, 10)
			for i := range samples {
				samples[i] = float32(int(il)*100 + int(xl)*10 + i)
			}

			traces = append(traces, Trace{
				Header: TraceHeader{
					Inline:    il,
					Crossline: xl,
					Raw:       thBuf,
				},
				Samples: samples,
			})
		}
	}

	return &SEGYFile{
		TextualHeader: th,
		BinaryHeader:  bh,
		Traces:        traces,
		ByteOrder:     binary.BigEndian,
	}
}

func TestReadWriteRoundTrip(t *testing.T) {
	orig := makeTestSEGY()

	var buf bytes.Buffer
	w := NewWriter(&buf, orig)
	if err := w.Write(); err != nil {
		t.Fatal(err)
	}

	parsed, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if len(parsed.Traces) != len(orig.Traces) {
		t.Errorf("expected %d traces, got %d", len(orig.Traces), len(parsed.Traces))
	}

	for i, tr := range parsed.Traces {
		if tr.Header.Inline != orig.Traces[i].Header.Inline {
			t.Errorf("trace %d: expected inline %d, got %d", i, orig.Traces[i].Header.Inline, tr.Header.Inline)
		}
		if tr.Header.Crossline != orig.Traces[i].Header.Crossline {
			t.Errorf("trace %d: expected crossline %d, got %d", i, orig.Traces[i].Header.Crossline, tr.Header.Crossline)
		}
		if len(tr.Samples) != len(orig.Traces[i].Samples) {
			t.Errorf("trace %d: expected %d samples, got %d", i, len(orig.Traces[i].Samples), len(tr.Samples))
		}
		for j, v := range tr.Samples {
			if math.Abs(float64(v-orig.Traces[i].Samples[j])) > 0.001 {
				t.Errorf("trace %d sample %d: expected %f, got %f", i, j, orig.Traces[i].Samples[j], v)
			}
		}
	}
}

func TestIBM2IEEE(t *testing.T) {
	bo := binary.BigEndian
	tests := []struct {
		ibmHex   uint32
		expected float32
	}{
		{0x00000000, 0},
		{0x41100000, 1.0},  // IBM 1.0
		{0xC1100000, -1.0}, // IBM -1.0
	}
	for _, tt := range tests {
		data := make([]byte, 4)
		bo.PutUint32(data, tt.ibmHex)
		v := ibm2ieee(data, bo)
		if math.Abs(float64(v-tt.expected)) > 0.01 {
			t.Errorf("0x%08X: expected %f, got %f", tt.ibmHex, tt.expected, v)
		}
	}
}

func TestParseTextualHeader(t *testing.T) {
	var raw [TextualHeaderSize]byte
	// SEG-Y 用 EBCDIC 编码，ASCII 'S'=0x53 在 EBCDIC 表中是 0x53 位不常用的码点
	// 直接用已知 EBCDIC 码点: 0xC5=E, 0xC7=G, 0xC9=I, 0x40=space
	// 但我们用原始字节测试至少不崩溃
	th := TextualHeader{Raw: raw}
	th.Decode()
	if len(th.ASCII) != TextualHeaderSize {
		t.Errorf("expected decoded ASCII length %d, got %d", TextualHeaderSize, len(th.ASCII))
	}
	if len(th.Lines) != 40 {
		t.Errorf("expected 40 lines, got %d", len(th.Lines))
	}
}

func TestInlineCrosslineCount(t *testing.T) {
	segy := makeTestSEGY()
	if segy.InlineCount() != 3 {
		t.Errorf("expected 3 inlines, got %d", segy.InlineCount())
	}
	if segy.CrosslineCount() != 3 {
		t.Errorf("expected 3 crosslines, got %d", segy.CrosslineCount())
	}
}

func TestJSON(t *testing.T) {
	segy := makeTestSEGY()
	json, err := segy.JSON(true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(json, "inline_count") {
		t.Errorf("JSON missing inline_count")
	}
	if !strings.Contains(json, "trace_count") {
		t.Errorf("JSON missing trace_count")
	}
}

func TestSampleFormatConstants(t *testing.T) {
	if FormatIBM != 1 {
		t.Errorf("expected FormatIBM=1")
	}
	if FormatIEEE != 5 {
		t.Errorf("expected FormatIEEE=5")
	}
}
