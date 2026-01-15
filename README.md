# pdffill

[![Go Reference](https://pkg.go.dev/badge/github.com/roryphillips/pdffill.svg)](https://pkg.go.dev/github.com/roryphillips/pdffill)
[![Go Report Card](https://goreportcard.com/badge/github.com/roryphillips/pdffill)](https://goreportcard.com/report/github.com/roryphillips/pdffill)

Fast, minimal PDF form filling library using only Go stdlib.

## Features

- **Zero dependencies** - only uses Go standard library
- **Fast** - optimized for speed with minimal allocations
- **Simple API** - initialize once, fill many times
- **PDF 1.3 - 1.7 support** - handles XRef streams and compressed object streams
- **Complete field type support** - text, multiline, checkboxes, radio buttons, numbers
- **Comprehensive validation** - MaxLen, required fields, read-only checks
- **Flexible validation modes** - none, basic, or strict
- **PDF Bundling** - combine multiple filled forms into one document
- **Optional compression** - reduce bundle file size with flate compression
- **Field deduplication** - automatic field name prefixing for bundles
- **Field introspection** - query field types, metadata, and constraints
- **Focused** - does one thing well: fill PDF forms
- **Embedded-friendly** - works great with `go:embed`

## Installation

```bash
go get github.com/roryphillips/pdffill
```

## Usage

```go
package main

import (
    _ "embed"
    "log"
    "os"

    "github.com/roryphillips/pdffill"
)

//go:embed template.pdf
var templatePDF []byte

var template *pdffill.Template

func init() {
    var err error
    template, err = pdffill.New(templatePDF)
    if err != nil {
        log.Fatal(err)
    }
}

func main() {
    // Fill the form
    formData := map[string]string{
        "name":    "John Doe",
        "email":   "john@example.com",
        "company": "Acme Corp",
    }

    filledPDF, err := template.Fill(formData)
    if err != nil {
        log.Fatal(err)
    }

    // Write to file
    if err := os.WriteFile("output.pdf", filledPDF, 0644); err != nil {
        log.Fatal(err)
    }
}
```

## Discovering Field Names

To find out what field names are available in your PDF template:

```go
template, _ := pdffill.New(templatePDF)
fields := template.FieldNames()
for _, name := range fields {
    fmt.Println(name)
}
```

## Field Type Support

The library automatically handles different PDF field types:

### Text Fields
```go
formData := map[string]string{
    "name":    "John Doe",
    "email":   "john@example.com",
    "address": "123 Main St",
}
```

### Multiline Text Fields
```go
formData := map[string]string{
    "description": `This is a long description
that spans multiple lines
and will be properly encoded.`,
}
```

### Checkboxes
Accepts various truthy/falsy values:
```go
formData := map[string]string{
    "agreed":     "Yes",    // or "On", "true", "1", "X", "checked"
    "subscribed": "Off",    // or "No", "false", "0", ""
}
```

### Radio Buttons
```go
formData := map[string]string{
    "gender":        "Male",      // or "Female", "Other"
    "maritalStatus": "Single",    // select one option from the group
}
```

### Number Fields
```go
formData := map[string]string{
    "age":      "35",
    "quantity": "42",
    "amount":   "1250.50",
}
```

### Inspecting Field Types
```go
info, err := template.GetFieldInfo("fieldName")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Field: %s\n", info.Name)
fmt.Printf("Type: %s\n", info.Type)  // Text, Checkbox, Radio, etc.
fmt.Printf("Object Number: %d\n", info.ObjNum)

if info.Type == pdffill.FieldTypeRadio {
    fmt.Printf("Radio options: %d\n", len(info.KidRefs))
}
```

## Validation

The library supports comprehensive validation of form data before filling:

### Basic Usage

```go
// Use strict validation
opts := pdffill.StrictFillOptions()
filled, err := template.FillWithOptions(formData, opts)
if err != nil {
    // Handle validation errors
    log.Printf("Validation failed: %v", err)
}
```

### Validation Modes

```go
// No validation (fastest, default)
opts := pdffill.DefaultFillOptions()
opts.Validation = pdffill.ValidationNone

// Basic validation (required fields)
opts.Validation = pdffill.ValidationBasic

// Strict validation (all constraints)
opts.Validation = pdffill.ValidationStrict
```

### Validation Features

**Max Length Constraints**
```go
// Field has MaxLen=6, value is too long
formData := map[string]string{
    "naics_code": "1234567890", // Error: exce
	// eds maximum 6
}

// Or truncate automatically
opts := pdffill.StrictFillOptions()
opts.TruncateMaxLen = true  // Truncates to "123456"
```

**Read-Only Fields**
```go
// Trying to set read-only/calculated field
formData := map[string]string{
    "total_field": "100", // Error: field is read-only
}

// Or skip read-only fields
opts := pdffill.StrictFillOptions()
opts.SkipReadOnly = true  // Silently skips read-only fields
```

**Required Fields**
```go
// Check required fields first
requiredFields := template.GetRequiredFields()
fmt.Printf("Required: %v\n", requiredFields)

// Validation will error if required fields are missing
```

**Validation Without Filling**
```go
// Validate data before committing to fill
err := template.ValidateOnly(formData, pdffill.StrictFillOptions())
if err != nil {
    log.Printf("Data validation failed: %v", err)
    return
}

// Data is valid, proceed with fill
filled, _ := template.Fill(formData)
```

**Inspect Field Constraints**
```go
constraints, err := template.GetFieldConstraints("fieldName")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Required: %v\n", constraints.Required)
fmt.Printf("ReadOnly: %v\n", constraints.ReadOnly)
fmt.Printf("MaxLen: %d\n", constraints.MaxLen)
fmt.Printf("Comb: %v\n", constraints.Comb)
```

**Multiple Validation Errors**
```go
// Returns all validation errors at once
filled, err := template.FillWithOptions(formData, opts)
if err != nil {
    if valErr, ok := err.(*pdffill.ValidationErrors); ok {
        fmt.Printf("Found %d validation errors:\n", len(valErr.Errors))
        for _, e := range valErr.Errors {
            fmt.Printf("  - %s: %s\n", e.Field, e.Message)
        }
    }
}
```

## Bundling Multiple Filled PDFs

Combine multiple filled forms into a single PDF document:

```go
bundler := pdffill.NewBundler()

// Add multiple filled forms from the same template
err := bundler.FillMultiple(template,
    map[string]string{"name": "John Doe", "year": "2026"},
    map[string]string{"name": "Jane Smith", "year": "2026"},
    map[string]string{"name": "Bob Johnson", "year": "2027"},
)
if err != nil {
    log.Fatal(err)
}

// Add forms from different templates
err = bundler.FillMultiple(otherTemplate,
    map[string]string{"field1": "value1"},
    map[string]string{"field1": "value2"},
)
if err != nil {
    log.Fatal(err)
}

// Generate the bundled PDF
bundledPDF, err := bundler.Bundle()
if err != nil {
    log.Fatal(err)
}

os.WriteFile("bundled.pdf", bundledPDF, 0644)
```

**Key features:**
- Field names are automatically deduplicated with prefixes (`f0_`, `f1_`, etc.)
- Forms are grouped by template in the bundle
- Efficient object renumbering and page concatenation
- All objects and cross-references are properly merged

### Compression

Enable optional stream compression to reduce bundle file size:

```go
bundler := pdffill.NewBundler()
bundler.EnableCompression()  // Enable flate compression
bundler.FillMultiple(template, data1, data2, data3)

compressedPDF, err := bundler.Bundle()  // Smaller file size
```

**Compression tradeoffs:**
- Reduces file size (varies by content, typically 4-30% for PDFs with uncompressed streams)
- Adds ~50% processing overhead
- Already-compressed streams are automatically skipped
- Only compresses if it actually reduces size

Use compression when file size matters more than speed (e.g., email attachments, storage).

## HTTP Server Example

```go
func handleFillPDF(w http.ResponseWriter, r *http.Request) {
    formData := map[string]string{
        "name":  r.FormValue("name"),
        "email": r.FormValue("email"),
    }

    filledPDF, err := template.Fill(formData)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/pdf")
    w.Header().Set("Content-Disposition", "attachment; filename=filled.pdf")
    w.Write(filledPDF)
}

func handleBundlePDFs(w http.ResponseWriter, r *http.Request) {
    // Parse multiple form submissions
    var formSets []map[string]string
    // ... populate formSets from request ...

    bundler := pdffill.NewBundler()
    bundler.FillMultiple(template, formSets...)

    bundledPDF, err := bundler.Bundle()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/pdf")
    w.Header().Set("Content-Disposition", "attachment; filename=bundle.pdf")
    w.Write(bundledPDF)
}
```

## Performance

Benchmarks on Apple M1 with a 205-field OSHA form:

### Single Form Filling
```
BenchmarkNew-8            2    759ms/op    3.0 MB/op     961 allocs/op
BenchmarkFill-8         176      7ms/op    7.1 MB/op    9511 allocs/op
BenchmarkFillMultiple-8  54     24ms/op    7.1 MB/op    9537 allocs/op
```

- Template parsing: ~759ms (done once)
- Single field fill: ~7ms
- Multiple field fill: ~24ms

### PDF Bundling
```
BenchmarkBundler_FillMultiple-8   51   26ms/op   21 MB/op   28570 allocs/op
BenchmarkBundler_Bundle-8         14  110ms/op   61 MB/op  135209 allocs/op
BenchmarkBundler_FullWorkflow-8   10  107ms/op   83 MB/op  133095 allocs/op
```

- Fill 3 forms: ~26ms
- Bundle into single PDF: ~110ms
- Full workflow (fill + bundle): ~107ms

### Validation Performance
```
BenchmarkFillNoValidation-8       106   11ms/op   7.1 MB/op   9517 allocs/op
BenchmarkFillWithValidation-8       2  829ms/op   7.1 MB/op  10245 allocs/op
```

- No validation (default): ~11ms
- With strict validation: ~829ms (only recommended when needed)

## Supported Field Types

✅ Text fields
✅ Multiline text fields
✅ Number fields
✅ Checkboxes
✅ Radio button groups
❌ Choice/dropdown fields (coming soon)
❌ Signature fields

## Limitations

- **AcroForms only** - only supports PDF AcroForm fields
- **No creation** - cannot create PDFs from scratch
- **JavaScript validation** - doesn't execute JavaScript validation scripts (but does check PDF-level constraints)
- **No encryption** - doesn't support encrypted PDFs

## Design Philosophy

This library does exactly one thing: fill PDF form fields as fast as possible using only the Go standard library. It doesn't try to be a full-featured PDF library. For more complex PDF operations, consider other libraries.

## PDF Utilities

The library also includes utilities for working with PDFs:

### Count Pages
```go
count, err := pdffill.CountPages(pdfData)
fmt.Printf("PDF has %d pages\n", count)
```

### Extract Single Page
```go
page3, err := pdffill.ExtractPage(pdfData, 3)
os.WriteFile("page3.pdf", page3, 0644)
```

### Split PDF into Pages
```go
pages, err := pdffill.SplitPages(pdfData)
for i, pageData := range pages {
    filename := fmt.Sprintf("page_%d.pdf", i+1)
    os.WriteFile(filename, pageData, 0644)
}
```

## Contributing

Contributions are welcome! Please ensure:
- All tests pass (`go test ./...`)
- Code is formatted (`go fmt ./...`)
- Documentation is updated for new features
- Benchmarks are included for performance-critical changes

## License

MIT - see [LICENSE](LICENSE) file for details.
