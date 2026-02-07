package app

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

type xmlTemplate struct{}

func NewXMLTemplate() Template {
	return xmlTemplate{}
}

func (t xmlTemplate) Name() string {
	return "xml"
}

func (t xmlTemplate) Extension() string {
	return "xml"
}

func (t xmlTemplate) Render(w io.Writer, input TemplateInput) error {
	doc := xmlChat{
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
		xmlMsg := xmlMessage{
			Sender: xmlSender{
				ID:   msg.SenderID,
				Name: msg.SenderName,
			},
			Time: msg.Date.Format(time.RFC3339),
			Text: text,
		}

		if msg.ReplyTo != nil {
			xmlMsg.Reply = &xmlReply{
				MessageID: msg.ReplyTo.MessageID,
				Sender: xmlSender{
					ID:   msg.ReplyTo.SenderID,
					Name: msg.ReplyTo.SenderName,
				},
				Text: msg.ReplyTo.Text,
			}
		}
		if len(msg.Reactions) > 0 {
			reactions := make([]xmlReaction, 0, len(msg.Reactions))
			for _, reaction := range msg.Reactions {
				reactions = append(reactions, xmlReaction(reaction))
			}
			xmlMsg.Reactions = &xmlReactions{Items: reactions}
		}
		doc.Messages = append(doc.Messages, xmlMsg)
	}

	exporter := xml.NewEncoder(w)
	exporter.Indent("", "  ")
	if err := exporter.Encode(doc); err != nil {
		return fmt.Errorf("failed to write xml: %w", err)
	}
	if err := exporter.Flush(); err != nil {
		return fmt.Errorf("failed to flush xml: %w", err)
	}
	return nil
}

type xmlChat struct {
	XMLName       xml.Name     `xml:"chat"`
	Title         string       `xml:"title,attr"`
	ExportDate    string       `xml:"export_date"`
	TotalMessages int          `xml:"total_messages"`
	Since         *string      `xml:"since,omitempty"`
	Until         *string      `xml:"until,omitempty"`
	Messages      []xmlMessage `xml:"message"`
}

type xmlMessage struct {
	Sender    xmlSender     `xml:"sender"`
	Time      string        `xml:"time"`
	Text      string        `xml:"text"`
	Reply     *xmlReply     `xml:"reply,omitempty"`
	Reactions *xmlReactions `xml:"reactions,omitempty"`
}

type xmlSender struct {
	ID   int64  `xml:"id,attr"`
	Name string `xml:"name,omitempty"`
}

type xmlReply struct {
	MessageID int       `xml:"message_id,attr,omitempty"`
	Sender    xmlSender `xml:"sender"`
	Text      string    `xml:"text,omitempty"`
}

type xmlReactions struct {
	Items []xmlReaction `xml:"reaction"`
}

type xmlReaction struct {
	Emoji string `xml:"emoji,attr"`
	Count int    `xml:"count,attr"`
}
