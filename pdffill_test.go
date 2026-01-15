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
