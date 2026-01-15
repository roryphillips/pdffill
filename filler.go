package pdffill

import (
	"bytes"
	"fmt"
)

// Fill fills the form fields and returns a new PDF with the provided values.
//
// This is the primary method for filling PDF forms. It takes a map of field names
// to values and returns a complete, filled PDF as a byte slice.
//
// The method supports all common AcroForm field types:
//   - Text fields (single-line and multiline)
//   - Number fields
//   - Checkboxes (accepts: Yes/No, On/Off, true/false, 1/0, X, checked)
//   - Radio buttons
//
// Performance: ~6-10ms per fill operation (after template parsing).
//
// Example:
//
//	formData := map[string]string{
//		"name":     "John Doe",
//		"email":    "john@example.com",
//		"age":      "30",
//		"agreed":   "Yes",
//	}
//
//	filledPDF, err := template.Fill(formData)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	os.WriteFile("output.pdf", filledPDF, 0644)
//
// Returns an error if:
//   - formData is empty
//   - any field name doesn't exist in the template
//   - field value cannot be set (e.g., malformed PDF object)
//
// For validation and advanced options, see FillWithOptions.
func (t *Template) Fill(formData map[string]string) ([]byte, error) {
	if len(formData) == 0 {
		return nil, fmt.Errorf("no form data provided")
	}

	// Build object replacements map
	replacements := make(map[int][]byte)

	for fieldName, value := range formData {
		field, exists := t.fields[fieldName]
		if !exists {
			return nil, fmt.Errorf("field %q not found in template", fieldName)
		}

		// Get original object
		obj, err := t.getObject(field.objNum)
		if err != nil {
			return nil, fmt.Errorf("get field object %d: %w", field.objNum, err)
		}

		// Create modified object with filled value
		newContent, err := t.setFieldValue(obj.content, value)
		if err != nil {
			return nil, fmt.Errorf("set field %q: %w", fieldName, err)
		}

		replacements[field.objNum] = newContent
	}

	// Rebuild PDF with replacements
	result, err := t.rebuildPDF(replacements)
	if err != nil {
		return nil, fmt.Errorf("rebuild PDF: %w", err)
	}

	return result, nil
}

// setFieldValue modifies a field object's /V (value) entry.
// Handles different field types: text, checkbox, radio buttons, etc.
func (t *Template) setFieldValue(objContent []byte, value string) ([]byte, error) {
	// Detect field type to determine how to set the value
	fieldType := detectFieldType(objContent)

	switch fieldType {
	case FieldTypeCheckbox:
		return t.setCheckboxValue(objContent, value)
	case FieldTypeRadio:
		return t.setRadioValue(objContent, value)
	case FieldTypeText, FieldTypeMultilineText, FieldTypeNumber:
		return t.setTextValue(objContent, value)
	default:
		// Default to text value
		return t.setTextValue(objContent, value)
	}
}

// setTextValue sets the value for text, multiline, and number fields.
func (t *Template) setTextValue(objContent []byte, value string) ([]byte, error) {
	// Find /V entry in dictionary
	vIdx := bytes.Index(objContent, []byte("/V"))
	if vIdx == -1 {
		// No /V entry, need to add it
		return t.addFieldValue(objContent, value)
	}

	// Find the value after /V
	start := vIdx + 2 // len("/V")
	for start < len(objContent) && isWhitespace(objContent[start]) {
		start++
	}

	var end int
	if objContent[start] == '(' {
		// String literal
		end = start + 1
		depth := 1
		for end < len(objContent) && depth > 0 {
			if objContent[end] == '\\' && end+1 < len(objContent) {
				end += 2
				continue
			}
			if objContent[end] == '(' {
				depth++
			} else if objContent[end] == ')' {
				depth--
			}
			end++
		}
	} else if objContent[start] == '<' {
		// Hex string
		end = bytes.IndexByte(objContent[start:], '>')
		if end == -1 {
			return nil, fmt.Errorf("unterminated hex string")
		}
		end += start + 1
	} else if objContent[start] == '/' {
		// Name object
		end = start + 1
		for end < len(objContent) && !isWhitespace(objContent[end]) && !isDelimiter(objContent[end]) {
			end++
		}
	} else {
		return nil, fmt.Errorf("unsupported value type")
	}

	// Replace old value with new value
	newValue := encodePDFString(value)
	result := make([]byte, 0, len(objContent)-(end-start)+len(newValue))
	result = append(result, objContent[:start]...)
	result = append(result, newValue...)
	result = append(result, objContent[end:]...)

	return result, nil
}

// addFieldValue adds a /V entry to a field object that doesn't have one.
func (t *Template) addFieldValue(objContent []byte, value string) ([]byte, error) {
	// Find end of dictionary (before >>)
	dictEnd := bytes.LastIndex(objContent, []byte(">>"))
	if dictEnd == -1 {
		return nil, fmt.Errorf("dictionary end not found")
	}

	newValue := encodePDFString(value)
	result := make([]byte, 0, len(objContent)+4+len(newValue))
	result = append(result, objContent[:dictEnd]...)
	result = append(result, []byte("/V ")...)
	result = append(result, newValue...)
	result = append(result, '\n')
	result = append(result, objContent[dictEnd:]...)

	return result, nil
}

