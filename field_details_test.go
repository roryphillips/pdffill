package pdffill

import (
	"bytes"
	_ "embed"
	"testing"
)

//go:embed testdata/osha_bundle.pdf
var detailsPDF []byte

func TestButtonFieldDetails(t *testing.T) {
	template, err := New(detailsPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Find button fields and inspect them
	buttonCount := 0
	radioCount := 0

	for fieldName, fieldRef := range template.fields {
		obj, err := template.getObject(fieldRef.objNum)
		if err != nil {
			continue
		}

		// Only look at button fields
		if !bytes.Contains(obj.content, []byte("/FT /Btn")) {
			continue
		}

		buttonCount++

		// Check if it has /Kids (radio button group)
		hasKids := bytes.Contains(obj.content, []byte("/Kids"))

		if hasKids {
			radioCount++
			if radioCount <= 5 {
				t.Logf("Radio button: %s", fieldName)
				t.Logf("  Content sample: %s", obj.content[:min(300, len(obj.content))])
			}
		} else {
			if buttonCount-radioCount <= 5 {
				t.Logf("Checkbox: %s", fieldName)
				t.Logf("  Content sample: %s", obj.content[:min(300, len(obj.content))])
			}
		}
	}

	t.Logf("\nTotal buttons: %d, Radio groups: %d, Checkboxes: %d",
		buttonCount, radioCount, buttonCount-radioCount)
}

func TestCalculatedFields(t *testing.T) {
	template, err := New(detailsPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Look for calculated fields (they have /AA or /Calculate entries)
	calculatedCount := 0

	for fieldName, fieldRef := range template.fields {
		obj, err := template.getObject(fieldRef.objNum)
		if err != nil {
			continue
		}

		hasCalculation := bytes.Contains(obj.content, []byte("/AA")) ||
			bytes.Contains(obj.content, []byte("/Calculate"))

		if hasCalculation {
			calculatedCount++
			if calculatedCount <= 5 {
				t.Logf("Calculated field: %s", fieldName)
				// Find the calculation script
				aaIdx := bytes.Index(obj.content, []byte("/AA"))
				if aaIdx != -1 {
					snippet := obj.content[aaIdx:min(aaIdx+200, len(obj.content))]
					t.Logf("  Script snippet: %s", snippet)
				}
			}
		}
	}

	t.Logf("\nTotal calculated fields: %d", calculatedCount)
}
