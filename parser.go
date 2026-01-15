package pdffill

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// pdfObject represents a parsed PDF object with its number and content.
type pdfObject struct {
	num        int
	generation int
	offset     int
	content    []byte
}

// parseFields implements the PDF parsing to locate AcroForm fields.
func (t *Template) parseFields() error {
	// Find cross-reference table
	xref, err := t.findXRef()
	if err != nil {
		return err
	}

	// Parse trailer to find catalog
	catalog, err := t.findCatalog(xref)
	if err != nil {
		return err
	}

	// Find AcroForm dictionary in catalog
	acroForm, err := t.findAcroForm(catalog)
	if err != nil {
		return err
	}

	// Parse Fields array to get all form fields
	if err := t.parseFieldArray(acroForm); err != nil {
		return err
	}

	return nil
}

// findXRef locates the cross-reference table offset.
func (t *Template) findXRef() (int, error) {
	// Find "startxref" keyword from end of file
	startxref := []byte("startxref")
	idx := bytes.LastIndex(t.data, startxref)
	if idx == -1 {
		return 0, fmt.Errorf("startxref not found")
	}

	// Read offset after startxref
	end := idx + len(startxref)
	eofIdx := bytes.Index(t.data[end:], []byte("%%EOF"))
	if eofIdx == -1 {
		return 0, fmt.Errorf("EOF marker not found")
	}

	offsetStr := strings.TrimSpace(string(t.data[end : end+eofIdx]))
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		return 0, fmt.Errorf("invalid xref offset: %w", err)
	}

	return offset, nil
}

// findCatalog locates and returns the catalog dictionary.
func (t *Template) findCatalog(xrefOffset int) (*pdfObject, error) {
	// First try traditional trailer dictionary
	trailerIdx := bytes.Index(t.data[xrefOffset:], []byte("trailer"))
	if trailerIdx != -1 {
		trailerStart := xrefOffset + trailerIdx + 7 // len("trailer")
		trailerDict, err := t.parseDictionary(trailerStart)
		if err != nil {
			return nil, fmt.Errorf("parse trailer: %w", err)
		}

		// Find /Root reference (catalog)
		rootRef := extractReference(trailerDict, "/Root")
		if rootRef == 0 {
			return nil, fmt.Errorf("catalog reference not found in trailer")
		}

		// Get catalog object
		catalog, err := t.getObject(rootRef)
		if err != nil {
			return nil, fmt.Errorf("get catalog: %w", err)
		}

		return catalog, nil
	}

	// No traditional trailer - try XRef stream (PDF 1.5+)
	return t.findCatalogFromXRefStream(xrefOffset)
}

