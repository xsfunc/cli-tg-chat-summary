package app

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeLines(t *testing.T) {
	input := "  first line\r\n\r\n second line \n\n\tthird line\r\n"
	expected := []string{"first line", "second line", "third line"}

	got := normalizeLines(input)
	if len(got) != len(expected) {
		t.Fatalf("expected %d lines, got %d", len(expected), len(got))
	}
	for i, line := range expected {
		if got[i] != line {
			t.Fatalf("line %d: expected %q, got %q", i, line, got[i])
		}
	}
}

func TestBuildMessageBlocks_CollapseBySender(t *testing.T) {
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	messages := []TemplateMessage{
		{SenderID: 1, Date: base.Add(0 * time.Minute), Text: "a"},
		{SenderID: 1, Date: base.Add(1 * time.Minute), Text: "b\n\nc"},
		{SenderID: 2, Date: base.Add(2 * time.Minute), Text: "x"},
		{SenderID: 1, Date: base.Add(3 * time.Minute), Text: "d"},
	}

	blocks := buildMessageBlocks(messages)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}

	if blocks[0].SenderID != 1 {
		t.Fatalf("block 0 sender: expected 1, got %d", blocks[0].SenderID)
	}
	if blocks[0].Start != base || blocks[0].End != base.Add(1*time.Minute) {
		t.Fatalf("block 0 time range mismatch: %v - %v", blocks[0].Start, blocks[0].End)
	}
	if strings.Join(blocks[0].Lines, "|") != "a|b|c" {
		t.Fatalf("block 0 lines mismatch: %v", blocks[0].Lines)
	}

	if blocks[1].SenderID != 2 || len(blocks[1].Lines) != 1 || blocks[1].Lines[0] != "x" {
		t.Fatalf("block 1 content mismatch: %+v", blocks[1])
	}

	if blocks[2].SenderID != 1 || len(blocks[2].Lines) != 1 || blocks[2].Lines[0] != "d" {
		t.Fatalf("block 2 content mismatch: %+v", blocks[2])
	}
}

func TestWriteMessageBlocks_Format(t *testing.T) {
	start := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	blocks := []messageBlock{
		{
			SenderID: 0,
			Start:    start,
			End:      start,
			Lines:    []string{"hello"},
		},
		{
			SenderID: 42,
			Start:    start.Add(5 * time.Minute),
			End:      start.Add(7 * time.Minute),
			Lines:    []string{"line one", "line two"},
		},
	}

	var b strings.Builder
	if err := writeMessageBlocks(&b, blocks); err != nil {
		t.Fatalf("writeMessageBlocks error: %v", err)
	}

	expected := strings.Join([]string{
		"[09:00] id=unknown:",
		"  hello",
		"[09:05-09:07] id=42:",
		"  line one",
		"  line two",
		"",
	}, "\n")

	if b.String() != expected {
		t.Fatalf("unexpected output:\n%q\nwant:\n%q", b.String(), expected)
	}
}
