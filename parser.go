package pdffill

import (
	"bytes"
	"fmt"
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
	// Parse trailer dictionary
	trailerIdx := bytes.Index(t.data[xrefOffset:], []byte("trailer"))
	if trailerIdx == -1 {
		return nil, fmt.Errorf("trailer not found")
	}

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
func (t *Template) getObject(objNum int) (*pdfObject, error) {
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
