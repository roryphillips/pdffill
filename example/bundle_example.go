package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"

	"github.com/roryphillips/pdffill"
)

//go:embed template.pdf
var templatePDF []byte

func main() {
	// Initialize template
	template, err := pdffill.New(templatePDF)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Template loaded with %d fields\n", len(template.FieldNames()))

	// Create a bundler
	bundler := pdffill.NewBundler()

	// Simulate filling multiple OSHA forms for different employees
	employees := []map[string]string{
		{
			"Employee's Name 1":               "John Doe",
			"Log of Injury/Illness Year":      "2026",
			"Summary of Injury/Illness City":  "Springfield",
			"Summary of Injury/Illness State": "IL",
		},
		{
			"Employee's Name 1":               "Jane Smith",
			"Log of Injury/Illness Year":      "2026",
			"Summary of Injury/Illness City":  "Chicago",
			"Summary of Injury/Illness State": "IL",
		},
		{
			"Employee's Name 1":               "Bob Johnson",
			"Log of Injury/Illness Year":      "2026",
			"Summary of Injury/Illness City":  "Peoria",
			"Summary of Injury/Illness State": "IL",
		},
	}

	// Fill all forms from the same template
	fmt.Println("Filling forms...")
	if err := bundler.FillMultiple(template, employees...); err != nil {
		log.Fatal(err)
	}

	// Bundle into a single PDF
	fmt.Println("Bundling PDFs...")
	bundledPDF, err := bundler.Bundle()
	if err != nil {
		log.Fatal(err)
	}

	// Write to file
	outputPath := "employee_bundle.pdf"
	if err := os.WriteFile(outputPath, bundledPDF, 0644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✓ Bundled %d forms into %s (%d bytes)\n",
		len(employees), outputPath, len(bundledPDF))
	fmt.Printf("✓ Field names automatically deduplicated with f0_, f1_, f2_ prefixes\n")
}
