package pdffill

import (
	"bytes"
	"fmt"
)

// ExtractPage extracts a single page from a PDF and returns it as a new PDF.
//
// This creates a completely independent PDF containing only the specified page,
// with all necessary resources (fonts, images, etc.) copied over.
//
// Page numbers are 1-indexed (page 1 is the first page).
//
// Example:
//
//	// Extract page 3 from a PDF
//	pdfData, _ := os.ReadFile("input.pdf")
//	page3, err := pdffill.ExtractPage(pdfData, 3)
//	if err != nil {
//		log.Fatal(err)
//	}
//	os.WriteFile("page3.pdf", page3, 0644)
//
// Returns an error if:
//   - pageNum is less than 1
//   - pageNum exceeds the number of pages in the PDF
//   - the PDF structure is invalid
//
// Note: This operation can be slow for complex PDFs with many embedded resources.
func ExtractPage(pdfData []byte, pageNum int) ([]byte, error) {
	if pageNum < 1 {
		return nil, fmt.Errorf("page number must be >= 1")
	}

	// Extract all page references from the PDF (handles nested page trees)
	pageRefs, err := ExtractPageRefs(pdfData)
	if err != nil {
		return nil, fmt.Errorf("extract page refs: %w", err)
	}

	if pageNum > len(pageRefs) {
		return nil, fmt.Errorf("page %d out of range (PDF has %d pages)", pageNum, len(pageRefs))
	}

	// Get the specific page object reference (0-indexed internally)
	targetPageRef := pageRefs[pageNum-1]

	// Extract all objects needed for this page
	objects := make(map[int][]byte)
	objectsToExtract := []int{targetPageRef}
	extracted := make(map[int]bool)

	for len(objectsToExtract) > 0 {
		objNum := objectsToExtract[0]
		objectsToExtract = objectsToExtract[1:]

		if extracted[objNum] {
			continue
		}

		objContent, err := getObjectContent(pdfData, objNum)
		if err != nil {
			continue
		}

		objects[objNum] = objContent
		extracted[objNum] = true

		// Find all references in this object
		refs := findAllReferences(objContent)
		for _, ref := range refs {
			if !extracted[ref] {
				objectsToExtract = append(objectsToExtract, ref)
			}
		}
	}

	// Build new PDF with just this page
	return buildSinglePagePDF(pdfData, targetPageRef, objects)
}

// SplitPages splits a PDF into individual pages, returning one PDF per page.
//
// This is a convenience function that calls ExtractPage for each page in the PDF.
// Each returned PDF is a complete, independent document.
//
// Example:
//
//	pdfData, _ := os.ReadFile("multi-page.pdf")
//	pages, err := pdffill.SplitPages(pdfData)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	for i, pageData := range pages {
//		filename := fmt.Sprintf("page_%d.pdf", i+1)
//		os.WriteFile(filename, pageData, 0644)
//	}
//
// Returns an error if page counting or extraction fails.
//
// Note: This operation can be slow for large PDFs with many pages.
// Consider using ExtractPage directly if you only need specific pages.
func SplitPages(pdfData []byte) ([][]byte, error) {
	pageCount, err := CountPages(pdfData)
	if err != nil {
		return nil, fmt.Errorf("count pages: %w", err)
	}

	pages := make([][]byte, pageCount)
	for i := 1; i <= pageCount; i++ {
		page, err := ExtractPage(pdfData, i)
		if err != nil {
			return nil, fmt.Errorf("extract page %d: %w", i, err)
		}
		pages[i-1] = page
	}

	return pages, nil
}

// CountPages returns the number of pages in a PDF.
//
// This reads the /Count entry from the PDF's page tree to determine
// the total number of pages.
//
// Example:
//
//	pdfData, _ := os.ReadFile("document.pdf")
//	count, err := pdffill.CountPages(pdfData)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("PDF has %d pages\n", count)
//
// Returns an error if the PDF structure is invalid or the page count
// cannot be determined.
func CountPages(pdfData []byte) (int, error) {
	return countPages(pdfData)
}

// findPagesObject locates the Pages dictionary in the PDF.
func findPagesObject(pdfData []byte) ([]byte, error) {
	// Find the catalog
	catalogIdx := bytes.Index(pdfData, []byte("/Type /Catalog"))
	if catalogIdx == -1 {
		return nil, fmt.Errorf("catalog not found")
	}

	// Work backwards to find the object start
	objStart := catalogIdx
	for objStart > 0 && !bytes.Equal(pdfData[objStart:objStart+4], []byte(" obj")) {
		objStart--
	}
	if objStart == 0 {
		return nil, fmt.Errorf("catalog object start not found")
	}

	// Find /Pages reference in catalog
	endObj := bytes.Index(pdfData[catalogIdx:], []byte("endobj"))
	if endObj == -1 {
		return nil, fmt.Errorf("catalog endobj not found")
	}

	catalogContent := pdfData[catalogIdx : catalogIdx+endObj]
	pagesRef := extractReference(catalogContent, "/Pages")
	if pagesRef == 0 {
		return nil, fmt.Errorf("pages reference not found in catalog")
	}

	// Get the Pages object
	return getObjectContent(pdfData, pagesRef)
}

