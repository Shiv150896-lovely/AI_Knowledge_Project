package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

const MaxPDFBytes int64 = 10 * 1024 * 1024
const MaxPDFPages = 20

func ExtractPDF(path string) ([]string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("invalid PDF: %w", err)
	}
	defer f.Close()
	if r.NumPage() > MaxPDFPages {
		return nil, fmt.Errorf("PDF exceeds %d pages", MaxPDFPages)
	}
	pages := make([]string, 0, r.NumPage())
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			pages = append(pages, "")
			continue
		}
		text, e := p.GetPlainText(nil)
		if e != nil {
			return nil, fmt.Errorf("extract page %d: %w", i, e)
		}
		if strings.TrimSpace(text) == "" {
			text = extractWithPDFToText(path, i)
		}
		// PDF text streams may contain malformed character mappings. PostgreSQL
		// accepts UTF-8 text only, so replace invalid byte sequences before chunks
		// are persisted while preserving all valid extracted content.
		text = strings.ToValidUTF8(text, "\uFFFD")
		pages = append(pages, strings.TrimSpace(text))
	}
	return pages, nil
}

// extractWithPDFToText is a compatibility fallback for valid PDFs whose text
// encoding is not exposed by the pure-Go reader. It is optional: if Poppler's
// pdftotext executable is unavailable, the caller continues with the Go result.
func extractWithPDFToText(path string, page int) string {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return ""
	}
	pageArg := strconv.Itoa(page)
	output, err := exec.Command("pdftotext", "-f", pageArg, "-l", pageArg, "-layout", path, "-").Output()
	if err != nil {
		return ""
	}
	return string(output)
}
func IsPDF(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	b := make([]byte, 5)
	n, _ := f.Read(b)
	return n == 5 && string(b) == "%PDF-"
}
func ChunkPages(pages []string) []struct {
	Page    int
	Content string
} {
	out := []struct {
		Page    int
		Content string
	}{}
	for i, p := range pages {
		p = strings.TrimSpace(p)
		for len(p) > 0 {
			end := 1200
			if len(p) <= end {
				out = append(out, struct {
					Page    int
					Content string
				}{i + 1, p})
				break
			}
			cut := strings.LastIndex(p[:end], " ")
			if cut < 300 {
				cut = end
			}
			out = append(out, struct {
				Page    int
				Content string
			}{i + 1, p[:cut]})
			p = strings.TrimSpace(p[cut:])
		}
	}
	return out
}
