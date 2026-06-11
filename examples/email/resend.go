package email

import (
	"context"
	"fmt"

	"github.com/resend/resend-go/v2"
)

// ResendSender implements auth.MagicLinkEmailSender using Resend.
type ResendSender struct {
	client      *resend.Client
	fromAddress string
	fromName    string
}

// NewResendSender creates a new ResendSender.
// apiKey is your Resend API key.
// fromAddress is the email address to send from (must be verified with Resend).
// fromName is the display name for the sender.
func NewResendSender(apiKey, fromAddress, fromName string) *ResendSender {
	return &ResendSender{
		client:      resend.NewClient(apiKey),
		fromAddress: fromAddress,
		fromName:    fromName,
	}
}

// SendMagicLink sends a magic link email using Resend.
func (s *ResendSender) SendMagicLink(ctx context.Context, email, verifyURL string, expiresInMinutes int) error {
	from := s.fromAddress
	if s.fromName != "" {
		from = fmt.Sprintf("%s <%s>", s.fromName, s.fromAddress)
	}

	body := fmt.Sprintf(`Click the link below to log in:

%s

This link expires in %d minutes.

If you didn't request this, you can safely ignore this email.`, verifyURL, expiresInMinutes)

	params := &resend.SendEmailRequest{
		From:    from,
		To:      []string{email},
		Subject: "Your login link",
		Text:    body,
	}

	_, err := s.client.Emails.SendWithContext(ctx, params)
	return err
}
