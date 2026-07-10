package segy

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

const (
	TextualHeaderSize  = 3200
	BinaryHeaderSize   = 400
	TraceHeaderSize    = 240
	Rev1TextHeaderSize = 3200 + 800
	Rev1MinHeaderSize  = 3600
	InlineHeaderPos    = 189 // byte offset within trace header
	CrosslineHeaderPos = 193
)

var (
	ebcdic2ascii = buildEBCDICTable()
	byteOrders   = map[string]binary.ByteOrder{
		"big":    binary.BigEndian,
		"little": binary.LittleEndian,
	}
)

type SampleFormat int

const (
	FormatIBM   SampleFormat = 1
	FormatInt32 SampleFormat = 2
	FormatInt16 SampleFormat = 3
	FormatIEEE  SampleFormat = 5
	FormatInt8  SampleFormat = 8
	FormatInt64 SampleFormat = 10
)

type TextualHeader struct {
	Raw   [TextualHeaderSize]byte
	ASCII string
	Lines []string
}

type BinaryHeader struct {
	JobID                int32
	LineNumber           int32
	ReelNumber           int32
	TracesPerEnsemble    int16
	AuxTracesPerEnsemble int16
	SampleInterval       int16
	SampleIntervalOrig   int16
	SamplesPerTrace      int16
	SamplesPerTraceOrig  int16
	SampleFormat         SampleFormat
	CDPUnits             int16
	MeasurementSystem    int16
	ByteOrder            string
}

type TraceHeader struct {
	TraceSeqLine   int32
	TraceSeqFile   int32
	FieldRecord    int32
	EnergySourcePt int32
	CDP            int32
	CDPX           int32
	CDPY           int32
	Inline         int32
	Crossline      int32
	Offset         int32
	SampleCount    int16
	SampleInterval int16
	Raw            [TraceHeaderSize]byte
	SourceX        float64
	SourceY        float64
	GroupX         float64
	GroupY         float64
}

type Trace struct {
	Header  TraceHeader
	Samples []float32
}

type SEGYFile struct {
	TextualHeader TextualHeader
	BinaryHeader  BinaryHeader
	ExtendedText  []TextualHeader
	Traces        []Trace
	FilePath      string
	ByteOrder     binary.ByteOrder
}

func buildEBCDICTable() [256]byte {
	var t [256]byte
	for i := range t {
		t[i] = '.'
	}
	// EBCDIC to ASCII mapping (common subset)
	ebcdicMap := map[int]byte{
		0x40: ' ', 0x4B: '.', 0x4E: '+', 0x4F: '|',
		0x50: '&', 0x5A: '!', 0x5B: '$', 0x5C: '*',
		0x5D: ')', 0x5E: ';', 0x5F: '~',
		0x60: '-', 0x61: '/', 0x6B: ',', 0x6C: '%',
		0x6D: '_', 0x6E: '>', 0x6F: '?',
		0x79: '`', 0x7A: ':', 0x7B: '#', 0x7C: '@',
		0x7D: '\'', 0x7E: '=', 0x7F: '"',
		0x81: 'a', 0x82: 'b', 0x83: 'c', 0x84: 'd', 0x85: 'e', 0x86: 'f', 0x87: 'g',
		0x88: 'h', 0x89: 'i',
		0x91: 'j', 0x92: 'k', 0x93: 'l', 0x94: 'm', 0x95: 'n', 0x96: 'o', 0x97: 'p',
		0x98: 'q', 0x99: 'r',
		0xA2: 's', 0xA3: 't', 0xA4: 'u', 0xA5: 'v', 0xA6: 'w', 0xA7: 'x', 0xA8: 'y', 0xA9: 'z',
		0xC1: 'A', 0xC2: 'B', 0xC3: 'C', 0xC4: 'D', 0xC5: 'E', 0xC6: 'F', 0xC7: 'G',
		0xC8: 'H', 0xC9: 'I',
		0xD1: 'J', 0xD2: 'K', 0xD3: 'L', 0xD4: 'M', 0xD5: 'N', 0xD6: 'O', 0xD7: 'P',
		0xD8: 'Q', 0xD9: 'R',
		0xE2: 'S', 0xE3: 'T', 0xE4: 'U', 0xE5: 'V', 0xE6: 'W', 0xE7: 'X', 0xE8: 'Y', 0xE9: 'Z',
		0xF0: '0', 0xF1: '1', 0xF2: '2', 0xF3: '3', 0xF4: '4', 0xF5: '5', 0xF6: '6',
		0xF7: '7', 0xF8: '8', 0xF9: '9',
	}
	for k, v := range ebcdicMap {
		t[k] = v
	}
	return t
}

