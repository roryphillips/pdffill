package pdffill

import (
	"bytes"
	_ "embed"
	"sort"
	"testing"

	"github.com/unidoc/unipdf/v3/model"
)

// TestFieldExtractionAgainstUniDoc compares our field extraction against UniDoc
// to verify we're finding all fields correctly.
func TestFieldExtractionAgainstUniDoc(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"template.pdf", testPDF},
		{"form_pdf13.pdf", testPDF13},
		{"form_pdf16.pdf", testPDF16},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get fields using our implementation
			ourFields, err := getOurFields(tc.data)
			if err != nil {
				t.Fatalf("Our implementation error: %v", err)
			}

			// Get fields using UniDoc
			unidocFields, err := getUniDocFields(tc.data)
			if err != nil {
				t.Fatalf("UniDoc error: %v", err)
			}

			t.Logf("Our implementation: %d fields", len(ourFields))
			t.Logf("UniDoc: %d fields", len(unidocFields))

			// Create sets for comparison
			ourSet := make(map[string]bool)
			for _, f := range ourFields {
				ourSet[f] = true
			}

			unidocSet := make(map[string]bool)
			for _, f := range unidocFields {
				unidocSet[f] = true
			}

			// Find fields missing from our implementation
			var missingFromUs []string
			for _, f := range unidocFields {
				if !ourSet[f] {
					missingFromUs = append(missingFromUs, f)
				}
			}

			// Find fields we have that UniDoc doesn't
			var extraInUs []string
			for _, f := range ourFields {
				if !unidocSet[f] {
					extraInUs = append(extraInUs, f)
				}
			}

			if len(missingFromUs) > 0 {
				sort.Strings(missingFromUs)
				t.Errorf("Missing %d fields that UniDoc found:", len(missingFromUs))
				for _, f := range missingFromUs {
					t.Errorf("  - %q", f)
				}
			}

			if len(extraInUs) > 0 {
				sort.Strings(extraInUs)
				t.Logf("Extra %d fields that UniDoc doesn't have (may be valid):", len(extraInUs))
				for _, f := range extraInUs {
					t.Logf("  + %q", f)
				}
			}

			// Summary
			if len(missingFromUs) == 0 {
				t.Logf("SUCCESS: Found all %d fields that UniDoc found", len(unidocFields))
				if len(ourFields) != len(unidocFields) {
					t.Logf("Note: Count mismatch - we have %d extra fields (parent containers or duplicates)", len(ourFields)-len(unidocFields))
				}
			}
		})
	}
}

func getOurFields(data []byte) ([]string, error) {
	template, err := New(data)
	if err != nil {
		return nil, err
	}
	return template.FieldNames(), nil
}

func getUniDocFields(data []byte) ([]string, error) {
	reader, err := model.NewPdfReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	acroForm := reader.AcroForm
	if acroForm == nil {
		return []string{}, nil
	}

	var fields []string
	if acroForm.Fields != nil {
		for _, field := range acroForm.AllFields() {
			name, err := field.FullName()
			if err != nil {
				continue
			}
			if name != "" {
				fields = append(fields, name)
			}
		}
	}

	return fields, nil
}
