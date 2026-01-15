# pdffill

Fast, minimal PDF form filling library using only Go stdlib.

## Features

- **Zero dependencies** - only uses Go standard library
- **Fast** - optimized for speed with minimal allocations
- **Simple API** - initialize once, fill many times
- **PDF Bundling** - combine multiple filled forms into one document
- **Field deduplication** - automatic field name prefixing for bundles
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

## Limitations

- **AcroForms only** - only supports PDF AcroForm fields
- **Text fields** - optimized for text field values
- **No creation** - cannot create PDFs from scratch
- **No validation** - doesn't validate field types or values
- **No encryption** - doesn't support encrypted PDFs

## Design Philosophy

This library does exactly one thing: fill PDF form fields as fast as possible using only the Go standard library. It doesn't try to be a full-featured PDF library. For more complex PDF operations, consider other libraries.

## License

MIT
