package smtp

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type StatusCode uint32

// SMTP status codes
const (
	Ready             StatusCode = 220
	Closing           StatusCode = 221
	Ok                StatusCode = 250
	StartData         StatusCode = 354
	ShuttingDown      StatusCode = 421
	SyntaxError       StatusCode = 500
	SyntaxErrorParam  StatusCode = 501
	NotImplemented    StatusCode = 502
	BadSequence       StatusCode = 503
	AbortMail         StatusCode = 552
	NoValidRecipients StatusCode = 554
)

// SMTP status codes for AUTH extension (RFC 4954)
const (
	AuthenticationSucceeded                               StatusCode = 235
	EncodedString                                         StatusCode = 334
	PasswordTransitionNeeded                              StatusCode = 432
	TemporaryAuthenticationFailure                        StatusCode = 454
	AuthenticationExchangeLineTooLong                     StatusCode = 500
	MalformedAuthInput                                    StatusCode = 501
	AuthCommandNotPermittedDuringMailTransaction          StatusCode = 503
	UnrecognizedAuthenticationType                        StatusCode = 504
	AuthenticationRequired                                StatusCode = 530
	AuthenticationMechanismTooWeak                        StatusCode = 534
	AuthenticationCredentialsInvalid                      StatusCode = 535
	EncryptionRequiredForRequestedAuthenticationMechanism StatusCode = 538
)

// ErrLtl Line too long error
var ErrLtl = errors.New("line too long")

// ErrIncomplete Incomplete data error
var ErrIncomplete = errors.New("incomplete data")

const (
	MAX_DATA_LINE = 1000
	MAX_CMD_LINE  = 512
)

// ReadUntill reads untill delim is found or max bytes are read.
// If delim was found it returns nil as error. If delim wasn't found after max bytes,
// it returns ErrLtl.
func ReadUntill(delim byte, max int, r io.Reader) ([]byte, error) {
	buffer := make([]byte, max)

	n := 0
	for n < max {
		read, err := r.Read(buffer[n : n+1])
		if read == 0 || err != nil {
			return buffer[0:n], err
		}

		if read > 1 {
			panic("Should read 1 byte at a time.")
		}

		if buffer[n] == delim {
			return buffer[0 : n+1], nil
		}

		n++

	}

	return buffer[0:n], ErrLtl
}

// SkipTillNewline removes all data untill a newline is found.
func SkipTillNewline(r io.Reader) error {
	var err error
	for {
		_, err = ReadUntill('\n', 1000, r)
		if err != nil {
			if err == ErrLtl {
				continue
			}

			break
		}

		break
	}

	return err
}

// DataReader implements the reader that will read the data from a MAIL cmd
type DataReader struct {
	br          *bufio.Reader
	state       int
	bytesInLine int
}

func NewDataReader(br *bufio.Reader) *DataReader {
	dr := &DataReader{
		br: br,
	}

	return dr
}

// Implementation from textproto.DotReader.Read
func (r *DataReader) Read(b []byte) (n int, err error) {
	// Run data through a simple state machine to
	// elide leading dots, rewrite trailing \r\n into \n,
	// and detect ending .\r\n line.
	const (
		stateBeginLine = iota // beginning of line; initial state; must be zero
		stateDot              // read . at beginning of line
		stateDotCR            // read .\r at beginning of line
		stateCR               // read \r (possibly at end of line)
		stateData             // reading data in middle of line
		stateEOF              // reached .\r\n end marker line
	)

	br := r.br
	for n < len(b) && r.state != stateEOF {
		var c byte
		c, err = br.ReadByte()
		if err != nil {
			err = ErrIncomplete
			break
		}
		r.bytesInLine++
		if r.bytesInLine > MAX_DATA_LINE {
			err = ErrLtl
			_ = SkipTillNewline(br)
			r.bytesInLine = 0
			r.state = stateBeginLine
			break
		}
		switch r.state {
		case stateBeginLine:
			if c == '.' {
				r.state = stateDot
				continue
			}
			if c == '\r' {
				r.state = stateCR
				continue
			}
			r.state = stateData

		case stateDot:
			if c == '\r' {
				r.state = stateDotCR
				continue
			}
			if c == '\n' {
				r.state = stateEOF
				continue
			}
			r.state = stateData

		case stateDotCR:
			if c == '\n' {
				r.state = stateEOF
				continue
			}
			// Not part of .\r\n.
			// Consume leading dot and emit saved \r.
			err := br.UnreadByte()
			if err != nil {
				return n, fmt.Errorf("couldn't unread byte: %w", err)
			}
			c = '\r'
			r.state = stateData

		case stateCR:
			if c == '\n' {
				r.state = stateBeginLine
				r.bytesInLine = 0
				break
			}
			// Not part of \r\n.  Emit saved \r
			err := br.UnreadByte()
			if err != nil {
				return n, fmt.Errorf("couldn't unread byte: %w", err)
			}
			c = '\r'
			r.state = stateData

		case stateData:
			if c == '\r' {
				r.state = stateCR
				continue
			}
			if c == '\n' {
				r.state = stateBeginLine
				r.bytesInLine = 0
			}
		}
		b[n] = c
		n++
	}

	if err == nil && r.state == stateEOF {
		err = io.EOF
	}

	return
}

// Cmd All SMTP answers/commands should implement this interface.
type Cmd interface {
	fmt.Stringer
}

// Answer A raw SMTP answer. Used to send a status code + message.
type Answer struct {
	Status  StatusCode
	Message string
}

func (c Answer) String() string {
	return fmt.Sprintf("%d %s", c.Status, c.Message)
}

