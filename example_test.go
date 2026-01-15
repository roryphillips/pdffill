package pdffill_test

import (
	_ "embed"
	"fmt"
	"log"
	"os"

	"github.com/roryphillips/pdffill"
)

//go:embed testdata/osha_bundle.pdf
var templatePDF []byte

func Example() {
	// Initialize template once (e.g., at package level)
	template, err := pdffill.New(templatePDF)
	if err != nil {
		log.Fatal(err)
	}

	// Fill form with data (using actual field names from the OSHA form)
	formData := map[string]string{
		"Summary of Injury/Illness City":  "Springfield",
		"Summary of Injury/Illness State": "IL",
		"Summary of Injury/Illness Phone": "555-1234",
		"Log of Injury/Illness Year":      "2026",
	}

	filledPDF, err := template.Fill(formData)
	if err != nil {
		log.Fatal(err)
	}

	// Write to file or serve via HTTP
	if err := os.WriteFile("output.pdf", filledPDF, 0644); err != nil {
		log.Fatal(err)
	}

	fmt.Println("PDF filled successfully")
	// Output: PDF filled successfully
}

func ExampleTemplate_FieldNames() {
	template, err := pdffill.New(templatePDF)
	if err != nil {
		log.Fatal(err)
	}

	// List all available field names
	fields := template.FieldNames()
	for _, name := range fields {
		fmt.Println(name)
	}
}
