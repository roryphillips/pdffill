package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"

	"github.com/roryq/pdffill"
)

//go:embed template.pdf
var templatePDF []byte

func main() {
	// Initialize template
	template, err := pdffill.New(templatePDF)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Template loaded successfully with %d fields\n", len(template.FieldNames()))

	// Fill form with data
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

	// Write to file
	outputPath := "filled_form.pdf"
	if err := os.WriteFile(outputPath, filledPDF, 0644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("PDF filled successfully: %s (%d bytes)\n", outputPath, len(filledPDF))
}
