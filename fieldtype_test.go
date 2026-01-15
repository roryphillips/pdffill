package pdffill

import (
	"bytes"
	_ "embed"
	"os"
	"testing"
)

//go:embed testdata/template.pdf
var fieldTypePDF []byte

func TestFieldType_Checkbox(t *testing.T) {
	template, err := New(fieldTypePDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Find a checkbox field (Reset buttons are checkboxes)
	checkboxField := "Reset 1"

	// Test checking the box
	filled, err := template.Fill(map[string]string{
		checkboxField: "Yes",
	})
	if err != nil {
		t.Fatalf("Fill() error = %v", err)
	}

	if len(filled) == 0 {
		t.Error("Fill() returned empty PDF")
	}

	// Verify the checkbox is checked
	if !bytes.Contains(filled, []byte("/V /Yes")) {
		t.Error("Checkbox should be set to /Yes")
	}

	t.Logf("Checkbox filled successfully: %d bytes", len(filled))
}

func TestFieldType_CheckboxVariations(t *testing.T) {
	template, err := New(fieldTypePDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	checkboxField := "Reset 2"

	testCases := []struct {
		name          string
		value         string
		expectedState string
	}{
		{"Checked with On", "On", "/Yes"},
		{"Checked with true", "true", "/Yes"},
		{"Checked with 1", "1", "/Yes"},
		{"Checked with X", "X", "/Yes"},
		{"Unchecked with Off", "Off", "/Off"},
		{"Unchecked with false", "false", "/Off"},
		{"Unchecked with 0", "0", "/Off"},
		{"Unchecked with empty", "", "/Off"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filled, err := template.Fill(map[string]string{
				checkboxField: tc.value,
			})
			if err != nil {
				t.Fatalf("Fill() error = %v", err)
			}

			if !bytes.Contains(filled, []byte("/V "+tc.expectedState)) &&
				!bytes.Contains(filled, []byte("/V"+tc.expectedState)) {
				t.Errorf("Expected checkbox state %s, not found in PDF", tc.expectedState)
			}
		})
	}
}

func TestFieldType_RadioButton(t *testing.T) {
	template, err := New(fieldTypePDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Find a radio button group
	radioField := "301 Gender"

	// Test setting a radio value
	filled, err := template.Fill(map[string]string{
		radioField: "Male",
	})
	if err != nil {
		t.Fatalf("Fill() error = %v", err)
	}

	if len(filled) == 0 {
		t.Error("Fill() returned empty PDF")
	}

	t.Logf("Radio button filled successfully: %d bytes", len(filled))
}

func TestFieldType_MultilineText(t *testing.T) {
	template, err := New(fieldTypePDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// The multiline field we found
	multilineField := "301 What Happened"

	longText := `This is a multiline text value.
It spans multiple lines.
And should be properly encoded in the PDF.`

	filled, err := template.Fill(map[string]string{
		multilineField: longText,
	})
	if err != nil {
		t.Fatalf("Fill() error = %v", err)
	}

	if len(filled) == 0 {
		t.Error("Fill() returned empty PDF")
	}

	// Verify the text is in the PDF (with escaped newlines)
	if !bytes.Contains(filled, []byte("multiline")) {
		t.Error("Multiline text should be in PDF")
	}

	t.Logf("Multiline text filled successfully: %d bytes", len(filled))
}

func TestFieldType_MixedFields(t *testing.T) {
	template, err := New(fieldTypePDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Mix different field types
	formData := map[string]string{
		// Text fields
		"Employee's Name 1":               "John Doe",
		"Log of Injury/Illness Year":      "2026",
		"Summary of Injury/Illness City":  "Springfield",
		"Summary of Injury/Illness State": "IL",
		"Summary of Injury/Illness Phone": "555-1234",

		// Checkbox
		"Reset 1": "Yes",

		// Radio buttons
		"301 Gender": "Male",
		"301 ER":     "Yes",

		// Multiline
		"301 What Happened": "Employee slipped on wet floor in warehouse.",
	}

	filled, err := template.Fill(formData)
	if err != nil {
		t.Fatalf("Fill() error = %v", err)
	}

	if len(filled) == 0 {
		t.Error("Fill() returned empty PDF")
	}

	// Verify various field types are present
	if !bytes.Contains(filled, []byte("John Doe")) {
		t.Error("Text field value not found")
	}
	if !bytes.Contains(filled, []byte("Springfield")) {
		t.Error("Another text field value not found")
	}

	t.Logf("Mixed field types filled successfully: %d bytes", len(filled))

	// Write for manual inspection
	os.WriteFile("mixed_fields_output.pdf", filled, 0644)
	defer os.Remove("mixed_fields_output.pdf")
}

func TestGetFieldInfo(t *testing.T) {
	template, err := New(fieldTypePDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	testCases := []struct {
		fieldName    string
		expectedType FieldType
	}{
		{"Employee's Name 1", FieldTypeText},
		{"301 What Happened", FieldTypeMultilineText},
		{"Reset 1", FieldTypeCheckbox},
		{"301 Gender", FieldTypeRadio},
	}

	for _, tc := range testCases {
		t.Run(tc.fieldName, func(t *testing.T) {
			info, err := template.GetFieldInfo(tc.fieldName)
			if err != nil {
				t.Fatalf("GetFieldInfo() error = %v", err)
			}

			if info.Type != tc.expectedType {
				t.Errorf("Expected type %s, got %s", tc.expectedType, info.Type)
			}

			t.Logf("Field: %s, Type: %s, ObjNum: %d", info.Name, info.Type, info.ObjNum)

			if tc.expectedType == FieldTypeRadio && !info.HasKids {
				t.Error("Radio button should have kids")
			}
			if tc.expectedType == FieldTypeRadio && len(info.KidRefs) == 0 {
				t.Error("Radio button should have kid references")
			}
		})
	}
}

func TestFieldType_NumberField(t *testing.T) {
	template, err := New(fieldTypePDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Use a field that likely expects numbers
	numberField := "Number of days injured or ill away from work 1"

	filled, err := template.Fill(map[string]string{
		numberField: "42",
	})
	if err != nil {
		t.Fatalf("Fill() error = %v", err)
	}

	if !bytes.Contains(filled, []byte("42")) {
		t.Error("Number value not found in PDF")
	}

	t.Logf("Number field filled successfully")
}

func BenchmarkFill_MixedFieldTypes(b *testing.B) {
	template, err := New(fieldTypePDF)
	if err != nil {
		b.Fatal(err)
	}

	formData := map[string]string{
		"Employee's Name 1":              "John Doe",
		"Log of Injury/Illness Year":     "2026",
		"Summary of Injury/Illness City": "Springfield",
		"Reset 1":                        "Yes",
		"301 Gender":                     "Male",
		"301 What Happened":              "Incident occurred in warehouse.",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := template.Fill(formData)
		if err != nil {
			b.Fatal(err)
		}
	}
}