// findCatalogFromXRefStream handles PDF 1.5+ XRef streams where trailer info
// is embedded in the XRef stream object dictionary.
func (t *Template) findCatalogFromXRefStream(xrefOffset int) (*pdfObject, error) {
	// XRef stream starts with "N 0 obj" at the offset
	// Find the object at xrefOffset
	if xrefOffset >= len(t.data) {
		return nil, fmt.Errorf("xref offset beyond file: %d", xrefOffset)
	}

	// Look for object definition starting near the offset
	// Format: "N 0 obj<</Type/XRef...>>"
	searchStart := xrefOffset
	if searchStart > 50 {
		searchStart -= 50 // Look a bit before in case of whitespace
	}

	// Find "obj" keyword near the offset
	chunk := t.data[searchStart:]
	objIdx := bytes.Index(chunk, []byte(" 0 obj"))
	if objIdx == -1 {
		return nil, fmt.Errorf("XRef stream object not found at offset %d", xrefOffset)
	}

	// Find object number before " 0 obj"
	numEnd := searchStart + objIdx
	numStart := numEnd
	for numStart > 0 && (t.data[numStart-1] >= '0' && t.data[numStart-1] <= '9') {
		numStart--
	}

	// Parse the XRef stream dictionary to get /Root
	dictStart := searchStart + objIdx + 6 // len(" 0 obj")

	// Skip whitespace and find dictionary start
	for dictStart < len(t.data) && (t.data[dictStart] == ' ' || t.data[dictStart] == '\n' || t.data[dictStart] == '\r') {
		dictStart++
	}

	if dictStart >= len(t.data) || t.data[dictStart] != '<' {
		return nil, fmt.Errorf("XRef stream dictionary not found")
	}

	// Parse the dictionary
	xrefDict, err := t.parseDictionary(dictStart)
	if err != nil {
		return nil, fmt.Errorf("parse XRef stream dict: %w", err)
	}

	// Verify it's an XRef stream
	if !bytes.Contains(xrefDict, []byte("/Type/XRef")) && !bytes.Contains(xrefDict, []byte("/Type /XRef")) {
		return nil, fmt.Errorf("object at xref offset is not an XRef stream")
	}

	// Find /Root reference
	rootRef := extractReference(xrefDict, "/Root")
	if rootRef == 0 {
		return nil, fmt.Errorf("catalog reference not found in XRef stream")
	}

	// Get catalog object
	catalog, err := t.getObject(rootRef)
	if err != nil {
		return nil, fmt.Errorf("get catalog: %w", err)
	}

	return catalog, nil
}

// findAcroForm locates the AcroForm dictionary from the catalog.
func (t *Template) findAcroForm(catalog *pdfObject) (*pdfObject, error) {
	acroFormRef := extractReference(catalog.content, "/AcroForm")
	if acroFormRef == 0 {
		return nil, fmt.Errorf("AcroForm not found in catalog")
	}

	acroForm, err := t.getObject(acroFormRef)
	if err != nil {
		return nil, fmt.Errorf("get AcroForm: %w", err)
	}

	return acroForm, nil
}

// parseFieldArray extracts all field references from the AcroForm Fields array.
func (t *Template) parseFieldArray(acroForm *pdfObject) error {
	// Find /Fields array
	fieldsIdx := bytes.Index(acroForm.content, []byte("/Fields"))
	if fieldsIdx == -1 {
		return fmt.Errorf("Fields array not found")
	}

	// Find array start
	arrayStart := bytes.IndexByte(acroForm.content[fieldsIdx:], '[')
	if arrayStart == -1 {
		return fmt.Errorf("Fields array opening bracket not found")
	}
	arrayStart += fieldsIdx

	// Find array end
	arrayEnd := bytes.IndexByte(acroForm.content[arrayStart:], ']')
	if arrayEnd == -1 {
		return fmt.Errorf("Fields array closing bracket not found")
	}
	arrayEnd += arrayStart

	// Parse field references in array
	arrayContent := acroForm.content[arrayStart+1 : arrayEnd]
	refs := parseReferences(arrayContent)

	// Get each field object and extract name
	for _, ref := range refs {
		field, err := t.getObject(ref)
		if err != nil {
			continue // Skip invalid fields
		}

		fieldName := extractName(field.content, "/T")
		if fieldName == "" {
			continue // Skip unnamed fields
		}

		t.fields[fieldName] = &fieldRef{
			objNum: field.num,
			offset: field.offset,
			length: len(field.content),
		}
	}

	return nil
}

// getObject retrieves a PDF object by its object number.
// Uses LastIndex to handle incremental updates where objects may appear multiple times.
// Also searches in object streams for PDF 1.5+ compressed objects.
func (t *Template) getObject(objNum int) (*pdfObject, error) {
	// First try direct object lookup
	obj, err := t.getDirectObject(objNum)
	if err == nil {
		return obj, nil
	}

	// If not found directly, search in object streams (PDF 1.5+)
	return t.getObjectFromStream(objNum)
}

