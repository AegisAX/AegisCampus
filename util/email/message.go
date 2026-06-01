// Package emailparser parses raw RFC 5322 + MIME multipart messages
// into a Message struct with the fields AegisCampus needs (From, To, Subject,
// Headers, Text, HTML, Attachments).
package emailparser

import (
	"net/textproto"
)

// Message represents a parsed email.
type Message struct {
	From        string
	To          []string
	Subject     string
	Headers     textproto.MIMEHeader
	Text        []byte
	HTML        []byte
	Attachments []*Attachment
}

// Attachment represents a single attachment in a Message.
type Attachment struct {
	Filename    string
	ContentType string
	Header      textproto.MIMEHeader
	Content     []byte
}
