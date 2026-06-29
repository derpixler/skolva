package mail

import (
	"context"
	"strings"
	"testing"
)

func TestNoopMailerRecordsMessages(t *testing.T) {
	var m Mailer = NewNoopMailer()

	msg := Message{To: []string{"a@example.com"}, Subject: "Hi", Body: "Body"}
	if err := m.Send(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	noop, ok := m.(*NoopMailer)
	if !ok {
		t.Fatal("expected *NoopMailer")
	}
	sent := noop.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].Subject != "Hi" {
		t.Errorf("unexpected subject: %s", sent[0].Subject)
	}
}

func TestSMTPMailerImplementsMailer(t *testing.T) {
	var _ Mailer = NewSMTPMailer("localhost", 1025, "", "", "noreply@skolva.org")
}

func TestSMTPMailerBuildMessage(t *testing.T) {
	m := NewSMTPMailer("localhost", 1025, "", "", "noreply@skolva.org")
	out := string(m.build(Message{
		To:      []string{"x@example.com", "y@example.com"},
		Subject: "Test Subject",
		Body:    "Hello World",
	}))

	for _, want := range []string{
		"From: noreply@skolva.org\r\n",
		"To: x@example.com, y@example.com\r\n",
		"Subject: Test Subject\r\n",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=\"utf-8\"\r\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected message to contain %q", want)
		}
	}
	if !strings.HasSuffix(out, "\r\n\r\nHello World") {
		t.Errorf("expected body at end of message, got:\n%s", out)
	}
}

func TestSMTPMailerNoRecipients(t *testing.T) {
	m := NewSMTPMailer("localhost", 1025, "", "", "noreply@skolva.org")
	if err := m.Send(context.Background(), Message{Subject: "x", Body: "y"}); err == nil {
		t.Error("expected error when no recipients are set")
	}
}
