package main

import (
	_ "embed"
	"fmt"
	"log"

	"github.com/roryphillips/pdffill"
)

//go:embed template.pdf
var templatePDF []byte

func main() {
	template, err := pdffill.New(templatePDF)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Validation Examples ===\n")

	// Example 1: Check required fields
	requiredFields := template.GetRequiredFields()
	fmt.Printf("1. Required fields: %d found\n", len(requiredFields))
	if len(requiredFields) > 0 && len(requiredFields) <= 5 {
		for _, field := range requiredFields {
			fmt.Printf("   - %s\n", field)
		}
	}

	// Example 2: Inspect field constraints
	fmt.Println("\n2. Field constraints:")
	fieldsToCheck := []string{
		"Summary of Injury/Illness NAICS",
		"Employee's Name 1",
		"All other Total",
	}

	for _, fieldName := range fieldsToCheck {
		constraints, err := template.GetFieldConstraints(fieldName)
		if err != nil {
			continue
		}
		fmt.Printf("   %s:\n", fieldName)
		if constraints.MaxLen > 0 {
			fmt.Printf("     MaxLen: %d\n", constraints.MaxLen)
		}
		if constraints.ReadOnly {
			fmt.Printf("     ReadOnly: true\n")
		}
		if constraints.Required {
			fmt.Printf("     Required: true\n")
		}
	}

	// Example 3: Validation failure - MaxLen exceeded
	fmt.Println("\n3. Testing MaxLen validation:")
	formData := map[string]string{
		"Summary of Injury/Illness NAICS": "1234567890", // MaxLen is 6
	}

	opts := pdffill.StrictFillOptions()
	_, err = template.FillWithOptions(formData, opts)
	if err != nil {
		fmt.Printf("   ✓ Caught validation error: %v\n", err)
	}

	// Example 4: Auto-truncate long values
	fmt.Println("\n4. Testing auto-truncation:")
	opts.TruncateMaxLen = true
	filled, err := template.FillWithOptions(formData, opts)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   ✓ Successfully filled with truncation (%d bytes)\n", len(filled))
	fmt.Printf("   Value was truncated from '1234567890' to '123456'\n")

	// Example 5: Read-only field protection
	fmt.Println("\n5. Testing read-only field protection:")
	formData = map[string]string{
		"All other Total": "999", // This is a read-only calculated field
	}

	opts = pdffill.StrictFillOptions()
	opts.SkipReadOnly = false
	_, err = template.FillWithOptions(formData, opts)
	if err != nil {
		fmt.Printf("   ✓ Caught read-only error: %v\n", err)
	}

	// Example 6: Skip read-only fields
	fmt.Println("\n6. Testing read-only skip:")
	formData["Employee's Name 1"] = "John Doe"
	opts.SkipReadOnly = true
	filled, err = template.FillWithOptions(formData, opts)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   ✓ Successfully filled with SkipReadOnly (%d bytes)\n", len(filled))
	fmt.Printf("   Read-only fields were silently skipped\n")

	// Example 7: Multiple validation errors
	fmt.Println("\n7. Testing multiple validation errors:")
	formData = map[string]string{
		"Summary of Injury/Illness NAICS": "1234567890",  // Too long
		"All other Total":                 "100",          // Read-only
		"NonExistentField":                "value",        // Doesn't exist
	}

	opts = pdffill.StrictFillOptions()
	opts.SkipReadOnly = false
	_, err = template.FillWithOptions(formData, opts)
	if valErr, ok := err.(*pdffill.ValidationErrors); ok {
		fmt.Printf("   ✓ Caught %d validation errors:\n", len(valErr.Errors))
		for i, e := range valErr.Errors {
			fmt.Printf("     %d. %s: %s\n", i+1, e.Field, e.Message)
		}
	}

	// Example 8: Validate without filling
	fmt.Println("\n8. Testing ValidateOnly:")
	formData = map[string]string{
		"Employee's Name 1": "Jane Smith",
		"Log of Injury/Illness Year": "2026",
	}

	err = template.ValidateOnly(formData, pdffill.StrictFillOptions())
	if err != nil {
		fmt.Printf("   ✗ Validation failed: %v\n", err)
	} else {
		fmt.Printf("   ✓ Validation passed - data is valid\n")
	}

	fmt.Println("\n=== Validation Examples Complete ===")
}
