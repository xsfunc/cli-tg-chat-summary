package app

import (
	"fmt"
	"io"
	"time"
)

type textTemplate struct{}

func NewTextTemplate() Template {
	return textTemplate{}
}

func (t textTemplate) Name() string {
	return "text"
}

func (t textTemplate) Extension() string {
	return "txt"
}

func (t textTemplate) Render(w io.Writer, input TemplateInput) error {
	if _, err := fmt.Fprintf(w, "Chat Summary: %s\n", input.ExportTitle); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := fmt.Fprintf(w, "Export Date: %s\n", input.ExportDate.Format(time.RFC1123)); err != nil {
		return fmt.Errorf("failed to write date: %w", err)
	}
	if _, err := fmt.Fprintf(w, "Total Messages: %d\n\n", input.TotalMessages); err != nil {
		return fmt.Errorf("failed to write count: %w", err)
	}

	blocks := buildMessageBlocks(input.Messages)
	if err := writeMessageBlocks(w, blocks); err != nil {
		return fmt.Errorf("failed to write message blocks: %w", err)
	}
	return nil
}
