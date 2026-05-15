package mailer

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strings"
	"time"
)

var defaultDialer = &net.Dialer{
	Timeout:   10 * time.Second,
	KeepAlive: 30 * time.Second,
}

// SMTPDialer dials an SMTP server and returns an authenticated Sender.
type SMTPDialer struct {
	// Host represents the host of the SMTP server.
	Host string
	// Port represents the port of the SMTP server.
	Port int
	// Username for SMTP AUTH (empty disables authentication).
	Username string
	// Password for SMTP AUTH.
	Password string
	// Auth overrides the default authentication mechanism.
	Auth smtp.Auth
	// SSL enables direct TLS (port 465 style). Prefer STARTTLS for ports 587/25.
	SSL bool
	// TLSConfig is used for STARTTLS or direct SSL.
	TLSConfig *tls.Config
	// LocalName is the EHLO/HELO hostname. Defaults to "localhost".
	LocalName string

	dialer netDialer
}

// NewWithDialer returns a new SMTPDialer using the provided net.Dialer.
func NewWithDialer(dialer *net.Dialer, host string, port int, username, password string) *SMTPDialer {
	return &SMTPDialer{
		dialer:   dialer,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		SSL:      port == 465,
	}
}

// NewDialer returns a new SMTPDialer using the default 10s/30s net.Dialer.
func NewDialer(host string, port int, username, password string) *SMTPDialer {
	return &SMTPDialer{
		dialer:   defaultDialer,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		SSL:      port == 465,
	}
}

// Dial connects + authenticates. The returned SendCloser must be closed by the caller.
func (d *SMTPDialer) Dial() (Sender, error) {
	if d.dialer == nil {
		d.dialer = defaultDialer
	}
	conn, err := d.dialer.Dial("tcp", addr(d.Host, d.Port))
	if err != nil {
		return nil, err
	}

	if d.SSL {
		conn = tlsClient(conn, d.tlsConfig())
	}

	c, err := smtpNewClient(conn, d.Host)
	if err != nil {
		return nil, err
	}

	if d.LocalName != "" {
		if err := c.Hello(d.LocalName); err != nil {
			return nil, err
		}
	}

	if !d.SSL {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(d.tlsConfig()); err != nil {
				c.Close()
				return nil, err
			}
		}
	}

	if d.Auth == nil && d.Username != "" {
		if ok, auths := c.Extension("AUTH"); ok {
			switch {
			case strings.Contains(auths, "CRAM-MD5"):
				d.Auth = smtp.CRAMMD5Auth(d.Username, d.Password)
			case strings.Contains(auths, "LOGIN") && !strings.Contains(auths, "PLAIN"):
				d.Auth = &loginAuth{
					username: d.Username,
					password: d.Password,
					host:     d.Host,
				}
			default:
				d.Auth = smtp.PlainAuth("", d.Username, d.Password, d.Host)
			}
		}
	}

	if d.Auth != nil {
		if err = c.Auth(d.Auth); err != nil {
			c.Close()
			return nil, err
		}
	}

	return &smtpSender{c, d}, nil
}

func (d *SMTPDialer) tlsConfig() *tls.Config {
	if d.TLSConfig == nil {
		return &tls.Config{ServerName: d.Host}
	}
	return d.TLSConfig
}

func addr(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

// smtpSender wraps an smtpClient with reconnect-on-Mail-EOF behavior.
type smtpSender struct {
	smtpClient
	d *SMTPDialer
}

func (c *smtpSender) Send(from string, to []string, msg io.WriterTo) error {
	if err := c.Mail(from); err != nil {
		if err == io.EOF {
			// Probably a timeout — reconnect and retry once.
			sc, derr := c.d.Dial()
			if derr == nil {
				if s, ok := sc.(*smtpSender); ok {
					*c = *s
					return c.Send(from, to, msg)
				}
			}
		}
		return err
	}

	for _, a := range to {
		if err := c.Rcpt(a); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	if _, err = msg.WriteTo(w); err != nil {
		w.Close()
		return err
	}

	return w.Close()
}

func (c *smtpSender) Close() error {
	return c.Quit()
}

func (c *smtpSender) Reset() error {
	return c.smtpClient.Reset()
}

// netDialer is the minimal interface satisfied by *net.Dialer.
type netDialer interface {
	Dial(network, address string) (net.Conn, error)
}

// Test hooks.
var (
	tlsClient     = tls.Client
	smtpNewClient = func(conn net.Conn, host string) (smtpClient, error) {
		return smtp.NewClient(conn, host)
	}
)

// smtpClient is the subset of net/smtp.Client used by this package.
type smtpClient interface {
	Hello(string) error
	Extension(string) (bool, string)
	StartTLS(*tls.Config) error
	Auth(smtp.Auth) error
	Mail(string) error
	Rcpt(string) error
	Reset() error
	Data() (io.WriteCloser, error)
	Quit() error
	Close() error
}