func (h *TextualHeader) Decode() {
	ascii := make([]byte, TextualHeaderSize)
	for i, b := range h.Raw {
		if b < 128 {
			ascii[i] = ebcdic2ascii[b]
		} else {
			ascii[i] = byte(b)
		}
	}
	h.ASCII = string(ascii)
	lines := make([]string, 0, 40)
	for i := 0; i < TextualHeaderSize; i += 80 {
		end := i + 80
		if end > TextualHeaderSize {
			end = TextualHeaderSize
		}
		lines = append(lines, string(ascii[i:end]))
	}
	h.Lines = lines
}

func (h *TextualHeader) Encode() [TextualHeaderSize]byte {
	return h.Raw
}

func Open(path string) (*SEGYFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("segy: open %s: %v", path, err)
	}
	defer f.Close()
	return Read(f)
}

func Read(r io.Reader) (*SEGYFile, error) {
	segy := &SEGYFile{
		ByteOrder: binary.BigEndian,
	}
	var th [TextualHeaderSize]byte
	if _, err := io.ReadFull(r, th[:]); err != nil {
		return nil, fmt.Errorf("segy: read textual header: %v", err)
	}
	segy.TextualHeader = TextualHeader{Raw: th}
	segy.TextualHeader.Decode()

	var bh [BinaryHeaderSize]byte
	if _, err := io.ReadFull(r, bh[:]); err != nil {
		return nil, fmt.Errorf("segy: read binary header: %v", err)
	}
	segy.BinaryHeader = parseBinaryHeader(bh[:])

	byteOrder := byteOrders[segy.BinaryHeader.ByteOrder]
	if byteOrder == nil {
		byteOrder = binary.BigEndian
	}
	segy.ByteOrder = byteOrder

	if segy.BinaryHeader.SamplesPerTrace < 1 {
		return nil, fmt.Errorf("segy: invalid samples per trace: %d", segy.BinaryHeader.SamplesPerTrace)
	}

	sampleSize := sampleSizeOf(segy.BinaryHeader.SampleFormat)
	if sampleSize < 1 {
		return nil, fmt.Errorf("segy: unsupported sample format: %d", segy.BinaryHeader.SampleFormat)
	}
	_ = sampleSize

	for {
		var traceHeader [TraceHeaderSize]byte
		_, err := io.ReadFull(r, traceHeader[:])
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("segy: read trace header: %v", err)
		}

		sampleCount := int(segy.BinaryHeader.SamplesPerTrace)
		th := parseTraceHeader(traceHeader[:], byteOrder)
		if th.SampleCount > 0 {
			sampleCount = int(th.SampleCount)
		}
		if sampleCount < 1 {
			continue
		}

		samples := make([]float32, sampleCount)
		dataBuf := make([]byte, sampleCount*sampleSize)
		if _, err := io.ReadFull(r, dataBuf); err != nil {
			return nil, fmt.Errorf("segy: read trace data: %v", err)
		}
		decodeSamples(dataBuf, segy.BinaryHeader.SampleFormat, byteOrder, samples)

		segy.Traces = append(segy.Traces, Trace{
			Header:  th,
			Samples: samples,
		})
	}

	return segy, nil
}

