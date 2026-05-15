package mailer

import (
	"errors"
	"fmt"
	"net/smtp"
)

// loginAuth implements the SMTP LOGIN authentication mechanism, used by some
// servers (notably Office 365 and older Exchange) instead of PLAIN.
// loginAuth implements smtp.Auth for the LOGIN authentication mechanism,
// which is required by some SMTP servers that do not advertise PLAIN.
type loginAuth struct {
	username string
	password string
	host     string
}

// Start returns the LOGIN mechanism name and an empty initial response.
func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS {
		advertised := false
		for _, mechanism := range server.Auth {
			if mechanism == "LOGIN" {
				advertised = true
				break
			}
		}
		if !advertised {
			return "", nil, errors.New("mailer: unencrypted connection")
		}
	}
	if server.Name != a.host {
		return "", nil, errors.New("mailer: wrong host name")
	}
	return "LOGIN", nil, nil
}

// Next responds to the server's Base64 challenges.
func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch string(fromServer) {
	case "Username:":
		return []byte(a.username), nil
	case "Password:":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("mailer: unexpected server challenge: %s", fromServer)
	}
}
