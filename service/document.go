package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"ai-knowledge-assistant/Repository"
	"ai-knowledge-assistant/Utils"
	"ai-knowledge-assistant/models"
)

type DocumentService struct {
	Repo        repository.Repository
	StoragePath string
}

func (s DocumentService) Upload(ctx context.Context, org string, file *multipart.FileHeader) (models.Document, error) {
	if file == nil {
		return models.Document{}, fmt.Errorf("PDF file is required")
	}
	if file.Size > utils.MaxPDFBytes {
		return models.Document{}, fmt.Errorf("PDF exceeds 10 MB")
	}
	n, err := s.Repo.DocumentCount(ctx, org)
	if err != nil {
		return models.Document{}, err
	}
	if n >= 10 {
		return models.Document{}, fmt.Errorf("document limit reached")
	}
	if err = os.MkdirAll(s.StoragePath, 0750); err != nil {
		return models.Document{}, err
	}
	raw, err := utils.NewToken()
	if err != nil {
		return models.Document{}, err
	}
	key := raw + ".pdf"
	dest := filepath.Join(s.StoragePath, key)
	in, err := file.Open()
	if err != nil {
		return models.Document{}, err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return models.Document{}, err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(dest)
		if copyErr != nil {
			return models.Document{}, copyErr
		}
		return models.Document{}, closeErr
	}
	if !utils.IsPDF(dest) {
		_ = os.Remove(dest)
		return models.Document{}, fmt.Errorf("file is not a valid PDF")
	}
	pages, err := utils.ExtractPDF(dest)
	if err != nil {
		_ = os.Remove(dest)
		return models.Document{}, err
	}
	if len(pages) > utils.MaxPDFPages {
		_ = os.Remove(dest)
		return models.Document{}, fmt.Errorf("PDF exceeds 20 pages")
	}
	id, err := s.Repo.CreateDocument(ctx, org, file.Filename, key, file.Size, len(pages), "processing")
	if err != nil {
		_ = os.Remove(dest)
		return models.Document{}, err
	}
	pieces := utils.ChunkPages(pages)
	chunks := make([]models.Chunk, 0, len(pieces))
	for _, p := range pieces {
		if p.Content != "" {
			chunks = append(chunks, models.Chunk{Page: p.Page, Content: p.Content})
		}
	}
	if len(chunks) == 0 {
		_ = s.Repo.SetDocumentStatus(ctx, id, org, "failed")
		return models.Document{}, fmt.Errorf("PDF contains no extractable text")
	}
	if err = s.Repo.AddChunks(ctx, org, id, chunks); err != nil {
		_ = s.Repo.SetDocumentStatus(ctx, id, org, "failed")
		return models.Document{}, err
	}
	if err = s.Repo.SetDocumentStatus(ctx, id, org, "ready"); err != nil {
		return models.Document{}, err
	}
	return models.Document{ID: id, Filename: file.Filename, SizeBytes: file.Size, PageCount: len(pages), Status: "ready"}, nil
}
func (s DocumentService) Delete(ctx context.Context, org, id string) error {
	key, err := s.Repo.DeleteDocument(ctx, org, id)
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(s.StoragePath, key))
}
