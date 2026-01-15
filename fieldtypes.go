package pdffill

import (
	"bytes"
	"fmt"
)

// FieldType represents the type of a PDF form field.
//
// PDF forms support various field types defined in the AcroForm specification.
// This library auto-detects field types based on PDF dictionary entries like
// /FT (field type) and /Ff (field flags).
type FieldType int

const (
	// FieldTypeUnknown indicates an unrecognized or unsupported field type.
	FieldTypeUnknown FieldType = iota

	// FieldTypeText is a single-line text field (/FT /Tx).
	FieldTypeText

	// FieldTypeMultilineText is a multi-line text field (/FT /Tx with /Ff bit 12).
	FieldTypeMultilineText

	// FieldTypeNumber is a numeric text field.
	FieldTypeNumber

	// FieldTypeCheckbox is a checkbox button (/FT /Btn without /Kids).
	FieldTypeCheckbox

	// FieldTypeRadio is a radio button group (/FT /Btn with /Kids).
	FieldTypeRadio

	// FieldTypeChoice is a dropdown or list field (/FT /Ch).
	FieldTypeChoice
)

// String returns the string representation of a field type.
func (ft FieldType) String() string {
	switch ft {
	case FieldTypeText:
		return "Text"
	case FieldTypeMultilineText:
		return "MultilineText"
	case FieldTypeNumber:
		return "Number"
	case FieldTypeCheckbox:
		return "Checkbox"
	case FieldTypeRadio:
		return "Radio"
	case FieldTypeChoice:
		return "Choice"
	default:
		return "Unknown"
	}
}

// FieldInfo contains metadata about a PDF form field.
//
// This provides detailed information about a field's type, location in the PDF,
// and type-specific metadata like radio button options or choice lists.
//
// Use GetFieldInfo to inspect fields before filling them, which is useful for:
//   - Building dynamic UI forms
//   - Validating field types match expected data
//   - Discovering radio button options
//   - Understanding field structure
type FieldInfo struct {
	Name    string    // Field name as it appears in the PDF
	Type    FieldType // Detected field type
	ObjNum  int       // PDF object number containing this field
	Offset  int       // Byte offset in the PDF (internal use)
	Length  int       // Length of field content (internal use)
	HasKids bool      // True for radio button groups with /Kids
	KidRefs []int     // Child object references for radio button groups
	Options []string  // Available options for choice fields
}

// GetFieldInfo returns detailed information about a specific field.
//
// This method inspects the field's PDF object to determine its type and
// extract type-specific metadata.
//
// Example:
//
//	info, err := template.GetFieldInfo("employment_status")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	switch info.Type {
//	case pdffill.FieldTypeRadio:
//		fmt.Printf("Radio button group with %d options\n", len(info.KidRefs))
//	case pdffill.FieldTypeCheckbox:
//		fmt.Println("Checkbox field")
//	case pdffill.FieldTypeText:
//		fmt.Println("Text field")
//	}
//
// Returns an error if the field doesn't exist in the template.
func (t *Template) GetFieldInfo(fieldName string) (*FieldInfo, error) {
	fieldRef, exists := t.fields[fieldName]
	if !exists {
		return nil, fmt.Errorf("field %q not found", fieldName)
	}

	obj, err := t.getObject(fieldRef.objNum)
	if err != nil {
		return nil, fmt.Errorf("get field object: %w", err)
	}

	info := &FieldInfo{
		Name:   fieldName,
		ObjNum: fieldRef.objNum,
		Offset: fieldRef.offset,
		Length: fieldRef.length,
	}

	// Detect field type
	info.Type = detectFieldType(obj.content)

	// Extract additional metadata based on type
	if info.Type == FieldTypeRadio {
		info.HasKids = true
		info.KidRefs = extractKidRefs(obj.content)
	}

	if info.Type == FieldTypeChoice {
		info.Options = extractOptions(obj.content)
	}

	return info, nil
}

// detectFieldType determines the type of a field from its content.
func detectFieldType(content []byte) FieldType {
	// Check for button fields
	if bytes.Contains(content, []byte("/FT /Btn")) {
		// Radio button groups have /Kids
		if bytes.Contains(content, []byte("/Kids")) {
			return FieldTypeRadio
		}
		// Otherwise it's a checkbox
		return FieldTypeCheckbox
	}

	// Check for choice fields
	if bytes.Contains(content, []byte("/FT /Ch")) {
		return FieldTypeChoice
	}

	// Check for text fields
	if bytes.Contains(content, []byte("/FT /Tx")) {
		// Check for multiline flag (bit 12 = 4096)
		ffIdx := bytes.Index(content, []byte("/Ff "))
		if ffIdx != -1 {
			start := ffIdx + 4
			for start < len(content) && isWhitespace(content[start]) {
				start++
			}
			end := start
			for end < len(content) && isDigit(content[end]) {
				end++
			}
			if end > start {
				flags, _ := parseInt(content[start:end])
				// Bit 12 (4096) = Multiline
				if flags&(1<<12) != 0 {
					return FieldTypeMultilineText
				}
			}
		}
		return FieldTypeText
	}

	return FieldTypeUnknown
}

// extractKidRefs extracts child object references from a radio button group.
func extractKidRefs(content []byte) []int {
	kidsIdx := bytes.Index(content, []byte("/Kids"))
	if kidsIdx == -1 {
		return nil
	}

	// Find array start
	arrayStart := bytes.IndexByte(content[kidsIdx:], '[')
	if arrayStart == -1 {
		return nil
	}
	arrayStart += kidsIdx

	// Find array end
	arrayEnd := bytes.IndexByte(content[arrayStart:], ']')
	if arrayEnd == -1 {
		return nil
	}
	arrayEnd += arrayStart

	arrayContent := content[arrayStart+1 : arrayEnd]
	return parseReferences(arrayContent)
}

// extractOptions extracts option values from a choice field.
func extractOptions(content []byte) []string {
	optIdx := bytes.Index(content, []byte("/Opt"))
	if optIdx == -1 {
		return nil
	}

	// Find array start
	arrayStart := bytes.IndexByte(content[optIdx:], '[')
	if arrayStart == -1 {
		return nil
	}
	arrayStart += optIdx

	// Find array end
	arrayEnd := bytes.IndexByte(content[arrayStart:], ']')
	if arrayEnd == -1 {
		return nil
	}
	arrayEnd += arrayStart

	// Parse string array
	arrayContent := content[arrayStart+1 : arrayEnd]
	return parseStringArray(arrayContent)
}

// parseStringArray extracts strings from a PDF array.
func parseStringArray(data []byte) []string {
	var result []string
	pos := 0

	for pos < len(data) {
		// Skip whitespace
		for pos < len(data) && isWhitespace(data[pos]) {
			pos++
		}

		if pos >= len(data) {
			break
		}

		// Check for string literal
		if data[pos] == '(' {
			end := pos + 1
			depth := 1
			for end < len(data) && depth > 0 {
				if data[end] == '\\' && end+1 < len(data) {
					end += 2
					continue
				}
				if data[end] == '(' {
					depth++
				} else if data[end] == ')' {
					depth--
				}
				end++
			}
			result = append(result, string(data[pos+1:end-1]))
			pos = end
		} else if data[pos] == '<' {
			// Hex string
			end := bytes.IndexByte(data[pos:], '>')
			if end == -1 {
				break
			}
			end += pos + 1
			result = append(result, decodeHexString(data[pos+1:end-1]))
			pos = end
		} else {
			pos++
		}
	}

	return result
}
