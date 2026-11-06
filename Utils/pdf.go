package utils

import (
	"fmt"
	"os"
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
		pages = append(pages, strings.TrimSpace(text))
	}
	return pages, nil
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
