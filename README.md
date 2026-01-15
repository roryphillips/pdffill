# pdffill

Fast, minimal PDF form filling library using only Go stdlib.

## Features

- **Zero dependencies** - only uses Go standard library
- **Fast** - optimized for speed with minimal allocations
- **Simple API** - initialize once, fill many times
- **Focused** - does one thing well: fill PDF forms
- **Embedded-friendly** - works great with `go:embed`

## Installation

```bash
go get github.com/roryq/pdffill
```

## Usage

```go
package main

import (
    _ "embed"
    "log"
    "os"

    "github.com/roryq/pdffill"
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
```

## Performance

Benchmarks on Apple M1 with a 205-field OSHA form:

```
BenchmarkNew-8            2    759ms/op    3.0 MB/op     961 allocs/op
BenchmarkFill-8         176      7ms/op    7.1 MB/op    9511 allocs/op
BenchmarkFillMultiple-8  54     24ms/op    7.1 MB/op    9537 allocs/op
```

- Template parsing: ~759ms (done once)
- Single field fill: ~7ms
- Multiple field fill: ~24ms

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
