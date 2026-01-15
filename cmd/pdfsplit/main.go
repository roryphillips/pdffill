package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/roryphillips/pdffill"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <pdf-file> [output-dir]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nSplits a PDF into individual pages.\n")
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s input.pdf output/\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s input.pdf         (outputs to ./input_pages/)\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := os.Args[1]

	// Determine output directory
	var outputDir string
	if len(os.Args) >= 3 {
		outputDir = os.Args[2]
	} else {
		// Default: create directory based on input filename
		baseName := strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile))
		outputDir = baseName + "_pages"
	}

	// Read input PDF
	fmt.Printf("Reading %s...\n", inputFile)
	pdfData, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Failed to read PDF: %v", err)
	}

	// Count pages
	pageCount, err := pdffill.CountPages(pdfData)
	if err != nil {
		log.Fatalf("Failed to count pages: %v", err)
	}

	fmt.Printf("Found %d pages\n", pageCount)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	fmt.Printf("Splitting into %s/\n", outputDir)

	// Split pages
	pages, err := pdffill.SplitPages(pdfData)
	if err != nil {
		log.Fatalf("Failed to split pages: %v", err)
	}

	// Write individual pages
	for i, pageData := range pages {
		pageNum := i + 1
		outputFile := filepath.Join(outputDir, fmt.Sprintf("page_%03d.pdf", pageNum))

		if err := os.WriteFile(outputFile, pageData, 0644); err != nil {
			log.Fatalf("Failed to write page %d: %v", pageNum, err)
		}

		fmt.Printf("  [%3d/%3d] %s (%d bytes)\n", pageNum, pageCount, outputFile, len(pageData))
	}

	fmt.Printf("\n✓ Successfully split %d pages to %s/\n", pageCount, outputDir)
}
