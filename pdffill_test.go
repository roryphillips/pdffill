package pdffill

import (
	"bytes"
	_ "embed"
	"testing"
)

//go:embed testdata/template.pdf
var testPDF []byte

//go:embed testdata/form_pdf13.pdf
var testPDF13 []byte

//go:embed testdata/form_pdf16.pdf
var testPDF16 []byte

func TestNew(t *testing.T) {
	template, err := New(testPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if template == nil {
		t.Fatal("New() returned nil template")
	}

	if len(template.fields) == 0 {
		t.Error("New() found no fields in PDF")
	}

	t.Logf("Found %d fields in test PDF", len(template.fields))
	for name := range template.fields {
		t.Logf("  Field: %q", name)
	}
}

func TestTemplate_FieldNames(t *testing.T) {
	template, err := New(testPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	names := template.FieldNames()
	if len(names) == 0 {
		t.Error("FieldNames() returned empty list")
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, name := range names {
		if seen[name] {
			t.Errorf("FieldNames() contains duplicate: %q", name)
		}
		seen[name] = true
	}
}

func TestTemplate_Fill(t *testing.T) {
	template, err := New(testPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Get first field name for testing
	fields := template.FieldNames()
	if len(fields) == 0 {
		t.Skip("No fields found in PDF")
	}

	testCases := []struct {
		name     string
		formData map[string]string
		wantErr  bool
	}{
		{
			name: "valid single field",
			formData: map[string]string{
				fields[0]: "Test Value",
			},
			wantErr: false,
		},
		{
			name:     "empty form data",
			formData: map[string]string{},
			wantErr:  true,
		},
		{
			name: "nonexistent field",
			formData: map[string]string{
				"nonexistent_field_xyz": "value",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := template.Fill(tc.formData)
			if (err != nil) != tc.wantErr {
				t.Errorf("Fill() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			if !tc.wantErr {
				if len(result) == 0 {
					t.Error("Fill() returned empty result")
				}
				if len(result) < 100 {
					t.Error("Fill() returned suspiciously small PDF")
				}
				// Check PDF header
				if string(result[:5]) != "%PDF-" {
					t.Error("Fill() result doesn't start with PDF header")
				}
			}
		})
	}
}

func TestTemplate_FillMultipleFields(t *testing.T) {
	template, err := New(testPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fields := template.FieldNames()
	if len(fields) < 2 {
		t.Skip("Need at least 2 fields for this test")
	}

	formData := map[string]string{
		fields[0]: "First Value",
		fields[1]: "Second Value",
	}

	result, err := template.Fill(formData)
	if err != nil {
		t.Fatalf("Fill() error = %v", err)
	}

	if len(result) == 0 {
		t.Error("Fill() returned empty result")
	}

	t.Logf("Filled PDF size: %d bytes (original: %d bytes)", len(result), len(testPDF))
}

func TestTemplate_Reusability(t *testing.T) {
	template, err := New(testPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fields := template.FieldNames()
	if len(fields) == 0 {
		t.Skip("No fields found")
	}

	// Fill multiple times with different data
	for i := 0; i < 3; i++ {
		formData := map[string]string{
			fields[0]: "Value " + string(rune('A'+i)),
		}

		result, err := template.Fill(formData)
		if err != nil {
			t.Errorf("Fill() iteration %d error = %v", i, err)
		}

		if len(result) == 0 {
			t.Errorf("Fill() iteration %d returned empty result", i)
		}
	}
}

func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := New(testPDF)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFill(b *testing.B) {
	template, err := New(testPDF)
	if err != nil {
		b.Fatal(err)
	}

	fields := template.FieldNames()
	if len(fields) == 0 {
		b.Skip("No fields found")
	}

	formData := map[string]string{
		fields[0]: "Benchmark Value",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := template.Fill(formData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFillMultiple(b *testing.B) {
	template, err := New(testPDF)
	if err != nil {
		b.Fatal(err)
	}

	fields := template.FieldNames()
	if len(fields) < 5 {
		b.Skip("Need at least 5 fields")
	}

	formData := make(map[string]string)
	for i := 0; i < min(5, len(fields)); i++ {
		formData[fields[i]] = "Value"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := template.Fill(formData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestPDFVersionSupport tests that both PDF 1.3 (traditional xref) and
// PDF 1.6 (XRef streams with compressed objects) can be parsed and filled.
func TestPDFVersionSupport(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		pdfVersion   string
		minFields    int
		description  string
	}{
		{
			name:        "PDF 1.3 (traditional xref)",
			data:        testPDF13,
			pdfVersion:  "1.3",
			minFields:   100,
			description: "Traditional cross-reference table format",
		},
		{
			name:        "PDF 1.6 (XRef stream)",
			data:        testPDF16,
			pdfVersion:  "1.6",
			minFields:   40,
			description: "XRef stream with compressed object streams",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify PDF version in header
			if !bytes.HasPrefix(tt.data, []byte("%PDF-"+tt.pdfVersion)) {
				t.Logf("Note: PDF header doesn't match expected version %s", tt.pdfVersion)
			}

			// Parse the PDF
			template, err := New(tt.data)
			if err != nil {
				t.Fatalf("New() error for %s: %v", tt.description, err)
			}

			// Check field count
			fields := template.FieldNames()
			t.Logf("Found %d fields in %s", len(fields), tt.name)

			if len(fields) < tt.minFields {
				t.Errorf("Expected at least %d fields, got %d", tt.minFields, len(fields))
			}

			// Test filling a field
			if len(fields) > 0 {
				formData := map[string]string{
					fields[0]: "Test Value",
				}

				filled, err := template.Fill(formData)
				if err != nil {
					t.Fatalf("Fill() error: %v", err)
				}

				if len(filled) == 0 {
					t.Error("Fill() returned empty PDF")
				}

				// Verify output is valid PDF
				if !bytes.HasPrefix(filled, []byte("%PDF-")) {
					t.Error("Filled PDF doesn't have valid header")
				}

				t.Logf("Successfully filled %s: %d bytes", tt.name, len(filled))
			}

			// Test field info retrieval
			if len(fields) > 0 {
				info, err := template.GetFieldInfo(fields[0])
				if err != nil {
					t.Errorf("GetFieldInfo() error: %v", err)
				} else {
					t.Logf("First field: %s (type: %s)", info.Name, info.Type)
				}
			}
		})
	}
}

// TestPDF16XRefStream specifically tests PDF 1.6 XRef stream handling.
func TestPDF16XRefStream(t *testing.T) {
	// Verify the PDF header
	if !bytes.Contains(testPDF16[:20], []byte("1.6")) {
		t.Skip("Test PDF is not version 1.6")
	}

	template, err := New(testPDF16)
	if err != nil {
		t.Fatalf("Failed to parse PDF 1.6: %v", err)
	}

	fields := template.FieldNames()
	t.Logf("PDF 1.6 has %d fields", len(fields))

	// Fill multiple fields
	formData := make(map[string]string)
	for i, field := range fields {
		if i >= 5 {
			break
		}
		formData[field] = "Test " + field
	}

	filled, err := template.Fill(formData)
	if err != nil {
		t.Fatalf("Fill() error: %v", err)
	}

	t.Logf("Filled PDF 1.6: %d bytes (original: %d bytes)", len(filled), len(testPDF16))

	// Verify the filled PDF is valid
	if !bytes.HasPrefix(filled, []byte("%PDF-")) {
		t.Error("Output is not a valid PDF")
	}

	if !bytes.HasSuffix(bytes.TrimSpace(filled), []byte("%%EOF")) {
		t.Error("Output doesn't end with EOF marker")
	}
}

// TestHierarchicalFieldParsing tests that fields nested in /Kids arrays are correctly parsed.
// This is a regression test for the issue where 78% of fields were missing because
// the parser only looked at top-level /Fields and didn't recursively follow /Kids.
func TestHierarchicalFieldParsing(t *testing.T) {
	template, err := New(testPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fields := template.FieldNames()

	// These fields were missing before the fix because they're stored in
	// compressed object streams and accessed via /Kids arrays
	requiredFields := []string{
		"301 Full name",
		"301 Address Street",
		"301 Case Number",
		"301 Date of Injury or Illness",
		"301 Phone",
		"301 Completed by",
	}

	fieldSet := make(map[string]bool)
	for _, name := range fields {
		fieldSet[name] = true
	}

	for _, required := range requiredFields {
		if !fieldSet[required] {
			t.Errorf("Required field %q not found (likely /Kids parsing failed)", required)
		}
	}

	// The template should have at least 200 fields (203 expected)
	// Previously only ~45 were found due to missing /Kids support
	if len(fields) < 200 {
		t.Errorf("Expected at least 200 fields, got %d (recursive /Kids parsing may have failed)", len(fields))
	}

	t.Logf("Found %d fields (expected ~203)", len(fields))
}

// TestCompressedObjectStreamParsing verifies that objects stored in compressed
// object streams (PDF 1.5+ /Type /ObjStm) can be correctly retrieved.
func TestCompressedObjectStreamParsing(t *testing.T) {
	// PDF 1.6 uses XRef streams and may have compressed object streams
	template, err := New(testPDF16)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fields := template.FieldNames()
	if len(fields) == 0 {
		t.Error("No fields found in PDF 1.6 - object stream parsing may have failed")
	}

	t.Logf("PDF 1.6 has %d fields", len(fields))

	// Verify we can fill fields (proves objects were properly retrieved)
	if len(fields) > 0 {
		formData := map[string]string{
			fields[0]: "Test Value from Compressed Object Stream",
		}
		filled, err := template.Fill(formData)
		if err != nil {
			t.Fatalf("Fill() error: %v", err)
		}
		if len(filled) == 0 {
			t.Error("Fill() returned empty result")
		}
	}
}

// TestObjectStreamFieldFilling verifies that fields stored in compressed object
// streams can be filled and the value appears in the output PDF.
// This is a regression test for the issue where filling failed for objects
// that were stored in ObjStm because rebuildPDF couldn't find them in the PDF body.
func TestObjectStreamFieldFilling(t *testing.T) {
	template, err := New(testPDF16)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fields := template.FieldNames()
	if len(fields) == 0 {
		t.Skip("No fields in PDF 1.6")
	}

	// Use a unique test value that we can search for in the output
	testValue := "OBJSTM_TEST_VALUE_12345"
	formData := map[string]string{
		fields[0]: testValue,
	}

	filled, err := template.Fill(formData)
	if err != nil {
		t.Fatalf("Fill() error: %v", err)
	}

	// The test value must appear in the output PDF
	if !bytes.Contains(filled, []byte(testValue)) {
		t.Errorf("Filled value %q not found in output PDF - object stream filling may have failed", testValue)
	}

	// Verify the output is a valid PDF
	if !bytes.HasPrefix(filled, []byte("%PDF-")) {
		t.Error("Output doesn't have valid PDF header")
	}

	t.Logf("Successfully filled field from object stream, output size: %d bytes", len(filled))
}

// TestFieldNameExtraction ensures field names are correctly extracted,
// including hierarchical names built from parent.child relationships.
func TestFieldNameExtraction(t *testing.T) {
	template, err := New(testPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fields := template.FieldNames()
	if len(fields) == 0 {
		t.Fatal("No fields found")
	}

	// Check that field names are non-empty and don't have issues
	for _, name := range fields {
		if name == "" {
			t.Error("Found empty field name")
		}
		// Field names shouldn't start with a dot (malformed hierarchical name)
		if len(name) > 0 && name[0] == '.' {
			t.Errorf("Field name starts with dot (malformed): %q", name)
		}
		// Field names shouldn't end with a dot
		if len(name) > 0 && name[len(name)-1] == '.' {
			t.Errorf("Field name ends with dot (malformed): %q", name)
		}
	}
}