// extractPageRefs extracts all page object references from the Pages tree.
// This recursively walks the page tree to handle nested Pages objects.
func extractPageRefs(pagesContent []byte) ([]int, error) {
	kidsIdx := bytes.Index(pagesContent, []byte("/Kids"))
	if kidsIdx == -1 {
		return nil, fmt.Errorf("Kids array not found")
	}

	arrayStart := bytes.IndexByte(pagesContent[kidsIdx:], '[')
	if arrayStart == -1 {
		return nil, fmt.Errorf("Kids array opening bracket not found")
	}
	arrayStart += kidsIdx

	arrayEnd := bytes.IndexByte(pagesContent[arrayStart:], ']')
	if arrayEnd == -1 {
		return nil, fmt.Errorf("Kids array closing bracket not found")
	}
	arrayEnd += arrayStart

	arrayContent := pagesContent[arrayStart+1 : arrayEnd]
	return parseReferences(arrayContent), nil
}

// ExtractPageRefs extracts all page object references from a PDF.
//
// This function handles nested page trees, which are used in PDFs with many pages
// to organize the page hierarchy. It returns the PDF object numbers for all
// Page objects (leaf nodes in the page tree).
//
// The returned slice contains object numbers in page order (page 1, page 2, etc.).
//
// Example:
//
//	pdfData, _ := os.ReadFile("document.pdf")
//	pageRefs, err := pdffill.ExtractPageRefs(pdfData)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Page 1 is object %d\n", pageRefs[0])
//	fmt.Printf("Page 2 is object %d\n", pageRefs[1])
//
// This is primarily used internally by ExtractPage and SplitPages, but is
// exported for advanced use cases.
//
// Returns an error if the PDF structure is invalid or the page tree cannot be parsed.
func ExtractPageRefs(pdfData []byte) ([]int, error) {
	pagesObj, err := findPagesObject(pdfData)
	if err != nil {
		return nil, err
	}

	initialRefs, err := extractPageRefs(pagesObj)
	if err != nil {
		return nil, err
	}

	// Recursively expand any Pages objects (not Page objects)
	var allPageRefs []int
	toExpand := initialRefs

	for len(toExpand) > 0 {
		ref := toExpand[0]
		toExpand = toExpand[1:]

		objContent, err := getObjectContent(pdfData, ref)
		if err != nil {
			continue
		}

		// Check if this is a Pages object (intermediate node) or Page object (leaf)
		if bytes.Contains(objContent, []byte("/Type /Pages")) {
			// It's a Pages object, extract its kids
			kids, err := extractPageRefs(objContent)
			if err != nil {
				continue
			}
			toExpand = append(toExpand, kids...)
		} else {
			// It's a Page object (or other), add to results
			allPageRefs = append(allPageRefs, ref)
		}
	}

	return allPageRefs, nil
}

// getObjectContent retrieves the content of a PDF object by number.
func getObjectContent(pdfData []byte, objNum int) ([]byte, error) {
	pattern := []byte(fmt.Sprintf("\n%d 0 obj", objNum))
	idx := bytes.LastIndex(pdfData, pattern)
	if idx == -1 {
		pattern = []byte(fmt.Sprintf("%d 0 obj", objNum))
		idx = bytes.Index(pdfData, pattern)
		if idx == -1 || (idx > 0 && isDigit(pdfData[idx-1])) {
			return nil, fmt.Errorf("object %d not found", objNum)
		}
	} else {
		idx++
	}

	objStart := idx + len(pattern) - 1
	if pdfData[idx] != '\n' {
		objStart = idx + len(pattern)
	}

	endObj := bytes.Index(pdfData[objStart:], []byte("endobj"))
	if endObj == -1 {
		return nil, fmt.Errorf("endobj not found for object %d", objNum)
	}

	return pdfData[objStart : objStart+endObj], nil
}

// findAllReferences finds all object references in content.
func findAllReferences(content []byte) []int {
	var refs []int
	seen := make(map[int]bool)

	pos := 0
	for pos < len(content) {
		// Look for pattern "N 0 R"
		numStart := -1
		for i := pos; i < len(content); i++ {
			if isDigit(content[i]) && (i == 0 || isWhitespace(content[i-1]) || content[i-1] == '[' || content[i-1] == '/') {
				numStart = i
				break
			}
		}

		if numStart == -1 {
			break
		}

		numEnd := numStart
		for numEnd < len(content) && isDigit(content[numEnd]) {
			numEnd++
		}

		// Check if followed by " 0 R"
		if numEnd+4 <= len(content) &&
			content[numEnd] == ' ' &&
			content[numEnd+1] == '0' &&
			content[numEnd+2] == ' ' &&
			content[numEnd+3] == 'R' {

			num, _ := parseInt(content[numStart:numEnd])
			if !seen[num] && num > 0 {
				refs = append(refs, num)
				seen[num] = true
			}
			pos = numEnd + 4
		} else {
			pos = numEnd
		}
	}

	return refs
}

