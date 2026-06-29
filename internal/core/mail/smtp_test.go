package mail

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	netmail "net/mail"
	"strings"
	"testing"
)

func TestSMTPMailerBuildMultipart(t *testing.T) {
	m := NewSMTPMailer("localhost", 1025, "", "", "noreply@skolva.org")
	built := m.build(Message{
		To:      []string{"x@example.com"},
		Subject: "Hi",
		Body:    "plain text body",
		HTML:    "<p>html body</p>",
	})

	msg, err := netmail.ReadMessage(strings.NewReader(string(built)))
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}

	mt, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	if mt != "multipart/alternative" {
		t.Fatalf("expected multipart/alternative, got %s", mt)
	}

	mr := multipart.NewReader(msg.Body, params["boundary"])
	var types, bodies []string
	for {
		part, err := mr.NextPart()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("next part: %v", err)
		}
		pmt, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		b, _ := io.ReadAll(part)
		types = append(types, pmt)
		bodies = append(bodies, string(b))
	}

	if len(types) != 2 || types[0] != "text/plain" || types[1] != "text/html" {
		t.Fatalf("unexpected parts: %v", types)
	}
	if !strings.Contains(bodies[0], "plain text body") {
		t.Errorf("missing plain text part: %q", bodies[0])
	}
	if !strings.Contains(bodies[1], "<p>html body</p>") {
		t.Errorf("missing html part: %q", bodies[1])
	}
}

func TestSMTPMailerBuildHTMLOnly(t *testing.T) {
	m := NewSMTPMailer("localhost", 1025, "", "", "noreply@skolva.org")
	out := string(m.build(Message{To: []string{"x@example.com"}, Subject: "S", HTML: "<h1>Hi</h1>"}))

	if !strings.Contains(out, "Content-Type: text/html; charset=\"utf-8\"") {
		t.Errorf("expected text/html content type, got:\n%s", out)
	}
	if !strings.HasSuffix(out, "\r\n\r\n<h1>Hi</h1>") {
		t.Errorf("expected html body at end, got:\n%s", out)
	}
}