// getDirectObject retrieves a directly-defined PDF object.
func (t *Template) getDirectObject(objNum int) (*pdfObject, error) {
	// Search for "N 0 obj" with proper boundaries to avoid matching "21 0 obj" when looking for "1 0 obj"
	pattern := []byte(fmt.Sprintf("\n%d 0 obj", objNum))
	idx := bytes.LastIndex(t.data, pattern)
	if idx == -1 {
		// Try without leading newline (for objects at start of file)
		pattern = []byte(fmt.Sprintf("%d 0 obj", objNum))
		idx = bytes.Index(t.data, pattern)
		if idx == -1 || (idx > 0 && isDigit(t.data[idx-1])) {
			return nil, fmt.Errorf("object %d not found", objNum)
		}
	} else {
		idx++ // Skip the newline we searched for
	}

	objStart := idx + len(pattern) - 1 // -1 because we added \n to pattern
	if t.data[idx] != '\n' {
		objStart = idx + len(pattern)
	}

	endObj := bytes.Index(t.data[objStart:], []byte("endobj"))
	if endObj == -1 {
		return nil, fmt.Errorf("endobj not found for object %d", objNum)
	}

	return &pdfObject{
		num:        objNum,
		generation: 0,
		offset:     idx,
		content:    t.data[objStart : objStart+endObj],
	}, nil
}

// getObjectFromStream retrieves an object from a compressed object stream (PDF 1.5+).
func (t *Template) getObjectFromStream(objNum int) (*pdfObject, error) {
	// First, try to find the object using XRef stream (if available)
	objStmNum, objIndex, err := t.findObjectInXRefStream(objNum)
	if err == nil && objStmNum > 0 {
		// Found in XRef stream - get from specific object stream
		return t.getObjectFromObjStm(objStmNum, objNum, objIndex)
	}

	// Fallback: search all object streams linearly
	objStmPattern := []byte("/Type/ObjStm")
	objStmPatternAlt := []byte("/Type /ObjStm")

	pos := 0
	for pos < len(t.data) {
		idx := bytes.Index(t.data[pos:], objStmPattern)
		idxAlt := bytes.Index(t.data[pos:], objStmPatternAlt)

		if idx == -1 && idxAlt == -1 {
			break
		}

		if idx == -1 || (idxAlt != -1 && idxAlt < idx) {
			idx = idxAlt
		}
		idx += pos

		objStart := bytes.LastIndex(t.data[:idx], []byte(" 0 obj"))
		if objStart == -1 {
			pos = idx + 10
			continue
		}

		numEnd := objStart
		numStart := numEnd - 1
		for numStart >= 0 && isDigit(t.data[numStart]) {
			numStart--
		}
		numStart++

		obj, err := t.extractObjectFromObjStm(numStart, objNum)
		if err == nil {
			return obj, nil
		}

		pos = idx + 10
	}

	return nil, fmt.Errorf("object %d not found in any object stream", objNum)
}

