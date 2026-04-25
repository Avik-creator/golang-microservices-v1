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

func BuildTransactionEmail(event *model.TransactionEvent) *model.Email {
	var subject, body string

	switch event.EventType {
	case "transaction.completed":
		subject = fmt.Sprintf("Payment Successful — %s %.2f", event.Currency, event.Amount)
		body = fmt.Sprintf(`Hello,

Your payment of %s %.2f has been processed successfully.

Transaction ID : %s
From Account   : %s
To Account     : %s
Status         : Completed

Thank you for using FinTech Platform.
`, event.Currency, event.Amount, event.TransactionID, event.FromAccountID, event.ToAccountID)

	case "transaction.failed":
		subject = fmt.Sprintf("Payment Failed — %s %.2f", event.Currency, event.Amount)
		body = fmt.Sprintf(`Hello,

Unfortunately, your payment of %s %.2f could not be processed.

Transaction ID : %s
From Account   : %s
To Account     : %s
Status         : Failed

Please check your account balance and try again. If the issue persists, contact support.
`, event.Currency, event.Amount, event.TransactionID, event.FromAccountID, event.ToAccountID)
	}

	return &model.Email{
		// In a real system, look up the user email from user-service.
		// For local dev, we send to a fixed address visible in Mailhog at :8025
		To:      "user@fintech.local",
		Subject: subject,
		Body:    body,
	}
}

// BuildFraudAlertEmail constructs the email body for a fraud alert.
func BuildFraudAlertEmail(alert *model.FraudAlert) *model.Email {
	reasons := strings.Join(alert.Reasons, ", ")
	subject := fmt.Sprintf("⚠️ Fraud Alert — Transaction %s", alert.TransactionID)
	body := fmt.Sprintf(`Hello,

A suspicious transaction has been detected on your account.

Transaction ID : %s
Amount         : %s %.2f
Risk Score     : %d / 100
Reasons        : %s
Detected At    : %s

If you did not authorise this transaction, please contact support immediately.

FinTech Platform Security Team
`, alert.TransactionID, alert.Currency, alert.Amount,
		alert.Score, reasons, alert.AlertedAt.Format("2006-01-02 15:04:05 UTC"))

	return &model.Email{
		To:      "user@fintech.local",
		Subject: subject,
		Body:    body,
	}
}
