package pdffill

import (
	"fmt"
)

// Template represents a parsed PDF form template ready for filling.
//
// A Template is created once from PDF bytes and can be reused to fill
// multiple forms efficiently. It is safe for concurrent use after creation.
//
// The Template caches the parsed field structure, making subsequent
// Fill operations very fast (~6-10ms).
type Template struct {
	data   []byte
	fields map[string]*fieldRef
}

// fieldRef stores the location and metadata of a form field in the PDF.
// This is an internal structure used for fast field lookup.
type fieldRef struct {
	objNum int // PDF object number containing the field
	offset int // Byte offset in the PDF
	length int // Length of the field content
}

// New creates a new Template from PDF bytes.
//
// The PDF data is typically loaded from an embedded file using go:embed,
// or read from disk. The Template parses the PDF structure to locate all
// AcroForm fields, which takes ~750ms for complex forms.
//
// Once created, the Template can be reused for unlimited fills without
// re-parsing, making it ideal for high-throughput scenarios.
//
// Example:
//
//	//go:embed form.pdf
//	var formPDF []byte
//
//	template, err := pdffill.New(formPDF)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Reuse template for many fills
//	for _, data := range formDataSets {
//		filled, _ := template.Fill(data)
//		// process filled PDF
//	}
//
// Returns an error if the PDF is invalid or cannot be parsed.
func New(pdfData []byte) (*Template, error) {
	if len(pdfData) < 5 || string(pdfData[:5]) != "%PDF-" {
		return nil, fmt.Errorf("invalid PDF: missing header")
	}

	t := &Template{
		data:   make([]byte, len(pdfData)),
		fields: make(map[string]*fieldRef),
	}
	copy(t.data, pdfData)

	if err := t.parseFields(); err != nil {
		return nil, fmt.Errorf("parse fields: %w", err)
	}

	return t, nil
}

// FieldNames returns all available field names in the template.
//
// Use this to discover what fields exist in a PDF form. The returned
// names can be used as keys in the formData map passed to Fill.
//
// Example:
//
//	fields := template.FieldNames()
//	for _, name := range fields {
//		fmt.Println(name)
//	}
//
// The order of field names is not guaranteed.
func (t *Template) FieldNames() []string {
	names := make([]string, 0, len(t.fields))
	for name := range t.fields {
		names = append(names, name)
	}
	return names
}