// findObjectInXRefStream looks up an object in the XRef stream to find its location.
// Returns (objStmNum, indexInObjStm, error) for type 2 entries, or (0, offset, error) for type 1.
func (t *Template) findObjectInXRefStream(objNum int) (int, int, error) {
	// Find startxref
	startxrefIdx := bytes.LastIndex(t.data, []byte("startxref"))
	if startxrefIdx == -1 {
		return 0, 0, fmt.Errorf("startxref not found")
	}

	// Parse xref offset
	offsetStart := startxrefIdx + 9
	for offsetStart < len(t.data) && isWhitespace(t.data[offsetStart]) {
		offsetStart++
	}
	offsetEnd := offsetStart
	for offsetEnd < len(t.data) && isDigit(t.data[offsetEnd]) {
		offsetEnd++
	}

	xrefOffset, _ := strconv.Atoi(string(t.data[offsetStart:offsetEnd]))
	if xrefOffset == 0 || xrefOffset >= len(t.data) {
		return 0, 0, fmt.Errorf("invalid xref offset")
	}

	// Check if it's an XRef stream (not traditional xref table)
	chunk := t.data[xrefOffset:]
	if !bytes.Contains(chunk[:min(500, len(chunk))], []byte("/Type/XRef")) &&
		!bytes.Contains(chunk[:min(500, len(chunk))], []byte("/Type /XRef")) {
		return 0, 0, fmt.Errorf("not an XRef stream")
	}

	// Parse W array (entry format)
	wIdx := bytes.Index(chunk, []byte("/W"))
	if wIdx == -1 {
		return 0, 0, fmt.Errorf("W array not found")
	}

	wStart := wIdx + 2
	for wStart < len(chunk) && chunk[wStart] != '[' {
		wStart++
	}
	wStart++

	var wVals []int
	for wStart < len(chunk) && chunk[wStart] != ']' {
		if isDigit(chunk[wStart]) {
			val := 0
			for wStart < len(chunk) && isDigit(chunk[wStart]) {
				val = val*10 + int(chunk[wStart]-'0')
				wStart++
			}
			wVals = append(wVals, val)
		} else {
			wStart++
		}
	}

	if len(wVals) < 3 {
		return 0, 0, fmt.Errorf("invalid W array")
	}

	entrySize := wVals[0] + wVals[1] + wVals[2]

	// Find and decompress stream
	streamIdx := bytes.Index(chunk, []byte(">>stream"))
	if streamIdx == -1 {
		return 0, 0, fmt.Errorf("stream not found")
	}

	streamStart := streamIdx + 8
	if chunk[streamStart] == '\r' {
		streamStart++
	}
	if chunk[streamStart] == '\n' {
		streamStart++
	}

	endstreamIdx := bytes.Index(chunk[streamStart:], []byte("endstream"))
	if endstreamIdx == -1 {
		return 0, 0, fmt.Errorf("endstream not found")
	}

	streamData := chunk[streamStart : streamStart+endstreamIdx]

	// Decompress with zlib
	reader := bytes.NewReader(streamData)
	zlibReader, err := zlib.NewReader(reader)
	if err != nil {
		return 0, 0, fmt.Errorf("zlib reader: %w", err)
	}
	defer zlibReader.Close()

	decompressed, err := io.ReadAll(zlibReader)
	if err != nil {
		return 0, 0, fmt.Errorf("decompress: %w", err)
	}

	// Check for predictor encoding
	columns := entrySize
	if bytes.Contains(chunk[:streamIdx], []byte("/Predictor")) {
		// PNG predictor - decode it
		decompressed = decodePNGPredictor(decompressed, columns)
	}

	// Look up the object
	entryOffset := objNum * entrySize
	if entryOffset+entrySize > len(decompressed) {
		return 0, 0, fmt.Errorf("object %d beyond xref data", objNum)
	}

	entry := decompressed[entryOffset : entryOffset+entrySize]

	// Parse entry based on W array
	typ := 0
	if wVals[0] > 0 {
		for i := 0; i < wVals[0]; i++ {
			typ = (typ << 8) | int(entry[i])
		}
	} else {
		typ = 1 // Default type is 1 if W[0] is 0
	}

	field2 := 0
	for i := 0; i < wVals[1]; i++ {
		field2 = (field2 << 8) | int(entry[wVals[0]+i])
	}

	field3 := 0
	for i := 0; i < wVals[2]; i++ {
		field3 = (field3 << 8) | int(entry[wVals[0]+wVals[1]+i])
	}

	switch typ {
	case 0:
		return 0, 0, fmt.Errorf("object %d is free", objNum)
	case 1:
		// Uncompressed object at offset field2
		return 0, field2, nil
	case 2:
		// Compressed in ObjStm field2, at index field3
		return field2, field3, nil
	default:
		return 0, 0, fmt.Errorf("unknown xref entry type %d", typ)
	}
}