// MultiAnswer A multiline answer.
type MultiAnswer struct {
	Status   StatusCode
	Messages []string
}

func (c MultiAnswer) String() string {
	if len(c.Messages) == 0 {
		return fmt.Sprintf("%d", c.Status)
	}

	result := ""
	for i := 0; i < len(c.Messages)-1; i++ {
		result += fmt.Sprintf("%d-%s", c.Status, c.Messages[i])
		result += "\r\n"
	}

	result += fmt.Sprintf("%d %s", c.Status, c.Messages[len(c.Messages)-1])

	return result
}

// InvalidCmd is a known command with invalid arguments or syntax
type InvalidCmd struct {
	// The command
	Cmd  string
	Info string
}

func (c InvalidCmd) String() string {
	return fmt.Sprintf("%s %s", c.Cmd, c.Info)
}

// UnknownCmd is a command that is none of the other commands. i.e. not implemented
type UnknownCmd struct {
	// The command
	Cmd  string
	Line string
}

func (c UnknownCmd) String() string {
	return c.Cmd
}

type HeloCmd struct {
	Domain string
}

func (c HeloCmd) String() string {
	return ""
}

type EhloCmd struct {
	Domain string
}

func (c EhloCmd) String() string {
	return ""
}

type QuitCmd struct {
}

func (c QuitCmd) String() string {
	return ""
}

type MailCmd struct {
	From         *MailAddress
	EightBitMIME bool
}

func (c MailCmd) String() string {
	return ""
}

type RcptCmd struct {
	To *MailAddress
}

func (c RcptCmd) String() string {
	return ""
}

type DataCmd struct {
	Data []byte
	R    DataReader
}

func (c DataCmd) String() string {
	return ""
}

type RsetCmd struct {
}

func (c RsetCmd) String() string {
	return ""
}

type StartTlsCmd struct {
}

func (c StartTlsCmd) String() string {
	return ""
}

type NoopCmd struct{}

func (c NoopCmd) String() string {
	return ""
}

// Not implemented because of security concerns
type VrfyCmd struct {
	Param string
}

func (c VrfyCmd) String() string {
	return ""
}

type ExpnCmd struct {
	ListName string
}

func (c ExpnCmd) String() string {
	return ""
}

type SendCmd struct{}

func (c SendCmd) String() string {
	return ""
}

type SomlCmd struct{}

func (c SomlCmd) String() string {
	return ""
}

type SamlCmd struct{}

func (c SamlCmd) String() string {
	return ""
}

type AuthCmd struct {
	Mechanism       string
	InitialResponse string
	R               bufio.Reader
}

func (c AuthCmd) String() string {
	return fmt.Sprintf("AUTH %s", c.Mechanism)
}

type Id struct {
	Timestamp int64
	Counter   uint32
}

func (id *Id) String() string {
	return strconv.FormatInt(id.Timestamp, 16) + strconv.FormatInt(int64(id.Counter), 16)
}

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

// Protocol Used as communication layer so we can easily switch between a real socket
// and a test implementation.
type Protocol interface {
	// Send a SMTP command.
	Send(Cmd)
	// Receive a command(will block while waiting for it).
	// Returns an error if something wen't wrong. E.g line was too long.
	GetCmd() (*Cmd, error)
	// Close the connection.
	Close()
	// StartTls starts the tls handshake.
	StartTls(*tls.Config) error
	// GetIP gets the ip of the client.
	GetIP() net.IP
	// Get the state that belongs to this connection.
	GetState() *State
}

type MtaProtocol struct {
	c      net.Conn
	br     *bufio.Reader
	parser parser
	state  *State
}

// NewMtaProtocol Creates a protocol that works over a socket.
// the net.Conn parameter will be closed when done.
func NewMtaProtocol(c net.Conn) *MtaProtocol {
	proto := &MtaProtocol{
		c:      c,
		br:     bufio.NewReader(c),
		parser: parser{},
		state:  &State{},
	}

	return proto
}

func (p *MtaProtocol) Send(c Cmd) {
	log.WithFields(log.Fields{
		"Cmd":       fmt.Sprintf("%#v", c),
		"SessionId": p.state.SessionId.String(),
		"Ip":        p.state.Ip.String(),
	}).Debug("Sending cmd")
	fmt.Fprintf(p.c, "%s\r\n", c)
}

func (p *MtaProtocol) GetCmd() (*Cmd, error) {
	cmd, err := p.parser.ParseCommand(p.br)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Debug("MtaProtocol.GetCmd could not parse command")
		return nil, err
	}

	log.WithFields(log.Fields{
		"Cmd":       fmt.Sprintf("%#v", cmd),
		"SessionId": p.state.SessionId.String(),
		"Ip":        p.state.Ip.String(),
	}).Debug("Received cmd")
	return &cmd, nil
}

func (p *MtaProtocol) Close() {
	err := p.c.Close()
	if err != nil {
		log.Printf("Error while closing protocol: %v", err)
	}
}

func (p *MtaProtocol) StartTls(c *tls.Config) error {
	tlsCon := tls.Server(p.c, c)
	err := tlsCon.Handshake()
	if err != nil {
		return err
	}

	p.c = tlsCon
	p.br.Reset(p.c)
	return nil
}

func (p *MtaProtocol) GetIP() net.IP {
	ip, _, err := net.SplitHostPort(p.c.RemoteAddr().String())
	if err != nil {
		log.Printf("Could not get ip: %v", p.c.RemoteAddr().String())
		return nil
	}

	return net.ParseIP(ip)
}

func (p *MtaProtocol) GetState() *State {
	return p.state
}