// rebuildPDF reconstructs the PDF with modified objects.
func (t *Template) rebuildPDF(replacements map[int][]byte) ([]byte, error) {
	var buf bytes.Buffer
	xrefEntries := make(map[int]int) // object number -> offset

	// Write PDF header
	headerEnd := bytes.IndexByte(t.data, '\n')
	if headerEnd == -1 {
		return nil, fmt.Errorf("invalid PDF header")
	}
	buf.Write(t.data[:headerEnd+1])

	// Find all objects and write them, applying replacements
	pos := headerEnd + 1

	for pos < len(t.data) {
		// Find next object
		objStart := bytes.Index(t.data[pos:], []byte(" 0 obj"))
		if objStart == -1 {
			break
		}
		objStart += pos

		// Find object number
		numStart := objStart - 1
		for numStart >= 0 && isDigit(t.data[numStart]) {
			numStart--
		}
		numStart++

		currentObjNum, _ := parseInt(t.data[numStart:objStart])

		// Find endobj
		endObjIdx := bytes.Index(t.data[objStart:], []byte("endobj"))
		if endObjIdx == -1 {
			break
		}
		endObjIdx += objStart + 6 // len("endobj")

		// Record offset
		xrefEntries[currentObjNum] = buf.Len()

		// Check if we have a replacement for this object
		if newContent, exists := replacements[currentObjNum]; exists {
			// Write modified object
			buf.WriteString(fmt.Sprintf("%d 0 obj\n", currentObjNum))
			buf.Write(newContent)
			buf.WriteString("\nendobj\n")
		} else {
			// Write original object
			buf.Write(t.data[numStart:endObjIdx])
			buf.WriteByte('\n')
		}

		pos = endObjIdx
	}

	// Write cross-reference table
	xrefOffset := buf.Len()
	if err := writeXRef(&buf, xrefEntries); err != nil {
		return nil, fmt.Errorf("write xref: %w", err)
	}

	// Write trailer
	if err := t.writeTrailer(&buf, xrefOffset, len(xrefEntries)); err != nil {
		return nil, fmt.Errorf("write trailer: %w", err)
	}

	return buf.Bytes(), nil
}

// writeXRef writes the cross-reference table.
func writeXRef(buf *bytes.Buffer, entries map[int]int) error {
	if len(entries) == 0 {
		return fmt.Errorf("no xref entries")
	}

	// Find max object number
	maxObj := 0
	for objNum := range entries {
		if objNum > maxObj {
			maxObj = objNum
		}
	}

	buf.WriteString("xref\n")
	buf.WriteString(fmt.Sprintf("0 %d\n", maxObj+1))
	buf.WriteString("0000000000 65535 f \n")

	for i := 1; i <= maxObj; i++ {
		if offset, exists := entries[i]; exists {
			buf.WriteString(fmt.Sprintf("%010d 00000 n \n", offset))
		} else {
			buf.WriteString("0000000000 00000 f \n")
		}
	}

	return nil
}

// writeTrailer writes the trailer dictionary.
func (t *Template) writeTrailer(buf *bytes.Buffer, xrefOffset, size int) error {
	// Find original trailer
	trailerIdx := bytes.LastIndex(t.data, []byte("trailer"))
	if trailerIdx == -1 {
		return fmt.Errorf("original trailer not found")
	}

	// Parse original trailer dict
	trailerDict, err := t.parseDictionary(trailerIdx + 7)
	if err != nil {
		return fmt.Errorf("parse original trailer: %w", err)
	}

	// Write trailer with updated Size
	buf.WriteString("trailer\n")

	// Modify Size in trailer
	newTrailer := updateSize(trailerDict, size)
	buf.Write(newTrailer)
	buf.WriteString("\n")

	// Write startxref
	buf.WriteString("startxref\n")
	buf.WriteString(fmt.Sprintf("%d\n", xrefOffset))
	buf.WriteString("%%EOF\n")

	return nil
}

// updateSize updates the /Size entry in a trailer dictionary.
func updateSize(trailer []byte, newSize int) []byte {
	sizeIdx := bytes.Index(trailer, []byte("/Size"))
	if sizeIdx == -1 {
		// Add Size entry
		end := bytes.Index(trailer, []byte(">>"))
		if end == -1 {
			return trailer
		}
		result := make([]byte, 0, len(trailer)+20)
		result = append(result, trailer[:end]...)
		result = append(result, []byte(fmt.Sprintf("/Size %d\n", newSize))...)
		result = append(result, trailer[end:]...)
		return result
	}

	// Replace existing Size
	start := sizeIdx + 5 // len("/Size")
	for start < len(trailer) && isWhitespace(trailer[start]) {
		start++
	}

	end := start
	for end < len(trailer) && isDigit(trailer[end]) {
		end++
	}

	result := make([]byte, 0, len(trailer))
	result = append(result, trailer[:start]...)
	result = append(result, []byte(fmt.Sprintf("%d", newSize))...)
	result = append(result, trailer[end:]...)

	return result
}

// encodePDFString encodes a string as a PDF string literal.
func encodePDFString(s string) []byte {
	var buf bytes.Buffer
	buf.WriteByte('(')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '(', ')', '\\':
			buf.WriteByte('\\')
			buf.WriteByte(c)
		case '\r':
			buf.WriteString("\\r")
		case '\n':
			buf.WriteString("\\n")
		case '\t':
			buf.WriteString("\\t")
		default:
			buf.WriteByte(c)
		}
	}
	buf.WriteByte(')')
	return buf.Bytes()
}

// parseInt parses an integer from a byte slice.
func parseInt(b []byte) (int, error) {
	// Trim whitespace
	start := 0
	for start < len(b) && isWhitespace(b[start]) {
		start++
	}

	end := start
	for end < len(b) && isDigit(b[end]) {
		end++
	}

	if end == start {
		return 0, fmt.Errorf("no digits found")
	}

	num := 0
	for i := start; i < end; i++ {
		num = num*10 + int(b[i]-'0')
	}

	return num, nil
}

func isDelimiter(c byte) bool {
	return c == '(' || c == ')' || c == '<' || c == '>' || c == '[' || c == ']' || c == '{' || c == '}' || c == '/' || c == '%'
}
