package smtp

import (
	"fmt"
	"net"
)

// State contains all the state for a single client
type State struct {
	From          *MailAddress
	To            []*MailAddress
	Data          []byte
	EightBitMIME  bool
	Secure        bool
	SessionId     Id
	Ip            net.IP
	Hostname      string
	Authenticated bool
	User          User
}

// User denotes an authenticated SMTP user.
type User interface {
	// Username returns the username / email address of the user.
	Username() string
}

// reset the state
func (s *State) Reset() {
	s.From = nil
	s.To = []*MailAddress{}
	s.Data = []byte{}
	s.EightBitMIME = false
}

// Checks the state if the client can send a MAIL command.
func (s *State) CanReceiveMail() (bool, string) {
	if s.From != nil {
		return false, "Sender already specified"
	}

	return true, ""
}

// Checks the state if the client can send a RCPT command.
func (s *State) CanReceiveRcpt() (bool, string) {
	if s.From == nil {
		return false, "Need mail before RCPT"
	}

	return true, ""
}

// Checks the state if the client can send a DATA command.
func (s *State) CanReceiveData() (bool, string) {
	if s.From == nil {
		return false, "Need mail before DATA"
	}

	if len(s.To) == 0 {
		return false, "Need RCPT before DATA"
	}

	return true, ""
}

// Check whether the auth user is allowed to send from the MAIL FROM email address and to the RCPT TO address.
func (s *State) AuthMatchesRcptAndMail() (bool, string) {

	// TODO: what if one of those variables is nil?

	// TODO: handle if user can send from multiple email addresses
	if s.From.Address != s.User.Username() {
		return false, fmt.Sprintf("5.7.1 Sender address rejected: not owned by user %s", s.User.Username())
	}

	// TODO: check for recipient?

	return true, ""
}

// AddHeader prepends the given header to the state.
func (s *State) AddHeader(headerKey string, headerValue string) {
	header := fmt.Sprintf("%s: %s\r\n", headerKey, headerValue)
	s.Data = append([]byte(header), s.Data...)
}