func parseBinaryHeader(data []byte) BinaryHeader {
	bo := binary.BigEndian
	get := func(offset, size int) int32 {
		switch size {
		case 2:
			return int32(bo.Uint16(data[offset:]))
		case 4:
			return int32(bo.Uint32(data[offset:]))
		}
		return 0
	}

	bh := BinaryHeader{
		JobID:                get(0, 4),
		LineNumber:           get(4, 4),
		ReelNumber:           get(8, 4),
		TracesPerEnsemble:    int16(get(12, 2)),
		AuxTracesPerEnsemble: int16(get(14, 2)),
		SampleInterval:       int16(get(16, 2)),
		SampleIntervalOrig:   int16(get(18, 2)),
		SamplesPerTrace:      int16(get(20, 2)),
		SamplesPerTraceOrig:  int16(get(22, 2)),
		SampleFormat:         SampleFormat(get(24, 2)),
		CDPUnits:             int16(get(26, 2)),
		MeasurementSystem:    int16(get(28, 2)),
		ByteOrder:            "big",
	}
	if bh.SampleFormat == 0 {
		bh.SampleFormat = FormatIBM
	}
	return bh
}

func parseTraceHeader(data []byte, bo binary.ByteOrder) TraceHeader {
	th := TraceHeader{
		TraceSeqLine:   int32(bo.Uint32(data[0:])),
		TraceSeqFile:   int32(bo.Uint32(data[4:])),
		FieldRecord:    int32(bo.Uint32(data[8:])),
		EnergySourcePt: int32(bo.Uint32(data[12:])),
		CDP:            int32(bo.Uint32(data[20:])),
		CDPX:           int32(bo.Uint32(data[72:])),
		CDPY:           int32(bo.Uint32(data[76:])),
		Offset:         int32(bo.Uint32(data[36:])),
		SampleCount:    int16(bo.Uint16(data[114:])),
		SampleInterval: int16(bo.Uint16(data[116:])),
		SourceX:        float64(int32(bo.Uint32(data[72:]))),
		SourceY:        float64(int32(bo.Uint32(data[76:]))),
		GroupX:         float64(int32(bo.Uint32(data[80:]))),
		GroupY:         float64(int32(bo.Uint32(data[84:]))),
	}
	copy(th.Raw[:], data)

	if InlineHeaderPos+4 <= len(data) {
		th.Inline = int32(bo.Uint32(data[InlineHeaderPos:]))
	}
	if CrosslineHeaderPos+4 <= len(data) {
		th.Crossline = int32(bo.Uint32(data[CrosslineHeaderPos:]))
	}
	return th
}

func sampleSizeOf(format SampleFormat) int {
	switch format {
	case FormatIBM, FormatIEEE:
		return 4
	case FormatInt32:
		return 4
	case FormatInt16:
		return 2
	case FormatInt8:
		return 1
	case FormatInt64:
		return 8
	}
	return 0
}

func decodeSamples(data []byte, format SampleFormat, bo binary.ByteOrder, out []float32) {
	switch format {
	case FormatIEEE:
		for i := range out {
			if i*4+4 <= len(data) {
				out[i] = math.Float32frombits(bo.Uint32(data[i*4:]))
			}
		}
	case FormatIBM:
		for i := range out {
			if i*4+4 <= len(data) {
				out[i] = ibm2ieee(data[i*4:], bo)
			}
		}
	case FormatInt16:
		for i := range out {
			if i*2+2 <= len(data) {
				out[i] = float32(int16(bo.Uint16(data[i*2:])))
			}
		}
	case FormatInt32:
		for i := range out {
			if i*4+4 <= len(data) {
				out[i] = float32(int32(bo.Uint32(data[i*4:])))
			}
		}
	default:
		for i := range out {
			if i*4+4 <= len(data) {
				out[i] = 0
			}
		}
	}
}

func ibm2ieee(data []byte, bo binary.ByteOrder) float32 {
	raw := bo.Uint32(data)
	if raw == 0 {
		return 0
	}
	exp := int((raw>>24)&0x7F) - 64
	mant := float64(raw & 0x00FFFFFF)
	if mant == 0 {
		return 0
	}
	sign := float64(1)
	if (raw>>31)&1 == 1 {
		sign = -1
	}
	return float32(sign * math.Pow(16, float64(exp)) * (mant / 16777216.0))
}

type SEGYWriter struct {
	w    io.Writer
	segy *SEGYFile
}

