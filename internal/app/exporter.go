package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"cli-tg-chat-summary/internal/telegram"
)

type Exporter interface {
	Export(exportTitle string, messages []telegram.Message, opts RunOptions) (string, error)
}

type DefaultExporter struct {
	Now      func() time.Time
	Getwd    func() (string, error)
	MkdirAll func(path string, perm os.FileMode) error
	Create   func(path string) (io.WriteCloser, error)
}

func NewDefaultExporter() *DefaultExporter {
	return &DefaultExporter{
		Now:      time.Now,
		Getwd:    os.Getwd,
		MkdirAll: os.MkdirAll,
		Create: func(path string) (io.WriteCloser, error) {
			return os.Create(path)
		},
	}
}

func (e *DefaultExporter) Export(exportTitle string, messages []telegram.Message, opts RunOptions) (string, error) {
	// format: ChatName_Date.txt or ChatName_TopicName_Date.txt
	// date range format: ChatName_YYYY-MM-DD_to_YYYY-MM-DD.txt
	cleanName := sanitizeFilename(exportTitle)
	var suffix string
	if opts.UseDateRange {
		suffix = fmt.Sprintf("%s_to_%s", opts.Since.Format("2006-01-02"), opts.Until.Format("2006-01-02"))
	} else {
		suffix = e.Now().Format("2006-01-02")
	}
	filename := fmt.Sprintf("exports/%s_%s.txt", cleanName, suffix)

	cwd, err := e.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	if err := e.MkdirAll(filepath.Join(cwd, "exports"), 0755); err != nil {
		return "", fmt.Errorf("failed to create exports directory: %w", err)
	}

	fullPath := filepath.Join(cwd, filename)
	f, err := e.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	if _, err := fmt.Fprintf(f, "Chat Summary: %s\n", exportTitle); err != nil {
		return "", fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := fmt.Fprintf(f, "Export Date: %s\n", e.Now().Format(time.RFC1123)); err != nil {
		return "", fmt.Errorf("failed to write date: %w", err)
	}
	if _, err := fmt.Fprintf(f, "Total Messages: %d\n\n", len(messages)); err != nil {
		return "", fmt.Errorf("failed to write count: %w", err)
	}

	blocks := buildMessageBlocks(messages)
	if err := writeMessageBlocks(f, blocks); err != nil {
		return "", fmt.Errorf("failed to write message blocks: %w", err)
	}

	return filename, nil
}
