package postgres

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"dapper.local/platform/internal/twilio"
)

var (
	ErrMessagingInput       = errors.New("check the settings: the email server, port and From address, or the Twilio Account SID, Auth Token and sending number")
	ErrMessagingUnavailable = errors.New("the server key needed to keep these settings safe is not set")
	ErrMessagingNotSet      = errors.New("these settings have not been saved yet")
)

// SMTPSettings are the outgoing email details. Password is only filled in when the server reads them back for use.
type SMTPSettings struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	FromAddress string `json:"fromAddress"`
	FromName    string `json:"fromName"`
	// NotifyEmail is where the business alerts go: one or more addresses, comma separated (the From address when blank).
	NotifyEmail string `json:"notifyEmail"`
	// Which business alerts are sent.
	NotifyNew         bool `json:"notifyNew"`
	NotifyCancelled   bool `json:"notifyCancelled"`
	NotifyRescheduled bool `json:"notifyRescheduled"`
}

// notifyList splits the business addresses ("a@x.ca, b@y.ca"), checks each and gives them back tidy. At most ten.
func notifyList(v string) ([]string, bool) {
	out := []string{}
	for _, part := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\t' }) {
		addr, err := mail.ParseAddress(part)
		if err != nil || addr.Address != part {
			return nil, false
		}
		out = append(out, part)
	}
	return out, len(out) <= 10
}

// SMSSettings are the Twilio details. AuthToken is only filled in when the server reads them back for use.
type SMSSettings struct {
	AccountSID string `json:"accountSid"`
	AuthToken  string `json:"authToken,omitempty"`
	From       string `json:"from"`
}

// MessagingView is what the dashboard sees: never a password or token, only whether one is saved.
type MessagingView struct {
	SMTP struct {
		SMTPSettings
		PasswordSet bool `json:"passwordSet"`
		Saved       bool `json:"saved"`
	} `json:"smtp"`
	SMS struct {
		SMSSettings
		TokenSet bool `json:"tokenSet"`
		Saved    bool `json:"saved"`
	} `json:"sms"`
}

const (
	smtpSealContext = "messaging/smtp-password"
	smsSealContext  = "messaging/twilio-token"
)

// messagingManager: these settings can send mail and texts as the business, so only the access-management capability may change them.
func (s *Store) messagingManager(ctx context.Context, tx pgx.Tx, actor, org string) error {
	ok, err := actorHasOrgCapability(ctx, tx, actor, org, "access.manage")
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *Store) MessagingSettings(ctx context.Context, actor, org string) (MessagingView, error) {
	var view MessagingView
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return view, err
	}
	defer tx.Rollback(ctx)
	if err = s.messagingManager(ctx, tx, actor, org); err != nil {
		return view, err
	}
	var smtpSealed, smsSealed []byte
	err = tx.QueryRow(ctx, `SELECT smtp_host,smtp_port,smtp_username,smtp_password_sealed,smtp_from_address,smtp_from_name,smtp_notify_email,notify_new,notify_cancelled,notify_rescheduled,twilio_account_sid,twilio_auth_token_sealed,twilio_from FROM messaging_settings WHERE organization_id=$1`, org).
		Scan(&view.SMTP.Host, &view.SMTP.Port, &view.SMTP.Username, &smtpSealed, &view.SMTP.FromAddress, &view.SMTP.FromName, &view.SMTP.NotifyEmail, &view.SMTP.NotifyNew, &view.SMTP.NotifyCancelled, &view.SMTP.NotifyRescheduled, &view.SMS.AccountSID, &smsSealed, &view.SMS.From)
	if errors.Is(err, pgx.ErrNoRows) {
		view.SMTP.Port = 587
		return view, nil
	}
	if err != nil {
		return view, err
	}
	view.SMTP.Saved = view.SMTP.Host != ""
	view.SMTP.PasswordSet = len(smtpSealed) > 0
	view.SMS.Saved = view.SMS.AccountSID != ""
	view.SMS.TokenSet = len(smsSealed) > 0
	return view, nil
}

func trimmed(v string, max int) (string, bool) {
	v = strings.TrimSpace(v)
	return v, utf8.RuneCountInString(v) <= max && !strings.ContainsAny(v, "\r\n")
}

