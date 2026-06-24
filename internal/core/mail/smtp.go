package mail

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type SMTPMailer struct {
	host string
	port int
	user string
	pass string
	from string
}

func NewSMTPMailer(host string, port int, user, pass, from string) *SMTPMailer {
	return &SMTPMailer{
		host: host,
		port: port,
		user: user,
		pass: pass,
		from: from,
	}
}

func (m *SMTPMailer) Send(_ context.Context, msg Message) error {
	if len(msg.To) == 0 {
		return errors.New("mail: no recipients")
	}
	addr := net.JoinHostPort(m.host, strconv.Itoa(m.port))
	var auth smtp.Auth
	if m.user != "" {
		auth = smtp.PlainAuth("", m.user, m.pass, m.host)
	}
	return smtp.SendMail(addr, auth, m.from, msg.To, m.build(msg))
}

func (m *SMTPMailer) build(msg Message) []byte {
	headers := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Date: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/plain; charset=\"utf-8\"\r\n"+
			"\r\n",
		m.from,
		strings.Join(msg.To, ", "),
		msg.Subject,
		time.Now().UTC().Format(time.RFC1123Z),
	)
	return []byte(headers + msg.Body)
}
