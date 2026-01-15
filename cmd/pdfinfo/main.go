package main

import (
	"fmt"
	"log"
	"os"

	"github.com/roryphillips/pdffill"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <pdf-file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nDisplays information about a PDF file.\n")
		os.Exit(1)
	}

	inputFile := os.Args[1]

	// Read PDF
	fmt.Printf("Reading %s...\n", inputFile)
	pdfData, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Failed to read PDF: %v", err)
	}

	// Get page count
	pageCount, err := pdffill.CountPages(pdfData)
	if err != nil {
		log.Fatalf("Failed to count pages: %v", err)
	}

	fmt.Printf("\n=== PDF Information ===\n")
	fmt.Printf("File: %s\n", inputFile)
	fmt.Printf("Size: %d bytes (%.2f MB)\n", len(pdfData), float64(len(pdfData))/1024/1024)
	fmt.Printf("Pages: %d\n", pageCount)

	// Extract page references
	pageRefs, err := pdffill.ExtractPageRefs(pdfData)
	if err != nil {
		log.Printf("Warning: Failed to extract page refs: %v", err)
	} else {
		fmt.Printf("\nPage object references:\n")
		for i, ref := range pageRefs {
			fmt.Printf("  Page %d: Object %d\n", i+1, ref)
		}
	}

	// Try to parse as template to get fields
	template, err := pdffill.New(pdfData)
	if err == nil {
		fields := template.FieldNames()
		fmt.Printf("\nForm fields: %d\n", len(fields))
		if len(fields) > 0 && len(fields) <= 10 {
			fmt.Println("Field names:")
			for _, name := range fields {
				fmt.Printf("  - %s\n", name)
			}
		} else if len(fields) > 10 {
			fmt.Println("Field names (first 10):")
			for i := 0; i < 10; i++ {
				fmt.Printf("  - %s\n", fields[i])
			}
			fmt.Printf("  ... and %d more\n", len(fields)-10)
		}
	}

	fmt.Println()
}