// buildSinglePagePDF constructs a new PDF with just one page.
func buildSinglePagePDF(originalPDF []byte, pageObjNum int, objects map[int][]byte) ([]byte, error) {
	var buf bytes.Buffer

	// Write PDF header
	headerEnd := bytes.IndexByte(originalPDF, '\n')
	if headerEnd == -1 {
		return nil, fmt.Errorf("invalid PDF header")
	}
	buf.Write(originalPDF[:headerEnd+1])

	// Track object offsets for xref
	xrefEntries := make(map[int]int)
	objNumMapping := make(map[int]int) // old -> new
	nextObjNum := 1

	// Assign new object numbers
	for oldNum := range objects {
		objNumMapping[oldNum] = nextObjNum
		nextObjNum++
	}

	// Write all objects with new numbering and updated references
	for oldNum, content := range objects {
		newNum := objNumMapping[oldNum]
		xrefEntries[newNum] = buf.Len()

		// Update references in content
		updatedContent := renumberReferencesInContent(content, objNumMapping)

		buf.WriteString(fmt.Sprintf("%d 0 obj\n", newNum))
		buf.Write(updatedContent)
		buf.WriteString("\nendobj\n")
	}

	// Create new Pages tree with single page
	pagesObjNum := nextObjNum
	nextObjNum++
	xrefEntries[pagesObjNum] = buf.Len()

	buf.WriteString(fmt.Sprintf("%d 0 obj\n", pagesObjNum))
	buf.WriteString("<< /Type /Pages\n")
	buf.WriteString(fmt.Sprintf("/Kids [ %d 0 R ]\n", objNumMapping[pageObjNum]))
	buf.WriteString("/Count 1\n")
	buf.WriteString(">>\nendobj\n")

	// Create new Catalog
	catalogObjNum := nextObjNum
	nextObjNum++
	xrefEntries[catalogObjNum] = buf.Len()

	buf.WriteString(fmt.Sprintf("%d 0 obj\n", catalogObjNum))
	buf.WriteString("<< /Type /Catalog\n")
	buf.WriteString(fmt.Sprintf("/Pages %d 0 R\n", pagesObjNum))
	buf.WriteString(">>\nendobj\n")

	// Write xref table
	xrefOffset := buf.Len()
	if err := writeXRef(&buf, xrefEntries); err != nil {
		return nil, fmt.Errorf("write xref: %w", err)
	}

	// Write trailer
	buf.WriteString("trailer\n")
	buf.WriteString(fmt.Sprintf("<< /Size %d\n", nextObjNum))
	buf.WriteString(fmt.Sprintf("/Root %d 0 R\n", catalogObjNum))
	buf.WriteString(">>\n")
	buf.WriteString("startxref\n")
	buf.WriteString(fmt.Sprintf("%d\n", xrefOffset))
	buf.WriteString("%%EOF\n")

	return buf.Bytes(), nil
}

// renumberReferencesInContent updates object references to use new numbering.
func renumberReferencesInContent(content []byte, mapping map[int]int) []byte {
	var result bytes.Buffer
	pos := 0

	for pos < len(content) {
		// Find next reference
		numStart := -1
		for i := pos; i < len(content); i++ {
			if isDigit(content[i]) && (i == 0 || isWhitespace(content[i-1]) || content[i-1] == '[' || content[i-1] == '/') {
				numStart = i
				break
			}
			result.WriteByte(content[i])
		}

		if numStart == -1 {
			break
		}

		numEnd := numStart
		for numEnd < len(content) && isDigit(content[numEnd]) {
			numEnd++
		}

		// Check if it's a reference
		if numEnd+4 <= len(content) &&
			content[numEnd] == ' ' &&
			content[numEnd+1] == '0' &&
			content[numEnd+2] == ' ' &&
			content[numEnd+3] == 'R' {

			oldNum, _ := parseInt(content[numStart:numEnd])
			if newNum, exists := mapping[oldNum]; exists {
				result.WriteString(fmt.Sprintf("%d 0 R", newNum))
			} else {
				result.Write(content[numStart : numEnd+4])
			}
			pos = numEnd + 4
		} else {
			result.Write(content[numStart:numEnd])
			pos = numEnd
		}
	}

	return result.Bytes()
}
