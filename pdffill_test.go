package pdffill

import (
	_ "embed"
	"testing"
)

//go:embed testdata/osha_bundle.pdf
var oshaPDF []byte

func TestNew(t *testing.T) {
	template, err := New(oshaPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if template == nil {
		t.Fatal("New() returned nil template")
	}

	if len(template.fields) == 0 {
		t.Error("New() found no fields in PDF")
	}

	t.Logf("Found %d fields in OSHA PDF", len(template.fields))
	for name := range template.fields {
		t.Logf("  Field: %q", name)
	}
}

func TestTemplate_FieldNames(t *testing.T) {
	template, err := New(oshaPDF)
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
	template, err := New(oshaPDF)
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
	template, err := New(oshaPDF)
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

	t.Logf("Filled PDF size: %d bytes (original: %d bytes)", len(result), len(oshaPDF))
}

func TestTemplate_Reusability(t *testing.T) {
	template, err := New(oshaPDF)
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
		_, err := New(oshaPDF)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFill(b *testing.B) {
	template, err := New(oshaPDF)
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
	template, err := New(oshaPDF)
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
