package app

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

type xmlCompactTemplate struct{}

func NewXMLCompactTemplate() Template {
	return xmlCompactTemplate{}
}

func (t xmlCompactTemplate) Name() string {
	return "xml-compact"
}

func (t xmlCompactTemplate) Extension() string {
	return "xml"
}

func (t xmlCompactTemplate) Render(w io.Writer, input TemplateInput) error {
	doc := xmlCompactChat{
		Title:         input.ExportTitle,
		ExportDate:    input.ExportDate.Format(time.RFC3339),
		TotalMessages: input.TotalMessages,
	}
	if input.Options.UseDateRange {
		since := input.Options.Since.Format(time.RFC3339)
		until := input.Options.Until.Format(time.RFC3339)
		doc.Since = &since
		doc.Until = &until
	}

	for _, msg := range input.Messages {
		lines := normalizeLines(msg.Text)
		if len(lines) == 0 {
			continue
		}
		text := strings.Join(lines, "\n")
		xmlMsg := xmlCompactMessage{
			SenderID:   msg.SenderID,
			SenderName: msg.SenderName,
			Time:       msg.Date.Format(time.RFC3339),
			Text:       text,
		}

		if msg.ReplyTo != nil {
			xmlMsg.Reply = &xmlCompactReply{
				MessageID: msg.ReplyTo.MessageID,
				SenderID:  msg.ReplyTo.SenderID,
				Name:      msg.ReplyTo.SenderName,
				Text:      msg.ReplyTo.Text,
			}
		}
		if len(msg.Reactions) > 0 {
			reactions := make([]xmlCompactReaction, 0, len(msg.Reactions))
			for _, reaction := range msg.Reactions {
				reactions = append(reactions, xmlCompactReaction(reaction))
			}
			xmlMsg.Reactions = &xmlCompactReactions{Items: reactions}
		}
		doc.Messages = append(doc.Messages, xmlMsg)
	}

	exporter := xml.NewEncoder(w)
	if err := exporter.Encode(doc); err != nil {
		return fmt.Errorf("failed to write xml compact: %w", err)
	}
	if err := exporter.Flush(); err != nil {
		return fmt.Errorf("failed to flush xml compact: %w", err)
	}
	return nil
}

type xmlCompactChat struct {
	XMLName       xml.Name            `xml:"c"`
	Title         string              `xml:"t,attr"`
	ExportDate    string              `xml:"d,attr"`
	TotalMessages int                 `xml:"n,attr"`
	Since         *string             `xml:"s,attr,omitempty"`
	Until         *string             `xml:"u,attr,omitempty"`
	Messages      []xmlCompactMessage `xml:"m"`
}

type xmlCompactMessage struct {
	Time       string               `xml:"t,attr"`
	SenderID   int64                `xml:"s,attr"`
	SenderName string               `xml:"n,attr,omitempty"`
	Text       string               `xml:",chardata"`
	Reply      *xmlCompactReply     `xml:"r,omitempty"`
	Reactions  *xmlCompactReactions `xml:"rx,omitempty"`
}

type xmlCompactReply struct {
	MessageID int    `xml:"i,attr,omitempty"`
	SenderID  int64  `xml:"s,attr"`
	Name      string `xml:"n,attr,omitempty"`
	Text      string `xml:",chardata"`
}

type xmlCompactReactions struct {
	Items []xmlCompactReaction `xml:"x"`
}

type xmlCompactReaction struct {
	Emoji string `xml:"e,attr"`
	Count int    `xml:"c,attr"`
}
