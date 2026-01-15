package pdffill

import (
	"bytes"
	"fmt"
	"strings"
)

// ValidationMode controls how strict validation is applied.
type ValidationMode int

const (
	// ValidationNone skips all validation (fastest, default)
	ValidationNone ValidationMode = iota
	// ValidationBasic checks required fields and basic constraints
	ValidationBasic
	// ValidationStrict enforces all PDF constraints including MaxLen, read-only, etc.
	ValidationStrict
)

// ValidationError represents a validation failure.
type ValidationError struct {
	Field      string
	Constraint string
	Message    string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field %q (%s): %s", e.Field, e.Constraint, e.Message)
}

// ValidationErrors represents multiple validation failures.
type ValidationErrors struct {
	Errors []*ValidationError
}

func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d validation errors:\n", len(e.Errors)))
	for i, err := range e.Errors {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
	}
	return sb.String()
}

// Add adds a validation error to the collection.
func (e *ValidationErrors) Add(field, constraint, message string) {
	e.Errors = append(e.Errors, &ValidationError{
		Field:      field,
		Constraint: constraint,
		Message:    message,
	})
}

// HasErrors returns true if there are any validation errors.
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// FieldConstraints represents validation constraints for a field.
type FieldConstraints struct {
	Required   bool
	ReadOnly   bool
	MaxLen     int
	Comb       bool    // Fixed-width character cells
	MinValue   float64 // For number fields
	MaxValue   float64 // For number fields
	HasMinMax  bool    // Whether min/max are set
}

// FillOptions configures how form filling behaves.
type FillOptions struct {
	Validation     ValidationMode
	SkipReadOnly   bool // Skip read-only fields instead of erroring
	TruncateMaxLen bool // Truncate values that exceed MaxLen instead of erroring
}

// DefaultFillOptions returns the default fill options (no validation, for speed).
func DefaultFillOptions() *FillOptions {
	return &FillOptions{
		Validation:     ValidationNone,
		SkipReadOnly:   false,
		TruncateMaxLen: false,
	}
}

// StrictFillOptions returns options with strict validation enabled.
func StrictFillOptions() *FillOptions {
	return &FillOptions{
		Validation:     ValidationStrict,
		SkipReadOnly:   true,
		TruncateMaxLen: false,
	}
}

// FillWithOptions fills the form with validation according to options.
func (t *Template) FillWithOptions(formData map[string]string, opts *FillOptions) ([]byte, error) {
	if opts == nil {
		opts = DefaultFillOptions()
	}

	// If validation is disabled, use fast path
	if opts.Validation == ValidationNone {
		return t.Fill(formData)
	}

	// Validate the form data first
	validationErrors := &ValidationErrors{}

	// Build constraint cache
	constraints := make(map[string]*FieldConstraints)
	for fieldName := range formData {
		if _, exists := t.fields[fieldName]; !exists {
			validationErrors.Add(fieldName, "existence", "field does not exist in template")
			continue
		}

		c, err := t.getFieldConstraints(fieldName)
		if err != nil {
			// Skip fields we can't analyze
			continue
		}
		constraints[fieldName] = c
	}

	// Check for required fields if validation is enabled
	if opts.Validation >= ValidationBasic {
		for fieldName := range t.fields {
			c, err := t.getFieldConstraints(fieldName)
			if err != nil {
				continue
			}

			if c.Required {
				value, provided := formData[fieldName]
				if !provided || value == "" {
					validationErrors.Add(fieldName, "required", "field is required but no value provided")
				}
			}

			// Check if trying to set read-only field
			if c.ReadOnly {
				if _, provided := formData[fieldName]; provided {
					if opts.SkipReadOnly {
						// Remove from formData to skip
						delete(formData, fieldName)
					} else {
						validationErrors.Add(fieldName, "read-only", "field is read-only")
					}
				}
			}
		}
	}

	// Validate individual field values
	for fieldName, value := range formData {
		c, exists := constraints[fieldName]
		if !exists {
			continue
		}

		// Check MaxLen
		if c.MaxLen > 0 && len(value) > c.MaxLen {
			if opts.TruncateMaxLen {
				formData[fieldName] = value[:c.MaxLen]
			} else {
				validationErrors.Add(fieldName, "max-length",
					fmt.Sprintf("value length %d exceeds maximum %d", len(value), c.MaxLen))
			}
		}

		// Additional strict validations
		if opts.Validation == ValidationStrict {
			// Add more validations here as needed
		}
	}

	// Return validation errors if any
	if validationErrors.HasErrors() {
		return nil, validationErrors
	}

	// Proceed with filling
	return t.Fill(formData)
}

// getFieldConstraints extracts validation constraints from a field.
func (t *Template) getFieldConstraints(fieldName string) (*FieldConstraints, error) {
	fieldRef, exists := t.fields[fieldName]
	if !exists {
		return nil, fmt.Errorf("field not found")
	}

	obj, err := t.getObject(fieldRef.objNum)
	if err != nil {
		return nil, err
	}

	c := &FieldConstraints{}

	// Extract field flags
	ffIdx := bytes.Index(obj.content, []byte("/Ff "))
	if ffIdx != -1 {
		start := ffIdx + 4
		for start < len(obj.content) && isWhitespace(obj.content[start]) {
			start++
		}
		end := start
		for end < len(obj.content) && isDigit(obj.content[end]) {
			end++
		}
		if end > start {
			flags, _ := parseInt(obj.content[start:end])

			// Bit 0: Read-only
			c.ReadOnly = (flags & 1) != 0

			// Bit 1: Required
			c.Required = (flags & 2) != 0

			// Bit 23: Comb
			c.Comb = (flags & (1 << 23)) != 0
		}
	}

	// Extract MaxLen
	maxLenIdx := bytes.Index(obj.content, []byte("/MaxLen"))
	if maxLenIdx != -1 {
		start := maxLenIdx + 7
		for start < len(obj.content) && isWhitespace(obj.content[start]) {
			start++
		}
		end := start
		for end < len(obj.content) && isDigit(obj.content[end]) {
			end++
		}
		if end > start {
			c.MaxLen, _ = parseInt(obj.content[start:end])
		}
	}

	return c, nil
}

// ValidateOnly validates form data without filling the PDF.
// Useful for checking data before committing to fill operation.
func (t *Template) ValidateOnly(formData map[string]string, opts *FillOptions) error {
	if opts == nil {
		opts = StrictFillOptions()
	}

	// Use FillWithOptions but discard the result
	_, err := t.FillWithOptions(formData, opts)
	return err
}

// GetRequiredFields returns a list of all required field names.
func (t *Template) GetRequiredFields() []string {
	var required []string

	for fieldName := range t.fields {
		c, err := t.getFieldConstraints(fieldName)
		if err != nil {
			continue
		}

		if c.Required {
			required = append(required, fieldName)
		}
	}

	return required
}

// GetFieldConstraints returns validation constraints for a specific field.
// This is exported for user inspection.
func (t *Template) GetFieldConstraints(fieldName string) (*FieldConstraints, error) {
	return t.getFieldConstraints(fieldName)
}
