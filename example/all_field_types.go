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

	// Demonstrate all field types
	formData := map[string]string{
		// === TEXT FIELDS ===
		"Employee's Name 1":               "Jane Smith",
		"Log of Injury/Illness Year":      "2026",
		"Summary of Injury/Illness City":  "Chicago",
		"Summary of Injury/Illness State": "IL",
		"Summary of Injury/Illness Phone": "312-555-0100",

		// === NUMBER FIELDS ===
		"Number of days injured or ill away from work 1":        "5",
		"Summary of Injury/Illness Annual avg num of employees": "250",

		// === CHECKBOXES ===
		"Reset 1": "Yes", // Checked
		"Reset 2": "Off", // Unchecked

		// === RADIO BUTTONS ===
		"301 Gender": "Female",
		"301 ER":     "Yes",

		// === MULTILINE TEXT ===
		"301 What Happened": `Employee was moving boxes in the warehouse when
she slipped on a wet floor. The floor had been recently mopped
but warning signs were not placed. Employee fell and injured her wrist.

Immediate first aid was administered and employee was sent to ER
for x-rays. No fracture detected but severe sprain diagnosed.`,
	}

	// Fill the form
	filledPDF, err := template.Fill(formData)
	if err != nil {
		log.Fatal(err)
	}

	// Write to file
	outputPath := "all_field_types_filled.pdf"
	if err := os.WriteFile(outputPath, filledPDF, 0644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\n✓ Form filled successfully: %s (%d bytes)\n", outputPath, len(filledPDF))
	fmt.Println("\nField types used:")
	fmt.Println("  - Text fields: Employee name, year, city, state, phone")
	fmt.Println("  - Number fields: Days away from work, employee count")
	fmt.Println("  - Checkboxes: Reset buttons (checked/unchecked)")
	fmt.Println("  - Radio buttons: Gender, ER visit")
	fmt.Println("  - Multiline text: Incident description")

	// Demonstrate field inspection
	fmt.Println("\nInspecting field types:")
	fieldsToInspect := []string{
		"Employee's Name 1",
		"Reset 1",
		"301 Gender",
		"301 What Happened",
	}

	for _, fieldName := range fieldsToInspect {
		info, err := template.GetFieldInfo(fieldName)
		if err != nil {
			continue
		}
		fmt.Printf("  %s: %s\n", fieldName, info.Type)
	}
}