// decodePNGPredictor decodes PNG predictor-encoded data.
func decodePNGPredictor(data []byte, columns int) []byte {
	rowSize := columns + 1 // +1 for filter byte
	rows := len(data) / rowSize
	if rows == 0 {
		return data
	}

	decoded := make([]byte, rows*columns)
	prevRow := make([]byte, columns)

	for row := 0; row < rows; row++ {
		srcOffset := row * rowSize
		if srcOffset+rowSize > len(data) {
			break
		}

		filterByte := data[srcOffset]
		rowData := data[srcOffset+1 : srcOffset+1+columns]
		dstOffset := row * columns

		switch filterByte {
		case 0: // None
			copy(decoded[dstOffset:], rowData)
		case 1: // Sub
			for i := 0; i < columns; i++ {
				if i == 0 {
					decoded[dstOffset+i] = rowData[i]
				} else {
					decoded[dstOffset+i] = rowData[i] + decoded[dstOffset+i-1]
				}
			}
		case 2: // Up
			for i := 0; i < columns; i++ {
				decoded[dstOffset+i] = rowData[i] + prevRow[i]
			}
		default:
			copy(decoded[dstOffset:], rowData)
		}

		copy(prevRow, decoded[dstOffset:dstOffset+columns])
	}

	return decoded
}

