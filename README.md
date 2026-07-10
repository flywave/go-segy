# go-segy

Go library for reading and writing SEG-Y seismic data format.

## Features

- Read SEG-Y rev 1.0 files
- Parse textual header (EBCDIC→ASCII), binary header, trace headers
- Support multiple sample formats: IEEE float, IBM float, int16, int32
- IBM float to IEEE float conversion
- Inline/Crossline extraction from trace headers
- Write SEG-Y files
- JSON metadata export
