package app

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type messageBlock struct {
	SenderID int64
	Start    time.Time
	End      time.Time
	Lines    []string
}

func formatSenderID(id int64) string {
	if id <= 0 {
		return "id=unknown"
	}
	return fmt.Sprintf("id=%d", id)
}

func normalizeLines(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return nil
	}
	rawLines := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func buildMessageBlocks(messages []TemplateMessage) []messageBlock {
	var blocks []messageBlock
	for _, msg := range messages {
		lines := normalizeLines(msg.Text)
		if len(lines) == 0 {
			continue
		}
		if len(blocks) == 0 || blocks[len(blocks)-1].SenderID != msg.SenderID {
			blocks = append(blocks, messageBlock{
				SenderID: msg.SenderID,
				Start:    msg.Date,
				End:      msg.Date,
				Lines:    lines,
			})
			continue
		}
		last := &blocks[len(blocks)-1]
		last.End = msg.Date
		last.Lines = append(last.Lines, lines...)
	}
	return blocks
}

func writeMessageBlocks(w io.Writer, blocks []messageBlock) error {
	for _, block := range blocks {
		start := block.Start.Format("15:04")
		end := block.End.Format("15:04")
		timeLabel := start
		if end != start {
			timeLabel = fmt.Sprintf("%s-%s", start, end)
		}
		if _, err := fmt.Fprintf(w, "[%s] %s:\n", timeLabel, formatSenderID(block.SenderID)); err != nil {
			return err
		}
		for _, line := range block.Lines {
			if _, err := fmt.Fprintf(w, "  %s\n", line); err != nil {
				return err
			}
		}
	}
	return nil
}
