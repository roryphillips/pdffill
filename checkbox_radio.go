package pdffill

import (
	"bytes"
	"fmt"
)

// setCheckboxValue sets the value for a checkbox field.
// Accepts: "On", "Yes", "true", "1", "checked" for checked state
// Accepts: "Off", "No", "false", "0", "unchecked", "" for unchecked state
func (t *Template) setCheckboxValue(objContent []byte, value string) ([]byte, error) {
	// Normalize the value to /Yes or /Off
	var pdfValue string
	switch value {
	case "On", "Yes", "yes", "true", "1", "checked", "X", "x":
		pdfValue = "/Yes"
	case "Off", "No", "no", "false", "0", "unchecked", "":
		pdfValue = "/Off"
	default:
		// Try to be lenient - anything truthy becomes checked
		if value != "" {
			pdfValue = "/Yes"
		} else {
			pdfValue = "/Off"
		}
	}

	return t.replaceNameValue(objContent, pdfValue)
}

// setRadioValue sets the value for a radio button group.
// The value should be the name of one of the radio options.
func (t *Template) setRadioValue(objContent []byte, value string) ([]byte, error) {
	// Radio button values are typically name objects like /Option1, /Option2
	// If the value doesn't start with /, add it
	pdfValue := value
	if len(value) > 0 && value[0] != '/' {
		pdfValue = "/" + value
	}

	// If empty, set to no selection (typically represented as /)
	if value == "" || value == "Off" || value == "None" {
		pdfValue = "/"
	}

	return t.replaceNameValue(objContent, pdfValue)
}

// replaceNameValue replaces a PDF name value (for checkboxes and radios).
func (t *Template) replaceNameValue(objContent []byte, pdfValue string) ([]byte, error) {
	// Find /V entry in dictionary
	vIdx := bytes.Index(objContent, []byte("/V"))
	if vIdx == -1 {
		// No /V entry, need to add it
		return t.addNameValue(objContent, pdfValue)
	}

	// Find the value after /V
	start := vIdx + 2 // len("/V")
	for start < len(objContent) && isWhitespace(objContent[start]) {
		start++
	}

	// For name objects, find the end (either whitespace or delimiter)
	end := start
	if objContent[start] == '/' {
		// It's a name object like /Off or /Yes
		end = start + 1
		for end < len(objContent) && !isWhitespace(objContent[end]) && !isDelimiter(objContent[end]) {
			end++
		}
	} else if objContent[start] == '(' {
		// String literal (less common for buttons)
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
	} else {
		return nil, fmt.Errorf("unexpected value format")
	}

	// Replace old value with new value
	result := make([]byte, 0, len(objContent)-(end-start)+len(pdfValue))
	result = append(result, objContent[:start]...)
	result = append(result, []byte(pdfValue)...)
	if end < len(objContent) {
		result = append(result, objContent[end:]...)
	}

	// Also update /AS (appearance state) if present
	result = t.updateAppearanceState(result, pdfValue)

	return result, nil
}

// addNameValue adds a /V entry with a name value to a field object.
func (t *Template) addNameValue(objContent []byte, pdfValue string) ([]byte, error) {
	// Find end of dictionary (before >>)
	dictEnd := bytes.LastIndex(objContent, []byte(">>"))
	if dictEnd == -1 {
		return nil, fmt.Errorf("dictionary end not found")
	}

	result := make([]byte, 0, len(objContent)+10+len(pdfValue))
	result = append(result, objContent[:dictEnd]...)
	result = append(result, []byte("/V ")...)
	result = append(result, []byte(pdfValue)...)
	result = append(result, ' ')
	result = append(result, objContent[dictEnd:]...)

	return result, nil
}

// updateAppearanceState updates the /AS (appearance state) to match /V.
// This ensures the checkbox/radio button displays correctly.
func (t *Template) updateAppearanceState(objContent []byte, value string) []byte {
	asIdx := bytes.Index(objContent, []byte("/AS"))
	if asIdx == -1 {
		// No /AS entry, don't add one (PDF viewer will handle it)
		return objContent
	}

	// Find the value after /AS
	start := asIdx + 3 // len("/AS")
	for start < len(objContent) && isWhitespace(objContent[start]) {
		start++
	}

	// Find end of the name
	end := start
	if objContent[start] == '/' {
		end = start + 1
		for end < len(objContent) && !isWhitespace(objContent[end]) && !isDelimiter(objContent[end]) {
			end++
		}
	} else {
		return objContent // Can't update if not a name
	}

	// Replace with new value
	result := make([]byte, 0, len(objContent))
	result = append(result, objContent[:start]...)
	result = append(result, []byte(value)...)
	if end < len(objContent) {
		result = append(result, objContent[end:]...)
	}

	return result
}
