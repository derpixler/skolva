package mail

import "context"

type Message struct {
	To      []string
	Subject string
	Body    string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}