// getObjectFromObjStm retrieves an object from a specific object stream.
func (t *Template) getObjectFromObjStm(objStmNum, targetObjNum, targetIndex int) (*pdfObject, error) {
	// Get the object stream
	objStm, err := t.getDirectObject(objStmNum)
	if err != nil {
		return nil, fmt.Errorf("get object stream %d: %w", objStmNum, err)
	}

	// Get First value
	firstVal := extractIntValue(objStm.content, "/First")
	if firstVal == 0 {
		return nil, fmt.Errorf("invalid object stream: First not found")
	}

	// Decompress stream
	streamData, err := t.extractAndDecompressStream(objStm.content)
	if err != nil {
		return nil, fmt.Errorf("decompress object stream: %w", err)
	}

	// Parse header to find our object
	if firstVal > len(streamData) {
		return nil, fmt.Errorf("First value beyond stream data")
	}

	header := streamData[:firstVal]
	parts := bytes.Fields(header)

	// Find the object at targetIndex
	if targetIndex*2+1 >= len(parts) {
		return nil, fmt.Errorf("index %d beyond header", targetIndex)
	}

	offset, _ := strconv.Atoi(string(parts[targetIndex*2+1]))
	contentStart := firstVal + offset

	// Find content end
	var contentEnd int
	if targetIndex*2+3 < len(parts) {
		nextOffset, _ := strconv.Atoi(string(parts[targetIndex*2+3]))
		contentEnd = firstVal + nextOffset
	} else {
		contentEnd = len(streamData)
	}

	if contentStart >= len(streamData) || contentEnd > len(streamData) {
		return nil, fmt.Errorf("object content out of bounds")
	}

	return &pdfObject{
		num:        targetObjNum,
		generation: 0,
		offset:     0,
		content:    streamData[contentStart:contentEnd],
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractObjectFromObjStm extracts a specific object from an object stream.
func (t *Template) extractObjectFromObjStm(objStmStart int, targetObjNum int) (*pdfObject, error) {
	// Get the object stream object
	objStm, err := t.getDirectObject(parseObjNumAt(t.data, objStmStart))
	if err != nil {
		return nil, err
	}

	// Parse the object stream dictionary to get /N (number of objects) and /First (offset to first object)
	nVal := extractIntValue(objStm.content, "/N")
	firstVal := extractIntValue(objStm.content, "/First")
	if nVal == 0 || firstVal == 0 {
		return nil, fmt.Errorf("invalid object stream: N=%d, First=%d", nVal, firstVal)
	}

	// Find and decompress the stream data
	streamData, err := t.extractAndDecompressStream(objStm.content)
	if err != nil {
		return nil, fmt.Errorf("decompress object stream: %w", err)
	}

	// Parse the object number/offset pairs at the beginning of the stream
	// Format: "objNum1 offset1 objNum2 offset2 ..." (space-separated integers)
	header := streamData[:firstVal]
	parts := bytes.Fields(header)

	if len(parts) < 2 || len(parts)%2 != 0 {
		return nil, fmt.Errorf("invalid object stream header")
	}

	// Find our target object
	for i := 0; i < len(parts); i += 2 {
		objNumStr := string(parts[i])
		offsetStr := string(parts[i+1])

		objNum, err1 := strconv.Atoi(objNumStr)
		offset, err2 := strconv.Atoi(offsetStr)
		if err1 != nil || err2 != nil {
			continue
		}

		if objNum == targetObjNum {
			// Found it! Extract the object content
			objContentStart := firstVal + offset

			// Find end of this object (start of next object or end of stream)
			var objContentEnd int
			if i+2 < len(parts) {
				nextOffsetStr := string(parts[i+3])
				nextOffset, _ := strconv.Atoi(nextOffsetStr)
				objContentEnd = firstVal + nextOffset
			} else {
				objContentEnd = len(streamData)
			}

			if objContentStart >= len(streamData) || objContentEnd > len(streamData) {
				return nil, fmt.Errorf("object content out of bounds")
			}

			return &pdfObject{
				num:        targetObjNum,
				generation: 0,
				offset:     0,
				content:    streamData[objContentStart:objContentEnd],
			}, nil
		}
	}

	return nil, fmt.Errorf("object %d not in this object stream", targetObjNum)
}

// parseObjNumAt parses an object number starting at the given position.
func parseObjNumAt(data []byte, pos int) int {
	end := pos
	for end < len(data) && isDigit(data[end]) {
		end++
	}
	if end == pos {
		return 0
	}
	num, _ := strconv.Atoi(string(data[pos:end]))
	return num
}

// extractIntValue extracts an integer value for a given key from a dictionary.
func extractIntValue(dict []byte, key string) int {
	idx := bytes.Index(dict, []byte(key))
	if idx == -1 {
		return 0
	}

	start := idx + len(key)
	for start < len(dict) && (dict[start] == ' ' || dict[start] == '\n' || dict[start] == '\r') {
		start++
	}

	end := start
	for end < len(dict) && isDigit(dict[end]) {
		end++
	}

	if end == start {
		return 0
	}

	val, _ := strconv.Atoi(string(dict[start:end]))
	return val
}

// extractAndDecompressStream extracts and decompresses stream data from an object.
func (t *Template) extractAndDecompressStream(objContent []byte) ([]byte, error) {
	// Find stream keyword
	streamIdx := bytes.Index(objContent, []byte("stream"))
	if streamIdx == -1 {
		return nil, fmt.Errorf("stream keyword not found")
	}

	// Find start of stream data (after "stream" and newline)
	dataStart := streamIdx + 6 // len("stream")
	if dataStart < len(objContent) && objContent[dataStart] == '\r' {
		dataStart++
	}
	if dataStart < len(objContent) && objContent[dataStart] == '\n' {
		dataStart++
	}

	// Find end of stream data
	endstreamIdx := bytes.Index(objContent[dataStart:], []byte("endstream"))
	if endstreamIdx == -1 {
		return nil, fmt.Errorf("endstream not found")
	}

	streamData := objContent[dataStart : dataStart+endstreamIdx]

	// Check if compressed
	if bytes.Contains(objContent[:streamIdx], []byte("/FlateDecode")) {
		return decompressFlate(streamData)
	}

	return streamData, nil
}

// decompressFlate decompresses zlib-encoded data (PDF FlateDecode filter).
// PDF FlateDecode uses zlib format (deflate with header and checksum).
func decompressFlate(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	zlibReader, err := zlib.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("zlib reader: %w", err)
	}
	defer zlibReader.Close()

	result, err := io.ReadAll(zlibReader)
	if err != nil {
		return nil, fmt.Errorf("zlib decompress: %w", err)
	}

	return result, nil
}

// parseDictionary parses a PDF dictionary starting at the given offset.
func (t *Template) parseDictionary(offset int) ([]byte, error) {
	dictStart := bytes.IndexByte(t.data[offset:], '<')
	if dictStart == -1 || offset+dictStart+1 >= len(t.data) {
		return nil, fmt.Errorf("dictionary start not found")
	}

	if t.data[offset+dictStart+1] != '<' {
		return nil, fmt.Errorf("not a dictionary")
	}

	dictStart += offset + 2
	depth := 1
	pos := dictStart

	for pos < len(t.data) && depth > 0 {
		if t.data[pos] == '<' && pos+1 < len(t.data) && t.data[pos+1] == '<' {
			depth++
			pos++
		} else if t.data[pos] == '>' && pos+1 < len(t.data) && t.data[pos+1] == '>' {
			depth--
			pos++
		}
		pos++
	}

	if depth != 0 {
		return nil, fmt.Errorf("unmatched dictionary delimiters")
	}

	return t.data[dictStart-2 : pos], nil
}

// extractReference extracts an indirect object reference from dictionary content.
func extractReference(dict []byte, key string) int {
	idx := bytes.Index(dict, []byte(key))
	if idx == -1 {
		return 0
	}

	// Find the reference number after the key
	start := idx + len(key)
	for start < len(dict) && isWhitespace(dict[start]) {
		start++
	}

	// Read digits
	end := start
	for end < len(dict) && isDigit(dict[end]) {
		end++
	}

	if end == start {
		return 0
	}

	num, _ := strconv.Atoi(string(dict[start:end]))
	return num
}

// extractName extracts a name value from dictionary content.
func extractName(dict []byte, key string) string {
	idx := bytes.Index(dict, []byte(key))
	if idx == -1 {
		return ""
	}

	// Find the name after the key (either in parentheses or as literal)
	start := idx + len(key)
	for start < len(dict) && isWhitespace(dict[start]) {
		start++
	}

	if start >= len(dict) {
		return ""
	}

	// Check for string literal (...)
	if dict[start] == '(' {
		end := bytes.IndexByte(dict[start+1:], ')')
		if end == -1 {
			return ""
		}
		return string(dict[start+1 : start+1+end])
	}

	// Check for hex string <...>
	if dict[start] == '<' {
		end := bytes.IndexByte(dict[start+1:], '>')
		if end == -1 {
			return ""
		}
		return decodeHexString(dict[start+1 : start+1+end])
	}

	return ""
}

// parseReferences extracts all object references from array content.
func parseReferences(arrayContent []byte) []int {
	var refs []int
	tokens := bytes.Fields(arrayContent)

	for i := 0; i < len(tokens); i++ {
		if num, err := strconv.Atoi(string(tokens[i])); err == nil {
			// Check if followed by "0 R" pattern
			if i+2 < len(tokens) && string(tokens[i+1]) == "0" && string(tokens[i+2]) == "R" {
				refs = append(refs, num)
				i += 2
			}
		}
	}

	return refs
}

// decodeHexString converts a hex string to regular string.
func decodeHexString(hex []byte) string {
	result := make([]byte, 0, len(hex)/2)
	for i := 0; i < len(hex)-1; i += 2 {
		high := hexValue(hex[i])
		low := hexValue(hex[i+1])
		if high != 255 && low != 255 {
			result = append(result, (high<<4)|low)
		}
	}
	return string(result)
}

// hexValue returns the numeric value of a hex character.
func hexValue(c byte) byte {
	if c >= '0' && c <= '9' {
		return c - '0'
	}
	if c >= 'A' && c <= 'F' {
		return c - 'A' + 10
	}
	if c >= 'a' && c <= 'f' {
		return c - 'a' + 10
	}
	return 255
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == 0
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
