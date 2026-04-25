package service

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"avikmukherjee/m/notification-service/internal/model"
)

type Mailer struct {
	host string
	port string
	from string
}

func NewMailer(host, port, from string) *Mailer {
	return &Mailer{host: host, port: port, from: from}
}

// Send delivers an email via SMTP. Mailhog requires no auth — perfect for local dev.
func (m *Mailer) Send(email *model.Email) error {
	addr := fmt.Sprintf("%s:%s", m.host, m.port)

	msg := strings.Join([]string{
		fmt.Sprintf("From: %s", m.from),
		fmt.Sprintf("To: %s", email.To),
		fmt.Sprintf("Subject: %s", email.Subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		email.Body,
	}, "\r\n")

	err := smtp.SendMail(addr, nil, m.from, []string{email.To}, []byte(msg))
	if err != nil {
		log.Printf("[mailer] failed to send email to %s: %v", email.To, err)
		return err
	}

	log.Printf("[mailer] ✉️  email sent to %s — subject: %s", email.To, email.Subject)
	return nil
}
