package operations

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/netip"
	"net/smtp"
	"net/url"
	"regexp"
	"strings"
	"time"

	"shipper/internal/netguard"
	"shipper/internal/store"
)

type NotificationConfig struct {
	Enabled    bool     `json:"enabled"`
	Kind       string   `json:"kind"`
	Events     []string `json:"events"`
	Webhook    string   `json:"webhook,omitempty"`
	Private    bool     `json:"private"`
	SMTP       string   `json:"smtp,omitempty"`
	Username   string   `json:"username,omitempty"`
	Password   string   `json:"password,omitempty"`
	From       string   `json:"from,omitempty"`
	To         string   `json:"to,omitempty"`
	Configured bool     `json:"configured"`
}

func LoadNotifications(ctx context.Context, db *store.Store) (NotificationConfig, error) {
	var c NotificationConfig
	raw, e := db.PrivateSetting(ctx, "notifications", `{"kind":"webhook","events":["deployment","service","backup","storage","certificate","daemon","proxy"]}`)
	if e == nil {
		e = json.Unmarshal([]byte(raw), &c)
	}
	return c, e
}
func (c NotificationConfig) Validate() error {
	if len(c.Events) > 10 {
		return errors.New("Choose fewer event types")
	}
	for _, v := range c.Events {
		if !strings.Contains("|deployment|service|backup|storage|certificate|daemon|proxy|update|task|", "|"+v+"|") {
			return errors.New("Choose a supported event type")
		}
	}
	if !c.Enabled {
		return nil
	}
	if c.Kind == "webhook" {
		u, e := url.Parse(c.Webhook)
		if e != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || len(c.Webhook) > 2048 || u.Scheme != "https" && !(c.Private && u.Scheme == "http") {
			return errors.New("Enter an HTTPS webhook URL or explicitly allow a private endpoint")
		}
		return nil
	}
	if c.Kind != "email" {
		return errors.New("Choose Webhook or Email")
	}
	host, port, e := net.SplitHostPort(c.SMTP)
	if e != nil || host == "" || port != "465" && port != "587" {
		return errors.New("Use an SMTP host with port 465 or 587")
	}
	for _, v := range []string{c.From, c.To} {
		m, e := mail.ParseAddress(v)
		if e != nil || m.Address != v || strings.ContainsAny(v, "\r\n") {
			return errors.New("Enter sender and recipient email addresses")
		}
	}
	return nil
}
func SafeDial(ctx context.Context, network, address string, private bool) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil {
		return nil, e
	}
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
	if e != nil || len(ips) == 0 {
		return nil, errors.New("Notification host could not be resolved")
	}
	for _, v := range ips {
		ip, ok := netip.AddrFromSlice(v.IP)
		if !private && (!ok || !netguard.Public(ip)) {
			return nil, errors.New("Private notification destinations require explicit permission")
		}
	}
	var last error
	for _, v := range ips {
		conn, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(v.IP.String(), port))
		if e == nil {
			return conn, nil
		}
		last = e
	}
	return nil, last
}

var discordWebhookPath = regexp.MustCompile(`^/api(?:/v[0-9]+)?/webhooks/[0-9]+/[^/]+/?$`)

func webhookPayload(destination string, event store.OperationEvent) []byte {
	var payload any = map[string]any{"version": "1", "event": event}
	u, err := url.Parse(destination)
	if err == nil && u.Scheme == "https" && (u.Port() == "" || u.Port() == "443") &&
		(strings.EqualFold(u.Hostname(), "discord.com") || strings.EqualFold(u.Hostname(), "discordapp.com") ||
			strings.EqualFold(u.Hostname(), "canary.discord.com") || strings.EqualFold(u.Hostname(), "ptb.discord.com")) &&
		discordWebhookPath.MatchString(u.Path) {
		content := "Shipper " + event.Kind + " " + event.State + "\n" + event.Summary
		if event.ApplicationID != "" {
			content += "\nApplication: " + event.ApplicationID
		}
		if event.Reference != "" {
			content += "\nReference: " + event.Reference
		}
		// Bound UTF-16 units too, so supplementary Unicode cannot exceed Discord's limit.
		units := 0
		for index, char := range content {
			size := 1
			if char > 0xffff {
				size = 2
			}
			if units+size > 1999 {
				content = content[:index] + "…"
				break
			}
			units += size
		}
		payload = map[string]any{"content": content, "allowed_mentions": map[string]any{"parse": []string{}}}
	}
	raw, _ := json.Marshal(payload)
	return raw
}

