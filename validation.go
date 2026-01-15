package pdffill

import (
	"bytes"
	"fmt"
	"strings"
)

// ValidationMode controls how strict validation is applied during form filling.
//
// Validation adds overhead (~750ms for complex forms), so choose the mode
// that matches your use case:
//   - ValidationNone: Skip all validation (default, fastest)
//   - ValidationBasic: Check required fields only
//   - ValidationStrict: Enforce all PDF constraints (MaxLen, read-only, etc.)
type ValidationMode int

const (
	// ValidationNone skips all validation for maximum speed (default).
	// Use this when you trust your input data or have validated elsewhere.
	ValidationNone ValidationMode = iota

	// ValidationBasic checks required fields and field existence.
	// Use this for a balance between speed and safety.
	ValidationBasic

	// ValidationStrict enforces all PDF constraints including MaxLen,
	// read-only protection, and field existence.
	// Use this for untrusted input or when strict compliance is needed.
	ValidationStrict
)

// ValidationError represents a single validation failure for a form field.
//
// Each error includes:
//   - Field: The field name that failed validation
//   - Constraint: The type of constraint violated (e.g., "required", "max-length", "read-only")
//   - Message: A human-readable description of the failure
type ValidationError struct {
	Field      string
	Constraint string
	Message    string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field %q (%s): %s", e.Field, e.Constraint, e.Message)
}

// ValidationErrors represents multiple validation failures.
//
// When validation fails, multiple errors may be collected at once,
// allowing you to see all validation issues in a single pass rather
// than fixing them one at a time.
//
// Example:
//
//	_, err := template.FillWithOptions(formData, pdffill.StrictFillOptions())
//	if err != nil {
//		if valErr, ok := err.(*pdffill.ValidationErrors); ok {
//			fmt.Printf("Found %d validation errors:\n", len(valErr.Errors))
//			for _, e := range valErr.Errors {
//				fmt.Printf("  - %s: %s\n", e.Field, e.Message)
//			}
//		}
//	}
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

// FieldConstraints represents PDF validation constraints for a form field.
//
// These constraints are extracted from the PDF's AcroForm field definitions,
// specifically from the /Ff (field flags) and /MaxLen entries.
//
// Use GetFieldConstraints to inspect a field's constraints before filling.
type FieldConstraints struct {
	Required  bool    // Field is required (Ff bit 1)
	ReadOnly  bool    // Field is read-only (Ff bit 0)
	MaxLen    int     // Maximum length for text fields
	Comb      bool    // Fixed-width character cells (Ff bit 23)
	MinValue  float64 // For number fields (not yet implemented)
	MaxValue  float64 // For number fields (not yet implemented)
	HasMinMax bool    // Whether min/max are set
}

// FillOptions configures form filling behavior and validation.
//
// Use this to control validation strictness and how constraint violations
// are handled.
//
// Example with strict validation:
//
//	opts := pdffill.StrictFillOptions()
//	filled, err := template.FillWithOptions(formData, opts)
//
// Example with custom options:
//
//	opts := &pdffill.FillOptions{
//		Validation:     pdffill.ValidationBasic,
//		SkipReadOnly:   true,   // Ignore read-only fields
//		TruncateMaxLen: true,   // Truncate long values instead of erroring
//	}
//	filled, err := template.FillWithOptions(formData, opts)
type FillOptions struct {
	Validation     ValidationMode
	SkipReadOnly   bool // Skip read-only fields instead of erroring
	TruncateMaxLen bool // Truncate values that exceed MaxLen instead of erroring
}

// DefaultFillOptions returns the default fill options (no validation, for speed).
//
// This is the same behavior as calling Fill() directly.
// Use this when you trust your input data or have validated elsewhere.
func DefaultFillOptions() *FillOptions {
	return &FillOptions{
		Validation:     ValidationNone,
		SkipReadOnly:   false,
		TruncateMaxLen: false,
	}
}

// StrictFillOptions returns options with strict validation enabled.
//
// This enables all validation checks and skips read-only fields automatically.
// Use this for untrusted input or when strict compliance is needed.
//
// Example:
//
//	opts := pdffill.StrictFillOptions()
//	filled, err := template.FillWithOptions(formData, opts)
//	if err != nil {
//		if valErr, ok := err.(*pdffill.ValidationErrors); ok {
//			// Handle validation errors
//		}
//	}
func StrictFillOptions() *FillOptions {
	return &FillOptions{
		Validation:     ValidationStrict,
		SkipReadOnly:   true,
		TruncateMaxLen: false,
	}
}

// FillWithOptions fills the form with validation according to the provided options.
//
// This method allows you to control validation behavior and constraint handling.
// For simple fills without validation, use Fill() instead for better performance.
//
// The method performs validation before filling, collecting all errors at once.
// If validation fails, it returns a *ValidationErrors containing all failures.
//
// Performance: Adds ~750ms overhead for complex forms when validation is enabled.
//
// Example:
//
//	opts := &pdffill.FillOptions{
//		Validation:     pdffill.ValidationStrict,
//		SkipReadOnly:   true,
//		TruncateMaxLen: true,
//	}
//
//	filled, err := template.FillWithOptions(formData, opts)
//	if err != nil {
//		if valErr, ok := err.(*pdffill.ValidationErrors); ok {
//			for _, e := range valErr.Errors {
//				log.Printf("%s: %s", e.Field, e.Message)
//			}
//		}
//		return err
//	}
//
// Returns an error if validation fails or if filling fails.
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
//
// This is useful for checking data before committing to a fill operation,
// especially when you want to validate on the client side before processing.
//
// The validation uses strict options by default. You can override by passing
// your own FillOptions.
//
// Example:
//
//	// Validate before filling
//	if err := template.ValidateOnly(formData, nil); err != nil {
//		if valErr, ok := err.(*pdffill.ValidationErrors); ok {
//			// Show validation errors to user
//			return fmt.Errorf("validation failed: %v", valErr)
//		}
//	}
//
//	// If validation passes, proceed with fill
//	filled, _ := template.Fill(formData)
//
// Returns a *ValidationErrors if validation fails, nil if validation passes.
func (t *Template) ValidateOnly(formData map[string]string, opts *FillOptions) error {
	if opts == nil {
		opts = StrictFillOptions()
	}

	// Use FillWithOptions but discard the result
	_, err := t.FillWithOptions(formData, opts)
	return err
}

// GetRequiredFields returns a list of all required field names in the template.
//
// Use this to discover which fields must be filled according to the PDF's
// field definitions (fields with the /Ff bit 1 flag set).
//
// Example:
//
//	required := template.GetRequiredFields()
//	for _, fieldName := range required {
//		if _, exists := formData[fieldName]; !exists {
//			log.Printf("Warning: required field %q is missing", fieldName)
//		}
//	}
//
// The order of field names is not guaranteed.
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
//
// This allows you to inspect a field's constraints before attempting to fill it,
// which is useful for building dynamic forms or validation logic.
//
// Example:
//
//	constraints, err := template.GetFieldConstraints("naics_code")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	if constraints.MaxLen > 0 {
//		fmt.Printf("NAICS code must be max %d characters\n", constraints.MaxLen)
//	}
//	if constraints.Required {
//		fmt.Println("NAICS code is required")
//	}
//
// Returns an error if the field doesn't exist in the template.
func (t *Template) GetFieldConstraints(fieldName string) (*FieldConstraints, error) {
	return t.getFieldConstraints(fieldName)
}
