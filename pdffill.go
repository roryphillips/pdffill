// Package pdffill provides fast, minimal PDF form filling using only Go stdlib.
package pdffill

import (
	"fmt"
)

// Template represents a PDF template ready for form filling.
type Template struct {
	data   []byte
	fields map[string]*fieldRef
}

// fieldRef stores the location and metadata of a form field in the PDF.
type fieldRef struct {
	objNum int
	offset int
	length int
}

// New creates a new Template from PDF bytes (e.g., from go:embed).
// It parses the PDF structure and locates all AcroForm fields.
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

// FieldNames returns a list of all available field names in the template.
func (t *Template) FieldNames() []string {
	names := make([]string, 0, len(t.fields))
	for name := range t.fields {
		names = append(names, name)
	}
	return names
}
