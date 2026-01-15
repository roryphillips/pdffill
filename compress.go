package pdffill

import (
	"bytes"
	"compress/flate"
	"fmt"
)

// compressStream compresses a PDF stream using flate compression.
// Returns the compressed stream data with updated dictionary entries.
func compressStream(streamData []byte) ([]byte, error) {
	var buf bytes.Buffer

	// Use flate compression (same as zlib but without headers)
	// Level 6 is a good balance between speed and compression
	fw, err := flate.NewWriter(&buf, 6)
	if err != nil {
		return nil, fmt.Errorf("create flate writer: %w", err)
	}

	if _, err := fw.Write(streamData); err != nil {
		return nil, fmt.Errorf("compress stream: %w", err)
	}

	if err := fw.Close(); err != nil {
		return nil, fmt.Errorf("close flate writer: %w", err)
	}

	return buf.Bytes(), nil
}

// compressObject compresses stream objects in PDF content.
// Identifies stream objects and compresses their data.
func compressObject(content []byte) ([]byte, bool, error) {
	// Find stream keyword
	streamIdx := bytes.Index(content, []byte("\nstream\n"))
	if streamIdx == -1 {
		streamIdx = bytes.Index(content, []byte("\rstream\r"))
	}
	if streamIdx == -1 {
		streamIdx = bytes.Index(content, []byte("\rstream\n"))
	}
	if streamIdx == -1 {
		// Not a stream object, return as-is
		return content, false, nil
	}

	// Find endstream
	endstreamIdx := bytes.Index(content[streamIdx:], []byte("endstream"))
	if endstreamIdx == -1 {
		return content, false, nil
	}
	endstreamIdx += streamIdx

	// Extract dictionary (before stream)
	dictEnd := streamIdx

	// Extract stream data (between "stream\n" and "endstream")
	streamStart := streamIdx + len("\nstream\n")
	streamEnd := endstreamIdx

	// Handle different line endings
	if content[streamIdx] == '\r' {
		if streamIdx+8 < len(content) && content[streamIdx+8] == '\r' {
			streamStart = streamIdx + len("\rstream\r")
		} else {
			streamStart = streamIdx + len("\rstream\n")
		}
	}

	streamData := content[streamStart:streamEnd]

	// Skip if already compressed (check for /FlateDecode filter)
	if bytes.Contains(content[:dictEnd], []byte("/FlateDecode")) ||
		bytes.Contains(content[:dictEnd], []byte("/Filter /FlateDecode")) {
		return content, false, nil
	}

	// Compress the stream data
	compressed, err := compressStream(streamData)
	if err != nil {
		return nil, false, err
	}

	// Only use compression if it actually reduces size
	if len(compressed) >= len(streamData) {
		return content, false, nil
	}

	// Build new object with compressed stream
	var result bytes.Buffer

	// Write dictionary with updated /Length and /Filter
	dict := content[:dictEnd]

	// Add /Filter /FlateDecode if not present
	if !bytes.Contains(dict, []byte("/Filter")) {
		// Insert filter before closing >>
		closeIdx := bytes.LastIndex(dict, []byte(">>"))
		if closeIdx == -1 {
			return content, false, nil
		}

		result.Write(dict[:closeIdx])
		result.WriteString("/Filter /FlateDecode\n")
		result.Write(dict[closeIdx:])
	} else {
		result.Write(dict)
	}

	// Update /Length entry
	newDict := result.Bytes()
	newDict = updateLength(newDict, len(compressed))

	// Write updated dictionary
	result.Reset()
	result.Write(newDict)

	// Write compressed stream
	result.WriteString("\nstream\n")
	result.Write(compressed)
	result.WriteString("\nendstream")

	// Write remainder
	result.Write(content[endstreamIdx+len("endstream"):])

	return result.Bytes(), true, nil
}

// updateLength updates or adds the /Length entry in a PDF dictionary.
func updateLength(dict []byte, newLength int) []byte {
	lengthIdx := bytes.Index(dict, []byte("/Length"))
	if lengthIdx == -1 {
		// Add /Length entry before >>
		closeIdx := bytes.LastIndex(dict, []byte(">>"))
		if closeIdx == -1 {
			return dict
		}

		var result bytes.Buffer
		result.Write(dict[:closeIdx])
		result.WriteString(fmt.Sprintf("/Length %d\n", newLength))
		result.Write(dict[closeIdx:])
		return result.Bytes()
	}

	// Find the value after /Length
	start := lengthIdx + len("/Length")
	for start < len(dict) && isWhitespace(dict[start]) {
		start++
	}

	// Skip past the number (could be direct value or reference)
	end := start
	for end < len(dict) && (isDigit(dict[end]) || dict[end] == ' ' || dict[end] == 'R') {
		end++
	}

	// Replace the length value
	var result bytes.Buffer
	result.Write(dict[:start])
	result.WriteString(fmt.Sprintf("%d", newLength))

	// Skip old value - find next whitespace or delimiter
	skipEnd := start
	for skipEnd < len(dict) && !isWhitespace(dict[skipEnd]) &&
		dict[skipEnd] != '/' && dict[skipEnd] != '>' {
		skipEnd++
	}

	result.Write(dict[skipEnd:])
	return result.Bytes()
}