func NewWriter(w io.Writer, segy *SEGYFile) *SEGYWriter {
	return &SEGYWriter{w: w, segy: segy}
}

func (sw *SEGYWriter) Write() error {
	_, err := sw.w.Write(sw.segy.TextualHeader.Raw[:])
	if err != nil {
		return err
	}
	bh := encodeBinaryHeader(sw.segy.BinaryHeader)
	_, err = sw.w.Write(bh)
	if err != nil {
		return err
	}
	for _, tr := range sw.segy.Traces {
		_, err = sw.w.Write(tr.Header.Raw[:])
		if err != nil {
			return err
		}
		sampleSize := sampleSizeOf(sw.segy.BinaryHeader.SampleFormat)
		if sampleSize < 1 {
			sampleSize = 4
		}
		data := encodeSamples(tr.Samples, sw.segy.BinaryHeader.SampleFormat, sw.segy.ByteOrder)
		_, err = sw.w.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}

func encodeBinaryHeader(bh BinaryHeader) []byte {
	data := make([]byte, BinaryHeaderSize)
	bo := binary.BigEndian
	bo.PutUint32(data[0:], uint32(bh.JobID))
	bo.PutUint32(data[4:], uint32(bh.LineNumber))
	bo.PutUint32(data[8:], uint32(bh.ReelNumber))
	bo.PutUint16(data[12:], uint16(bh.TracesPerEnsemble))
	bo.PutUint16(data[14:], uint16(bh.AuxTracesPerEnsemble))
	bo.PutUint16(data[16:], uint16(bh.SampleInterval))
	bo.PutUint16(data[18:], uint16(bh.SampleIntervalOrig))
	bo.PutUint16(data[20:], uint16(bh.SamplesPerTrace))
	bo.PutUint16(data[22:], uint16(bh.SamplesPerTraceOrig))
	bo.PutUint16(data[24:], uint16(bh.SampleFormat))
	return data
}

func encodeSamples(samples []float32, format SampleFormat, bo binary.ByteOrder) []byte {
	switch format {
	case FormatIEEE:
		data := make([]byte, len(samples)*4)
		for i, v := range samples {
			bo.PutUint32(data[i*4:], math.Float32bits(v))
		}
		return data
	default:
		data := make([]byte, len(samples)*4)
		for i, v := range samples {
			bo.PutUint32(data[i*4:], math.Float32bits(v))
		}
		return data
	}
}

func (s *SEGYFile) InlineCount() int {
	if len(s.Traces) == 0 {
		return 0
	}
	seen := make(map[int32]bool)
	for _, tr := range s.Traces {
		seen[tr.Header.Inline] = true
	}
	return len(seen)
}

func (s *SEGYFile) CrosslineCount() int {
	if len(s.Traces) == 0 {
		return 0
	}
	seen := make(map[int32]bool)
	for _, tr := range s.Traces {
		seen[tr.Header.Crossline] = true
	}
	return len(seen)
}

func (s *SEGYFile) JSON(indent bool) (string, error) {
	type segyJSON struct {
		FilePath       string       `json:"file_path,omitempty"`
		InlineCount    int          `json:"inline_count"`
		CrosslineCount int          `json:"crossline_count"`
		SampleCount    int          `json:"sample_count"`
		SampleFormat   SampleFormat `json:"sample_format"`
		TraceCount     int          `json:"trace_count"`
	}

	sj := segyJSON{
		FilePath:       s.FilePath,
		InlineCount:    s.InlineCount(),
		CrosslineCount: s.CrosslineCount(),
		SampleCount:    int(s.BinaryHeader.SamplesPerTrace),
		SampleFormat:   s.BinaryHeader.SampleFormat,
		TraceCount:     len(s.Traces),
	}

	var b []byte
	var err error
	if indent {
		b, err = json.MarshalIndent(sj, "", "  ")
	} else {
		b, err = json.Marshal(sj)
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *SEGYFile) TextualHeaderAsText() string {
	s.TextualHeader.Decode()
	return s.TextualHeader.ASCII
}

func (s *SEGYFile) FileName() string {
	return filepath.Base(s.FilePath)
}
