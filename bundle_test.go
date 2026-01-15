package pdffill

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"testing"
)

//go:embed testdata/template.pdf
var bundleTestPDF []byte

//go:embed testdata/form_pdf13.pdf
var bundlePDF13 []byte

//go:embed testdata/form_pdf16.pdf
var bundlePDF16 []byte

func TestBundler_FillMultiple(t *testing.T) {
	template, err := New(bundleTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fields := template.FieldNames()
	if len(fields) == 0 {
		t.Skip("No fields found")
	}

	bundler := NewBundler()

	// Create multiple filled forms from the same template
	formSets := []map[string]string{
		{fields[0]: "Value 1A", fields[1]: "Value 1B"},
		{fields[0]: "Value 2A", fields[1]: "Value 2B"},
		{fields[0]: "Value 3A", fields[1]: "Value 3B"},
	}

	err = bundler.FillMultiple(template, formSets...)
	if err != nil {
		t.Fatalf("FillMultiple() error = %v", err)
	}

	if len(bundler.forms) != 3 {
		t.Errorf("Expected 3 forms, got %d", len(bundler.forms))
	}
}

func TestBundler_Bundle(t *testing.T) {
	template, err := New(bundleTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fields := template.FieldNames()
	if len(fields) < 5 {
		t.Skip("Need at least 5 fields")
	}

	bundler := NewBundler()

	// Add multiple filled forms
	formSets := []map[string]string{
		{
			fields[0]: "Company A",
			fields[1]: "2026",
			fields[2]: "Springfield",
		},
		{
			fields[0]: "Company B",
			fields[1]: "2026",
			fields[2]: "Chicago",
		},
	}

	err = bundler.FillMultiple(template, formSets...)
	if err != nil {
		t.Fatalf("FillMultiple() error = %v", err)
	}

	// Bundle into single PDF
	bundledPDF, err := bundler.Bundle()
	if err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}

	// Verify output
	if len(bundledPDF) == 0 {
		t.Error("Bundle() returned empty PDF")
	}

	if string(bundledPDF[:5]) != "%PDF-" {
		t.Error("Bundle() result doesn't start with PDF header")
	}

	// Should be larger than a single form
	singleFilled, _ := template.Fill(formSets[0])
	if len(bundledPDF) < len(singleFilled) {
		t.Errorf("Bundled PDF (%d bytes) is smaller than single filled form (%d bytes)",
			len(bundledPDF), len(singleFilled))
	}

	t.Logf("Bundled PDF: %d bytes (single form: %d bytes)", len(bundledPDF), len(singleFilled))
}

func TestBundler_MultiplTemplates(t *testing.T) {
	template1, err := New(bundleTestPDF)
	if err != nil {
		t.Fatalf("New() template1 error = %v", err)
	}

	template2, err := New(bundleTestPDF)
	if err != nil {
		t.Fatalf("New() template2 error = %v", err)
	}

	fields := template1.FieldNames()
	if len(fields) < 3 {
		t.Skip("Need at least 3 fields")
	}

	bundler := NewBundler()

	// Add forms from template1
	err = bundler.FillMultiple(template1,
		map[string]string{fields[0]: "T1-Form1"},
		map[string]string{fields[0]: "T1-Form2"},
	)
	if err != nil {
		t.Fatalf("FillMultiple() template1 error = %v", err)
	}

	// Add forms from template2
	err = bundler.FillMultiple(template2,
		map[string]string{fields[0]: "T2-Form1"},
	)
	if err != nil {
		t.Fatalf("FillMultiple() template2 error = %v", err)
	}

	// Bundle
	bundledPDF, err := bundler.Bundle()
	if err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}

	if len(bundledPDF) == 0 {
		t.Error("Bundle() returned empty PDF")
	}

	t.Logf("Multi-template bundled PDF: %d bytes", len(bundledPDF))
}

func TestBundler_EmptyBundle(t *testing.T) {
	bundler := NewBundler()

	_, err := bundler.Bundle()
	if err == nil {
		t.Error("Bundle() should error on empty bundler")
	}
}

