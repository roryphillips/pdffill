package pdffill

import (
	"bytes"
	_ "embed"
	"fmt"
	"testing"
)

//go:embed testdata/osha_bundle.pdf
var inspectorPDF []byte

// TestInspectFieldTypes examines the OSHA PDF to identify different field types
func TestInspectFieldTypes(t *testing.T) {
	template, err := New(inspectorPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fieldTypes := make(map[string]map[string]bool) // field name -> field type indicators

	for fieldName, fieldRef := range template.fields {
		obj, err := template.getObject(fieldRef.objNum)
		if err != nil {
			continue
		}

		indicators := make(map[string]bool)

		// Check for field type
		if bytes.Contains(obj.content, []byte("/FT /Btn")) {
			indicators["Button"] = true
		}
		if bytes.Contains(obj.content, []byte("/FT /Tx")) {
			indicators["Text"] = true
		}
		if bytes.Contains(obj.content, []byte("/FT /Ch")) {
			indicators["Choice"] = true
		}

		// Check for flags
		if bytes.Contains(obj.content, []byte("/Ff ")) {
			// Extract flags value
			ffIdx := bytes.Index(obj.content, []byte("/Ff "))
			if ffIdx != -1 {
				start := ffIdx + 4
				for start < len(obj.content) && isWhitespace(obj.content[start]) {
					start++
				}
				end := start
				for end < len(obj.content) && isDigit(obj.content[end]) {
					end++
				}
				if end > start {
					flags, _ := parseInt(obj.content[start:end])
					indicators[fmt.Sprintf("Ff=%d", flags)] = true

					// Decode common flags
					if flags&(1<<14) != 0 {
						indicators["Radio"] = true
					}
					if flags&(1<<16) != 0 {
						indicators["Pushbutton"] = true
					}
					if flags&(1<<12) != 0 {
						indicators["Multiline"] = true
					}
				}
			}
		}

		// Check for appearance states (radio/checkbox)
		if bytes.Contains(obj.content, []byte("/AS ")) {
			indicators["HasAppearance"] = true
		}

		// Check for options (choice fields)
		if bytes.Contains(obj.content, []byte("/Opt")) {
			indicators["HasOptions"] = true
		}

		fieldTypes[fieldName] = indicators
	}

	// Group and display by type
	buttonFields := []string{}
	textFields := []string{}
	choiceFields := []string{}
	radioFields := []string{}
	multilineFields := []string{}

	for name, indicators := range fieldTypes {
		if indicators["Button"] {
			buttonFields = append(buttonFields, name)
		}
		if indicators["Radio"] {
			radioFields = append(radioFields, name)
		}
		if indicators["Text"] {
			textFields = append(textFields, name)
		}
		if indicators["Multiline"] {
			multilineFields = append(multilineFields, name)
		}
		if indicators["Choice"] {
			choiceFields = append(choiceFields, name)
		}
	}

	t.Logf("=== Field Type Summary ===")
	t.Logf("Button fields (checkboxes): %d", len(buttonFields))
	if len(buttonFields) > 0 && len(buttonFields) <= 10 {
		for _, name := range buttonFields {
			t.Logf("  - %s: %v", name, fieldTypes[name])
		}
	}

	t.Logf("Radio fields: %d", len(radioFields))
	if len(radioFields) > 0 && len(radioFields) <= 10 {
		for _, name := range radioFields {
			t.Logf("  - %s: %v", name, fieldTypes[name])
		}
	}

	t.Logf("Text fields: %d", len(textFields))
	if len(textFields) > 0 && len(textFields) <= 5 {
		for _, name := range textFields {
			t.Logf("  - %s: %v", name, fieldTypes[name])
		}
	}

	t.Logf("Multiline text fields: %d", len(multilineFields))
	if len(multilineFields) > 0 && len(multilineFields) <= 5 {
		for _, name := range multilineFields {
			t.Logf("  - %s: %v", name, fieldTypes[name])
		}
	}

	t.Logf("Choice fields: %d", len(choiceFields))
	if len(choiceFields) > 0 && len(choiceFields) <= 5 {
		for _, name := range choiceFields {
			t.Logf("  - %s: %v", name, fieldTypes[name])
		}
	}
}