// SaveSMTPSettings keeps the email details. A blank password keeps the one already saved.
func (s *Store) SaveSMTPSettings(ctx context.Context, actor, org string, in SMTPSettings) error {
	host, ok1 := trimmed(in.Host, 255)
	user, ok2 := trimmed(in.Username, 255)
	from, ok3 := trimmed(in.FromAddress, 254)
	name, ok4 := trimmed(in.FromName, 120)
	notify, ok5 := trimmed(in.NotifyEmail, 1000)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || host == "" || strings.ContainsAny(host, " /\\") || in.Port < 1 || in.Port > 65535 || len(in.Password) > 500 {
		return ErrMessagingInput
	}
	if addr, err := mail.ParseAddress(from); err != nil || addr.Address != from {
		return ErrMessagingInput
	}
	list, okList := notifyList(notify)
	if !okList {
		return ErrMessagingInput
	}
	notify = strings.Join(list, ", ")
	if !s.MFAConfigured() {
		return ErrMessagingUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = s.messagingManager(ctx, tx, actor, org); err != nil {
		return err
	}
	var sealed []byte
	if in.Password != "" {
		if sealed, err = s.sealSecret(smtpSealContext, []byte(in.Password)); err != nil {
			return err
		}
	}
	// keep the saved password when none was typed
	if _, err = tx.Exec(ctx, `INSERT INTO messaging_settings(organization_id,smtp_host,smtp_port,smtp_username,smtp_password_sealed,smtp_from_address,smtp_from_name,smtp_notify_email,notify_new,notify_cancelled,notify_rescheduled,updated_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$9,$10,$11,$12,$8)
ON CONFLICT(organization_id) DO UPDATE SET smtp_host=$2,smtp_port=$3,smtp_username=$4,smtp_password_sealed=COALESCE($5,messaging_settings.smtp_password_sealed),smtp_from_address=$6,smtp_from_name=$7,smtp_notify_email=$9,notify_new=$10,notify_cancelled=$11,notify_rescheduled=$12,updated_by=$8,updated_at=now()`,
		org, host, in.Port, user, nullIfEmpty(sealed), from, name, actor, notify, in.NotifyNew, in.NotifyCancelled, in.NotifyRescheduled); err != nil {
		return err
	}
	if err = structureAudit(ctx, tx, actor, org, "messaging.smtp_saved", org); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func nullIfEmpty(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// ClearSMTPSettings removes the saved email details; the server's own settings (if any) take over.
func (s *Store) ClearSMTPSettings(ctx context.Context, actor, org string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = s.messagingManager(ctx, tx, actor, org); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE messaging_settings SET smtp_host='',smtp_port=587,smtp_username='',smtp_password_sealed=NULL,smtp_from_address='',smtp_from_name='',updated_by=$2,updated_at=now() WHERE organization_id=$1`, org, actor); err != nil {
		return err
	}
	if err = structureAudit(ctx, tx, actor, org, "messaging.smtp_cleared", org); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SaveSMSSettings keeps the Twilio details. A blank Auth Token keeps the one already saved.
func (s *Store) SaveSMSSettings(ctx context.Context, actor, org string, in SMSSettings) error {
	sid, from := strings.TrimSpace(in.AccountSID), twilio.NormalizePhone(in.From)
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(in.From)), "MG") {
		from = strings.TrimSpace(in.From)
	}
	token := strings.TrimSpace(in.AuthToken)
	if !twilio.ValidSID(sid) || !twilio.ValidFrom(from) || (token != "" && (len(token) < 16 || len(token) > 100 || strings.ContainsAny(token, " \r\n\t"))) {
		return ErrMessagingInput
	}
	if !s.MFAConfigured() {
		return ErrMessagingUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = s.messagingManager(ctx, tx, actor, org); err != nil {
		return err
	}
	var sealed []byte
	if token != "" {
		if sealed, err = s.sealSecret(smsSealContext, []byte(token)); err != nil {
			return err
		}
	} else {
		var have bool
		if e := tx.QueryRow(ctx, `SELECT twilio_auth_token_sealed IS NOT NULL FROM messaging_settings WHERE organization_id=$1`, org).Scan(&have); e != nil || !have {
			return ErrMessagingInput // the first save needs a token
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO messaging_settings(organization_id,twilio_account_sid,twilio_auth_token_sealed,twilio_from,updated_by) VALUES($1,$2,$3,$4,$5)
ON CONFLICT(organization_id) DO UPDATE SET twilio_account_sid=$2,twilio_auth_token_sealed=COALESCE($3,messaging_settings.twilio_auth_token_sealed),twilio_from=$4,updated_by=$5,updated_at=now()`,
		org, sid, nullIfEmpty(sealed), from, actor); err != nil {
		return err
	}
	if err = structureAudit(ctx, tx, actor, org, "messaging.sms_saved", org); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ClearSMSSettings removes the saved Twilio details.
func (s *Store) ClearSMSSettings(ctx context.Context, actor, org string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = s.messagingManager(ctx, tx, actor, org); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE messaging_settings SET twilio_account_sid='',twilio_auth_token_sealed=NULL,twilio_from='',updated_by=$2,updated_at=now() WHERE organization_id=$1`, org, actor); err != nil {
		return err
	}
	if err = structureAudit(ctx, tx, actor, org, "messaging.sms_cleared", org); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SavedSMTP and SavedSMS read the full saved details for use by the server (never sent to a browser). ok is false when nothing is saved.
func (s *Store) SavedSMTP(ctx context.Context) (SMTPSettings, bool, error) {
	var out SMTPSettings
	var sealed []byte
	err := s.pool.QueryRow(ctx, `SELECT smtp_host,smtp_port,smtp_username,smtp_password_sealed,smtp_from_address,smtp_from_name FROM messaging_settings WHERE smtp_host<>'' ORDER BY updated_at DESC LIMIT 1`).
		Scan(&out.Host, &out.Port, &out.Username, &sealed, &out.FromAddress, &out.FromName)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	if len(sealed) > 0 {
		if !s.MFAConfigured() {
			return out, false, ErrMessagingUnavailable
		}
		plain, e := s.openSecret(smtpSealContext, sealed)
		if e != nil {
			return out, false, e
		}
		out.Password = string(plain)
	}
	return out, true, nil
}

func (s *Store) SavedSMS(ctx context.Context) (SMSSettings, bool, error) {
	var out SMSSettings
	var sealed []byte
	err := s.pool.QueryRow(ctx, `SELECT twilio_account_sid,twilio_auth_token_sealed,twilio_from FROM messaging_settings WHERE twilio_account_sid<>'' ORDER BY updated_at DESC LIMIT 1`).Scan(&out.AccountSID, &sealed, &out.From)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	if len(sealed) == 0 || !s.MFAConfigured() {
		return out, false, ErrMessagingNotSet
	}
	plain, e := s.openSecret(smsSealContext, sealed)
	if e != nil {
		return out, false, e
	}
	out.AuthToken = string(plain)
	return out, true, nil
}