func TestBundler_FieldNameDeduplication(t *testing.T) {
	template, err := New(bundleTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fields := template.FieldNames()
	if len(fields) == 0 {
		t.Skip("No fields found")
	}

	// Fill same template twice with same field
	form1, err := template.fillWithIndex(
		map[string]string{fields[0]: "Value1"},
		0,
	)
	if err != nil {
		t.Fatalf("fillWithIndex(0) error = %v", err)
	}

	form2, err := template.fillWithIndex(
		map[string]string{fields[0]: "Value2"},
		1,
	)
	if err != nil {
		t.Fatalf("fillWithIndex(1) error = %v", err)
	}

	// Check that field names are prefixed differently
	hasPrefix0 := containsString(form1, []byte("f0_"))
	hasPrefix1 := containsString(form2, []byte("f1_"))

	if !hasPrefix0 {
		t.Error("form1 should contain 'f0_' prefix")
	}
	if !hasPrefix1 {
		t.Error("form2 should contain 'f1_' prefix")
	}
}

func TestBundler_WriteToFile(t *testing.T) {
	template, err := New(bundleTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fields := template.FieldNames()
	if len(fields) < 2 {
		t.Skip("Need at least 2 fields")
	}

	bundler := NewBundler()

	err = bundler.FillMultiple(template,
		map[string]string{fields[0]: "Test 1", fields[1]: "2026"},
		map[string]string{fields[0]: "Test 2", fields[1]: "2027"},
		map[string]string{fields[0]: "Test 3", fields[1]: "2028"},
	)
	if err != nil {
		t.Fatalf("FillMultiple() error = %v", err)
	}

	bundledPDF, err := bundler.Bundle()
	if err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}

	// Write to file for manual inspection
	outputPath := "bundled_output.pdf"
	if err := os.WriteFile(outputPath, bundledPDF, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Clean up
	defer os.Remove(outputPath)

	// Verify file was created
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	if info.Size() == 0 {
		t.Error("Output file is empty")
	}

	t.Logf("Bundled PDF written to %s: %d bytes", outputPath, info.Size())
}

func BenchmarkBundler_FillMultiple(b *testing.B) {
	template, err := New(bundleTestPDF)
	if err != nil {
		b.Fatal(err)
	}

	fields := template.FieldNames()
	if len(fields) == 0 {
		b.Skip("No fields found")
	}

	formSets := []map[string]string{
		{fields[0]: "Value 1"},
		{fields[0]: "Value 2"},
		{fields[0]: "Value 3"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bundler := NewBundler()
		bundler.FillMultiple(template, formSets...)
	}
}

func BenchmarkBundler_Bundle(b *testing.B) {
	template, err := New(bundleTestPDF)
	if err != nil {
		b.Fatal(err)
	}

	fields := template.FieldNames()
	if len(fields) == 0 {
		b.Skip("No fields found")
	}

	formSets := []map[string]string{
		{fields[0]: "Value 1"},
		{fields[0]: "Value 2"},
		{fields[0]: "Value 3"},
	}

	bundler := NewBundler()
	bundler.FillMultiple(template, formSets...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := bundler.Bundle()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBundler_FullWorkflow(b *testing.B) {
	template, err := New(bundleTestPDF)
	if err != nil {
		b.Fatal(err)
	}

	fields := template.FieldNames()
	if len(fields) == 0 {
		b.Skip("No fields found")
	}

	formSets := []map[string]string{
		{fields[0]: "Value 1"},
		{fields[0]: "Value 2"},
		{fields[0]: "Value 3"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bundler := NewBundler()
		bundler.FillMultiple(template, formSets...)
		_, err := bundler.Bundle()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestBundler_Compression(t *testing.T) {
	template, err := New(bundleTestPDF)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	fields := template.FieldNames()
	if len(fields) < 5 {
		t.Skip("Need at least 5 fields")
	}

	formSets := []map[string]string{
		{fields[0]: "Employee One", fields[1]: "2026", fields[2]: "Test data"},
		{fields[0]: "Employee Two", fields[1]: "2026", fields[2]: "More data"},
		{fields[0]: "Employee Three", fields[1]: "2026", fields[2]: "Even more"},
	}

	// Bundle without compression
	bundlerNoCompress := NewBundler()
	bundlerNoCompress.FillMultiple(template, formSets...)
	uncompressed, err := bundlerNoCompress.Bundle()
	if err != nil {
		t.Fatalf("Bundle() without compression error = %v", err)
	}

	// Bundle with compression
	bundlerCompress := NewBundler()
	bundlerCompress.EnableCompression()
	bundlerCompress.FillMultiple(template, formSets...)
	compressed, err := bundlerCompress.Bundle()
	if err != nil {
		t.Fatalf("Bundle() with compression error = %v", err)
	}

	// Log sizes
	t.Logf("Uncompressed size: %d bytes", len(uncompressed))
	t.Logf("Compressed size: %d bytes", len(compressed))

	savings := float64(len(uncompressed)-len(compressed)) / float64(len(uncompressed)) * 100
	t.Logf("Compression savings: %.1f%%", savings)

	// Compressed should be smaller (or at least not larger)
	if len(compressed) > len(uncompressed) {
		t.Errorf("Compressed (%d) is larger than uncompressed (%d)", len(compressed), len(uncompressed))
	}

	// Verify the compressed PDF is valid (starts with %PDF)
	if !bytes.HasPrefix(compressed, []byte("%PDF")) {
		t.Error("Compressed PDF doesn't have valid header")
	}
}

func BenchmarkBundler_WithCompression(b *testing.B) {
	template, err := New(bundleTestPDF)
	if err != nil {
		b.Fatal(err)
	}

	fields := template.FieldNames()
	if len(fields) == 0 {
		b.Skip("No fields found")
	}

	formSets := []map[string]string{
		{fields[0]: "Value 1"},
		{fields[0]: "Value 2"},
		{fields[0]: "Value 3"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bundler := NewBundler()
		bundler.EnableCompression()
		bundler.FillMultiple(template, formSets...)
		_, err := bundler.Bundle()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function
func containsString(data []byte, substr []byte) bool {
	return len(data) > 0 && len(substr) > 0 &&
		(len(data) >= len(substr)) &&
		(bytes.Index(data, substr) != -1)
}

// TestRealWorldBundle tests bundling 1 PDF 1.3 form and 10 PDF 1.6 forms
// with all fields filled - simulating a real-world batch processing scenario.
func TestRealWorldBundle(t *testing.T) {
	// Parse both templates
	template13, err := New(bundlePDF13)
	if err != nil {
		t.Fatalf("New(PDF 1.3) error = %v", err)
	}

	template16, err := New(bundlePDF16)
	if err != nil {
		t.Fatalf("New(PDF 1.6) error = %v", err)
	}

	fields13 := template13.FieldNames()
	fields16 := template16.FieldNames()

	t.Logf("PDF 1.3 template: %d fields", len(fields13))
	t.Logf("PDF 1.6 template: %d fields", len(fields16))

	bundler := NewBundler()

	// Create form data for PDF 1.3 (1 form with all fields filled)
	formData13 := make(map[string]string)
	for i, field := range fields13 {
		formData13[field] = fmt.Sprintf("Value13_%d", i+1)
	}

	err = bundler.FillMultiple(template13, formData13)
	if err != nil {
		t.Fatalf("FillMultiple(PDF 1.3) error = %v", err)
	}

	// Create form data for PDF 1.6 (10 forms with all fields filled)
	formSets16 := make([]map[string]string, 10)
	for i := 0; i < 10; i++ {
		formSets16[i] = make(map[string]string)
		for j, field := range fields16 {
			formSets16[i][field] = fmt.Sprintf("Form%d_Field%d", i+1, j+1)
		}
	}

	err = bundler.FillMultiple(template16, formSets16...)
	if err != nil {
		t.Fatalf("FillMultiple(PDF 1.6) error = %v", err)
	}

	// Bundle all forms
	bundledPDF, err := bundler.Bundle()
	if err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}

	// Verify output
	if len(bundledPDF) == 0 {
		t.Error("Bundle() returned empty PDF")
	}

	if string(bundledPDF[:5]) != "%PDF-" {
		t.Error("Bundle() result doesn't start with PDF header")
	}

	t.Logf("Real-world bundle: %d bytes (1 PDF 1.3 + 10 PDF 1.6 forms)", len(bundledPDF))
	t.Logf("Total fields filled: %d", len(fields13)+10*len(fields16))
}

// BenchmarkRealWorldBundle benchmarks the real-world scenario of bundling
// 1 PDF 1.3 form and 10 PDF 1.6 forms with all fields filled.
func BenchmarkRealWorldBundle(b *testing.B) {
	// Parse both templates (outside benchmark loop)
	template13, err := New(bundlePDF13)
	if err != nil {
		b.Fatalf("New(PDF 1.3) error = %v", err)
	}

	template16, err := New(bundlePDF16)
	if err != nil {
		b.Fatalf("New(PDF 1.6) error = %v", err)
	}

	fields13 := template13.FieldNames()
	fields16 := template16.FieldNames()

	// Pre-create form data
	formData13 := make(map[string]string)
	for i, field := range fields13 {
		formData13[field] = fmt.Sprintf("Value13_%d", i+1)
	}

	formSets16 := make([]map[string]string, 10)
	for i := 0; i < 10; i++ {
		formSets16[i] = make(map[string]string)
		for j, field := range fields16 {
			formSets16[i][field] = fmt.Sprintf("Form%d_Field%d", i+1, j+1)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bundler := NewBundler()
		bundler.FillMultiple(template13, formData13)
		bundler.FillMultiple(template16, formSets16...)
		_, err := bundler.Bundle()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRealWorldBundleWithCompression benchmarks the same scenario with compression enabled.
func BenchmarkRealWorldBundleWithCompression(b *testing.B) {
	// Parse both templates (outside benchmark loop)
	template13, err := New(bundlePDF13)
	if err != nil {
		b.Fatalf("New(PDF 1.3) error = %v", err)
	}

	template16, err := New(bundlePDF16)
	if err != nil {
		b.Fatalf("New(PDF 1.6) error = %v", err)
	}

	fields13 := template13.FieldNames()
	fields16 := template16.FieldNames()

	// Pre-create form data
	formData13 := make(map[string]string)
	for i, field := range fields13 {
		formData13[field] = fmt.Sprintf("Value13_%d", i+1)
	}

	formSets16 := make([]map[string]string, 10)
	for i := 0; i < 10; i++ {
		formSets16[i] = make(map[string]string)
		for j, field := range fields16 {
			formSets16[i][field] = fmt.Sprintf("Form%d_Field%d", i+1, j+1)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bundler := NewBundler()
		bundler.EnableCompression()
		bundler.FillMultiple(template13, formData13)
		bundler.FillMultiple(template16, formSets16...)
		_, err := bundler.Bundle()
		if err != nil {
			b.Fatal(err)
		}
	}
}