func Send(ctx context.Context, c NotificationConfig, event store.OperationEvent) error {
	if e := c.Validate(); e != nil {
		return e
	}
	if !c.Enabled {
		return nil
	}
	if c.Kind == "webhook" {
		data := webhookPayload(c.Webhook, event)
		req, e := http.NewRequestWithContext(ctx, "POST", c.Webhook, bytes.NewReader(data))
		if e != nil {
			return e
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Shipper-Event", event.Kind)
		transport := &http.Transport{Proxy: nil, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return SafeDial(ctx, network, address, c.Private)
		}}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Notification redirects are disabled") }}
		res, e := client.Do(req)
		if e != nil {
			return errors.New("Webhook delivery failed")
		}
		res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("Webhook returned HTTP %d", res.StatusCode)
		}
		return nil
	}
	conn, e := SafeDial(ctx, "tcp", c.SMTP, c.Private)
	if e != nil {
		return e
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(8 * time.Second))
	host, port, _ := net.SplitHostPort(c.SMTP)
	config := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if port == "465" {
		secure := tls.Client(conn, config)
		if e = secure.HandshakeContext(ctx); e != nil {
			return errors.New("SMTP TLS handshake failed")
		}
		conn = secure
	}
	client, e := smtp.NewClient(conn, host)
	if e != nil {
		return errors.New("SMTP connection failed")
	}
	defer client.Close()
	if port == "587" {
		if e = client.StartTLS(config); e != nil {
			return errors.New("SMTP requires trusted TLS")
		}
	}
	if c.Username != "" {
		if e = client.Auth(smtp.PlainAuth("", c.Username, c.Password, host)); e != nil {
			return errors.New("SMTP authentication failed")
		}
	}
	if e = client.Mail(c.From); e != nil {
		return e
	}
	if e = client.Rcpt(c.To); e != nil {
		return e
	}
	w, e := client.Data()
	if e != nil {
		return e
	}
	_, e = fmt.Fprintf(w, "From: %s\r\nTo: %s\r\nSubject: Shipper %s %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\nApplication: %s\r\nReference: %s\r\n", c.From, c.To, event.Kind, event.State, event.Summary, event.ApplicationID, event.Reference)
	if e != nil {
		return e
	}
	if e = w.Close(); e != nil {
		return e
	}
	return client.Quit()
}
func (s *Service) Deliver(ctx context.Context) {
	c, e := LoadNotifications(ctx, s.DB)
	if e != nil {
		return
	}
	for i := 0; i < 3 && ctx.Err() == nil; i++ {
		v, n, e := s.DB.NextNotification(ctx)
		if e != nil {
			return
		}
		wanted := false
		for _, kind := range c.Events {
			wanted = wanted || kind == v.Kind
		}
		success := true
		failure := ""
		if c.Enabled && wanted {
			e = Send(ctx, c, v)
			success = e == nil
			if e != nil {
				// Do not persist SMTP responses or destination URLs, which can echo credentials.
				failure = "SMTP delivery failed"
				if c.Kind == "webhook" {
					failure = "Webhook delivery failed"
				}
				if strings.HasPrefix(e.Error(), "Webhook returned HTTP ") {
					failure = e.Error()
				}
			}
		}
		_ = s.DB.FinishNotification(ctx, v.Sequence, n, success, failure)
	}
}
