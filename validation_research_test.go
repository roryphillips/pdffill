package pdffill

import (
	"bytes"
	_ "embed"
	"testing"
)

//go:embed testdata/osha_bundle.pdf
var validationPDF []byte

// TestResearchValidationConstraints examines PDF fields to identify validation rules
func TestResearchValidationConstraints(t *testing.T) {
	template, err := New(validationPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	constraints := make(map[string]map[string]interface{})

	for fieldName, fieldRef := range template.fields {
		obj, err := template.getObject(fieldRef.objNum)
		if err != nil {
			continue
		}

		fieldConstraints := make(map[string]interface{})

		// Check for MaxLen (maximum character length)
		if bytes.Contains(obj.content, []byte("/MaxLen")) {
			maxLenIdx := bytes.Index(obj.content, []byte("/MaxLen"))
			if maxLenIdx != -1 {
				start := maxLenIdx + 7 // len("/MaxLen")
				for start < len(obj.content) && isWhitespace(obj.content[start]) {
					start++
				}
				end := start
				for end < len(obj.content) && isDigit(obj.content[end]) {
					end++
				}
				if end > start {
					maxLen, _ := parseInt(obj.content[start:end])
					fieldConstraints["MaxLen"] = maxLen
				}
			}
		}

		// Check for required flag (Ff bit 1 = required)
		if bytes.Contains(obj.content, []byte("/Ff ")) {
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
					if flags&1 != 0 {
						fieldConstraints["Required"] = true
					}
					if flags&(1<<13) != 0 {
						fieldConstraints["DoNotSpellCheck"] = true
					}
					if flags&(1<<21) != 0 {
						fieldConstraints["DoNotScroll"] = true
					}
					if flags&(1<<23) != 0 {
						fieldConstraints["Comb"] = true
					}
				}
			}
		}

		// Check for format actions (JavaScript validation/formatting)
		if bytes.Contains(obj.content, []byte("/AA")) {
			fieldConstraints["HasActions"] = true

			// Try to extract the action
			aaIdx := bytes.Index(obj.content, []byte("/AA"))
			if aaIdx != -1 {
				snippet := obj.content[aaIdx:min(aaIdx+500, len(obj.content))]

				// Look for /F (Format) action
				if bytes.Contains(snippet, []byte("/F")) {
					fieldConstraints["HasFormatAction"] = true
				}
				// Look for /V (Validate) action
				if bytes.Contains(snippet, []byte("/V")) {
					fieldConstraints["HasValidateAction"] = true
				}
				// Look for /K (Keystroke) action
				if bytes.Contains(snippet, []byte("/K")) {
					fieldConstraints["HasKeystrokeAction"] = true
				}
			}
		}

		// Check for default value
		if bytes.Contains(obj.content, []byte("/DV")) {
			dvIdx := bytes.Index(obj.content, []byte("/DV"))
			if dvIdx != -1 {
				fieldConstraints["HasDefaultValue"] = true
			}
		}

		if len(fieldConstraints) > 0 {
			constraints[fieldName] = fieldConstraints
		}
	}

	// Report findings
	t.Logf("=== Validation Constraints Found ===")

	requiredFields := 0
	maxLenFields := 0
	actionFields := 0
	defaultFields := 0

	for name, c := range constraints {
		if _, ok := c["Required"]; ok {
			requiredFields++
			if requiredFields <= 5 {
				t.Logf("Required field: %s", name)
			}
		}
		if maxLen, ok := c["MaxLen"]; ok {
			maxLenFields++
			if maxLenFields <= 5 {
				t.Logf("MaxLen field: %s = %v", name, maxLen)
			}
		}
		if _, ok := c["HasActions"]; ok {
			actionFields++
			if actionFields <= 5 {
				t.Logf("Field with actions: %s => %v", name, c)
			}
		}
		if _, ok := c["HasDefaultValue"]; ok {
			defaultFields++
		}
	}

	t.Logf("\nSummary:")
	t.Logf("  Fields with constraints: %d", len(constraints))
	t.Logf("  Required fields: %d", requiredFields)
	t.Logf("  MaxLen fields: %d", maxLenFields)
	t.Logf("  Fields with actions: %d", actionFields)
	t.Logf("  Fields with defaults: %d", defaultFields)
}

// TestExtractFieldFlags examines common flag patterns
func TestExtractFieldFlags(t *testing.T) {
	template, err := New(validationPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	flagCounts := make(map[int]int)
	flagExamples := make(map[int]string)

	for fieldName, fieldRef := range template.fields {
		obj, err := template.getObject(fieldRef.objNum)
		if err != nil {
			continue
		}

		ffIdx := bytes.Index(obj.content, []byte("/Ff "))
		if ffIdx == -1 {
			continue
		}

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
			flagCounts[flags]++
			if _, exists := flagExamples[flags]; !exists {
				flagExamples[flags] = fieldName
			}
		}
	}

	t.Logf("Flag patterns found:")
	for flag, count := range flagCounts {
		t.Logf("  Ff=%d: %d fields (example: %s)", flag, count, flagExamples[flag])

		// Decode common bits
		if flag&1 != 0 {
			t.Logf("    - Bit 0: ReadOnly")
		}
		if flag&2 != 0 {
			t.Logf("    - Bit 1: Required")
		}
		if flag&(1<<12) != 0 {
			t.Logf("    - Bit 12: Multiline")
		}
		if flag&(1<<13) != 0 {
			t.Logf("    - Bit 13: Password")
		}
		if flag&(1<<16) != 0 {
			t.Logf("    - Bit 16: Pushbutton")
		}
		if flag&(1<<23) != 0 {
			t.Logf("    - Bit 23: Comb")
		}
		if flag&(1<<25) != 0 {
			t.Logf("    - Bit 25: CommitOnSelChange")
		}
	}
}
