package pdffill

import (
	_ "embed"
	"strings"
	"testing"
)

//go:embed testdata/osha_bundle.pdf
var validationTestPDF []byte

func TestValidation_RequiredFields(t *testing.T) {
	template, err := New(validationTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	requiredFields := template.GetRequiredFields()
	t.Logf("Found %d required fields", len(requiredFields))

	if len(requiredFields) == 0 {
		t.Skip("No required fields in this PDF")
	}

	// Try to fill without providing required field
	opts := StrictFillOptions()
	opts.Validation = ValidationBasic

	formData := map[string]string{
		"Employee's Name 1": "John Doe", // Not a required field
	}

	_, err = template.FillWithOptions(formData, opts)
	if err == nil {
		t.Error("Expected validation error for missing required fields")
	}

	// Check error contains required field information
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("Error should mention required fields: %v", err)
	}

	t.Logf("Validation correctly caught missing required fields: %v", err)
}

func TestValidation_MaxLen(t *testing.T) {
	template, err := New(validationTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// NAICS field has MaxLen of 6
	naicsField := "Summary of Injury/Illness NAICS"

	// Test with value that's too long
	opts := StrictFillOptions()

	formData := map[string]string{
		naicsField: "1234567890", // Too long (should be max 6)
	}

	_, err = template.FillWithOptions(formData, opts)
	if err == nil {
		t.Error("Expected validation error for MaxLen violation")
	}

	if !strings.Contains(err.Error(), "max-length") {
		t.Errorf("Error should mention max-length: %v", err)
	}

	t.Logf("Validation correctly caught MaxLen violation: %v", err)
}

func TestValidation_MaxLenTruncate(t *testing.T) {
	template, err := New(validationTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	naicsField := "Summary of Injury/Illness NAICS"

	// Test with truncation enabled
	opts := StrictFillOptions()
	opts.TruncateMaxLen = true

	formData := map[string]string{
		naicsField: "1234567890", // Will be truncated to "123456"
	}

	filled, err := template.FillWithOptions(formData, opts)
	if err != nil {
		t.Fatalf("FillWithOptions() with truncation error = %v", err)
	}

	if len(filled) == 0 {
		t.Error("Fill should succeed with truncation")
	}

	t.Logf("Successfully filled with truncation: %d bytes", len(filled))
}

func TestValidation_ReadOnly(t *testing.T) {
	template, err := New(validationTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Find a read-only field (calculated fields are usually read-only)
	readOnlyField := "All other Total" // This should be read-only (Ff=1)

	opts := StrictFillOptions()
	opts.SkipReadOnly = false

	formData := map[string]string{
		readOnlyField: "100", // Try to set read-only field
	}

	_, err = template.FillWithOptions(formData, opts)
	if err == nil {
		t.Error("Expected validation error for read-only field")
	}

	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("Error should mention read-only: %v", err)
	}

	t.Logf("Validation correctly caught read-only violation: %v", err)
}

func TestValidation_SkipReadOnly(t *testing.T) {
	template, err := New(validationTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	readOnlyField := "All other Total"

	opts := StrictFillOptions()
	opts.SkipReadOnly = true // Should skip instead of error

	formData := map[string]string{
		readOnlyField:      "100", // Will be skipped
		"Employee's Name 1": "John Doe",
	}

	filled, err := template.FillWithOptions(formData, opts)
	if err != nil {
		t.Fatalf("FillWithOptions() with SkipReadOnly should succeed: %v", err)
	}

	if len(filled) == 0 {
		t.Error("Fill should succeed when skipping read-only")
	}

	t.Logf("Successfully filled with SkipReadOnly: %d bytes", len(filled))
}

func TestValidation_NonExistentField(t *testing.T) {
	template, err := New(validationTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	opts := StrictFillOptions()

	formData := map[string]string{
		"NonExistentField": "value",
	}

	_, err = template.FillWithOptions(formData, opts)
	if err == nil {
		t.Error("Expected validation error for non-existent field")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Error should mention field doesn't exist: %v", err)
	}
}

func TestValidation_NoValidation(t *testing.T) {
	template, err := New(validationTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// With ValidationNone, everything should pass (fast path)
	opts := DefaultFillOptions()

	formData := map[string]string{
		"Employee's Name 1": "John Doe",
	}

	filled, err := template.FillWithOptions(formData, opts)
	if err != nil {
		t.Fatalf("FillWithOptions() with no validation error = %v", err)
	}

	if len(filled) == 0 {
		t.Error("Fill should succeed")
	}
}

func TestValidation_MultipleErrors(t *testing.T) {
	template, err := New(validationTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	opts := StrictFillOptions()

	formData := map[string]string{
		"Summary of Injury/Illness NAICS": "1234567890",  // Too long
		"All other Total":                 "100",          // Read-only
		"NonExistentField":                "value",        // Doesn't exist
	}

	_, err = template.FillWithOptions(formData, opts)
	if err == nil {
		t.Error("Expected multiple validation errors")
	}

	// Check that error contains multiple violations
	errStr := err.Error()
	if !strings.Contains(errStr, "validation error") {
		t.Errorf("Should have validation errors: %v", err)
	}

	t.Logf("Multiple validation errors caught:\n%v", err)
}

func TestGetFieldConstraints(t *testing.T) {
	template, err := New(validationTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	testCases := []struct {
		fieldName       string
		expectRequired  bool
		expectReadOnly  bool
		expectMaxLen    int
	}{
		{"All other Total", false, true, 0},
		{"Summary of Injury/Illness NAICS", false, false, 6},
		{"Employee's Name 1", false, false, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.fieldName, func(t *testing.T) {
			constraints, err := template.GetFieldConstraints(tc.fieldName)
			if err != nil {
				t.Fatalf("GetFieldConstraints() error = %v", err)
			}

			if constraints.Required != tc.expectRequired {
				t.Errorf("Expected Required=%v, got %v", tc.expectRequired, constraints.Required)
			}

			if constraints.ReadOnly != tc.expectReadOnly {
				t.Errorf("Expected ReadOnly=%v, got %v", tc.expectReadOnly, constraints.ReadOnly)
			}

			if constraints.MaxLen != tc.expectMaxLen {
				t.Errorf("Expected MaxLen=%d, got %d", tc.expectMaxLen, constraints.MaxLen)
			}

			t.Logf("Field: %s, Required: %v, ReadOnly: %v, MaxLen: %d",
				tc.fieldName, constraints.Required, constraints.ReadOnly, constraints.MaxLen)
		})
	}
}

func TestValidateOnly(t *testing.T) {
	template, err := New(validationTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Test ValidateOnly without actually filling
	opts := StrictFillOptions()

	formData := map[string]string{
		"Summary of Injury/Illness NAICS": "123456", // Valid
		"Employee's Name 1":               "John Doe",
	}

	err = template.ValidateOnly(formData, opts)
	if err != nil {
		t.Errorf("ValidateOnly() should pass: %v", err)
	}

	// Test with invalid data
	formData["Summary of Injury/Illness NAICS"] = "1234567890" // Too long

	err = template.ValidateOnly(formData, opts)
	if err == nil {
		t.Error("ValidateOnly() should catch validation errors")
	}
}

func BenchmarkFillWithValidation(b *testing.B) {
	template, err := New(validationTestPDF)
	if err != nil {
		b.Fatal(err)
	}

	opts := StrictFillOptions()
	formData := map[string]string{
		"Employee's Name 1": "John Doe",
		"Log of Injury/Illness Year": "2026",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := template.FillWithOptions(formData, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFillNoValidation(b *testing.B) {
	template, err := New(validationTestPDF)
	if err != nil {
		b.Fatal(err)
	}

	formData := map[string]string{
		"Employee's Name 1": "John Doe",
		"Log of Injury/Illness Year": "2026",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := template.Fill(formData)
		if err != nil {
			b.Fatal(err)
		}
	}
}
