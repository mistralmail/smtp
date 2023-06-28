package mta

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/gopistolet/smtp/smtp"
	c "github.com/smartystreets/goconvey/convey"
)

// Some default ip
var someIp string = "1.2.3.4"

// Dummy mail handler
func dummyHandler(*smtp.State) error {
	return nil
}

// Dummy mail handler which returns error
func dummyHandlerError(*smtp.State) error {
	return smtp.SMTPErrorPermanentMailboxNotAvailable
}

type testProtocol struct {
	t *testing.T
	// Goconvey context so it works in a different goroutine
	ctx       c.C
	cmds      []smtp.Cmd
	answers   []interface{}
	expectTLS bool
	state     smtp.State
}

func getMailWithoutError(a string) *smtp.MailAddress {
	addr, _ := smtp.ParseAddress(a)
	return &addr
}

func (p *testProtocol) Send(cmd smtp.Cmd) {
	p.ctx.So(len(p.answers), c.ShouldBeGreaterThan, 0)

	//c.Printf("RECEIVED: %#v\n", cmd)

	answer := p.answers[0]
	p.answers = p.answers[1:]

	if cmdA, ok := cmd.(smtp.Answer); ok {
		p.ctx.So(cmdA, c.ShouldHaveSameTypeAs, answer)
		cmdE, _ := answer.(smtp.Answer)
		p.ctx.So(cmdA.Status, c.ShouldEqual, cmdE.Status)
	} else if cmdA, ok := cmd.(smtp.MultiAnswer); ok {
		p.ctx.So(cmdA, c.ShouldHaveSameTypeAs, answer)
		cmdE, _ := answer.(smtp.MultiAnswer)
		p.ctx.So(cmdA.Status, c.ShouldEqual, cmdE.Status)
	} else {
		p.t.Fatalf("Answer should be Answer or MultiAnswer")
	}
}

func (p *testProtocol) GetCmd() (*smtp.Cmd, error) {
	p.ctx.So(len(p.cmds), c.ShouldBeGreaterThan, 0)

	cmd := p.cmds[0]
	p.cmds = p.cmds[1:]

	if cmd == nil {
		return nil, io.EOF
	}

	//c.Printf("SENDING: %#v\n", cmd)
	return &cmd, nil
}

func (p *testProtocol) Close() {
	// Did not expect connection to be closed, got more commands
	p.ctx.So(len(p.cmds), c.ShouldBeLessThanOrEqualTo, 0)

	// Did not expect connection to be closed, need more answers
	p.ctx.So(len(p.answers), c.ShouldBeLessThanOrEqualTo, 0)
}

func (p *testProtocol) StartTls(c *tls.Config) error {
	if !p.expectTLS {
		p.t.Fatalf("Did not expect StartTls")
		return errors.New("NOT IMPLEMENTED")
	}

	return nil
}

func (p *testProtocol) GetIP() net.IP {
	return net.ParseIP("127.0.0.1")
}

func (p *testProtocol) GetState() *smtp.State {
	return &p.state
}

// Tests answers for HELO,EHLO and QUIT
func TestAnswersHeloQuit(t *testing.T) {
	cfg := Config{
		Hostname: "home.sweet.home",
	}

	mta := New(cfg, HandlerFunc(dummyHandler))
	if mta == nil {
		t.Fatal("Could not create mta server")
	}

	c.Convey("Testing answers for HELO and QUIT.", t, func(ctx c.C) {

		// Test connection with HELO and QUIT
		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.HeloCmd{
					Domain: "some.sender",
				},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: cfg.Hostname,
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}
		mta.HandleClient(proto)
		c.So(proto.GetState().Hostname, c.ShouldEqual, "some.sender")
	})

	c.Convey("Testing answers for HELO and close connection.", t, func(ctx c.C) {
		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.HeloCmd{
					Domain: "some.sender",
				},
				nil,
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: cfg.Hostname,
				},
			},
		}
		mta.HandleClient(proto)

	})

	c.Convey("Testing answers for EHLO and QUIT.", t, func(ctx c.C) {

		// Test connection with EHLO and QUIT
		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.EhloCmd{
					Domain: "some.sender",
				},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.MultiAnswer{
					Status: smtp.Ok,
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}
		mta.HandleClient(proto)
	})

	c.Convey("Testing answers for EHLO and close connection.", t, func(ctx c.C) {
		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.EhloCmd{
					Domain: "some.sender.ehlo",
				},
				nil,
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.MultiAnswer{
					Status: smtp.Ok,
				},
			},
		}
		mta.HandleClient(proto)
		c.So(proto.GetState().Hostname, c.ShouldEqual, "some.sender.ehlo")
	})
}

// Test answers if we are given a sequence of MAIL,RCPT,DATA commands.
func TestMailAnswersCorrectSequence(t *testing.T) {
	cfg := Config{
		Hostname: "home.sweet.home",
	}

	mta := New(cfg, HandlerFunc(dummyHandler))
	if mta == nil {
		t.Fatal("Could not create mta server")
	}

	c.Convey("Testing correct sequence of MAIL,RCPT,DATA commands.", t, func(ctx c.C) {

		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.HeloCmd{
					Domain: "some.sender",
				},
				smtp.MailCmd{
					From: getMailWithoutError("someone@somewhere.test"),
				},
				smtp.RcptCmd{
					To: getMailWithoutError("guy1@somewhere.test"),
				},
				smtp.RcptCmd{
					To: getMailWithoutError("guy2@somewhere.test"),
				},
				smtp.DataCmd{
					R: *smtp.NewDataReader(bufio.NewReader(bytes.NewReader([]byte("Some test email\n.\n")))),
				},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: cfg.Hostname,
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: "OK",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: "OK",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: "OK",
				},
				smtp.Answer{
					Status:  smtp.StartData,
					Message: "OK",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: "OK",
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}
		mta.HandleClient(proto)
	})

	c.Convey("Testing wrong sequence of MAIL,RCPT,DATA commands.", t, func(ctx c.C) {
		c.Convey("RCPT before MAIL", func() {
			proto := &testProtocol{
				t:   t,
				ctx: ctx,
				cmds: []smtp.Cmd{
					smtp.HeloCmd{
						Domain: "some.sender",
					},
					smtp.RcptCmd{
						To: getMailWithoutError("guy1@somewhere.test"),
					},
					smtp.QuitCmd{},
				},
				answers: []interface{}{
					smtp.Answer{
						Status:  smtp.Ready,
						Message: cfg.Hostname + " Service Ready",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: cfg.Hostname,
					},
					smtp.Answer{
						Status:  smtp.BadSequence,
						Message: "Need mail before RCPT",
					},
					smtp.Answer{
						Status:  smtp.Closing,
						Message: "Bye!",
					},
				},
			}
			mta.HandleClient(proto)
		})

		c.Convey("DATA before MAIL", func() {
			proto := &testProtocol{
				t:   t,
				ctx: ctx,
				cmds: []smtp.Cmd{
					smtp.HeloCmd{
						Domain: "some.sender",
					},
					smtp.DataCmd{},
					smtp.QuitCmd{},
				},
				answers: []interface{}{
					smtp.Answer{
						Status:  smtp.Ready,
						Message: cfg.Hostname + " Service Ready",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: cfg.Hostname,
					},
					smtp.Answer{
						Status:  smtp.BadSequence,
						Message: "Need mail before DATA",
					},
					smtp.Answer{
						Status:  smtp.Closing,
						Message: "Bye!",
					},
				},
			}
			mta.HandleClient(proto)
		})

		c.Convey("DATA before RCPT", func() {
			proto := &testProtocol{
				t:   t,
				ctx: ctx,
				cmds: []smtp.Cmd{
					smtp.HeloCmd{
						Domain: "some.sender",
					},
					smtp.MailCmd{
						From: getMailWithoutError("guy@somewhere.test"),
					},
					smtp.DataCmd{},
					smtp.QuitCmd{},
				},
				answers: []interface{}{
					smtp.Answer{
						Status:  smtp.Ready,
						Message: cfg.Hostname + " Service Ready",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: cfg.Hostname,
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.BadSequence,
						Message: "Need RCPT before DATA",
					},
					smtp.Answer{
						Status:  smtp.Closing,
						Message: "Bye!",
					},
				},
			}
			mta.HandleClient(proto)
		})

		c.Convey("Multiple MAIL commands.", func() {
			proto := &testProtocol{
				t:   t,
				ctx: ctx,
				cmds: []smtp.Cmd{
					smtp.HeloCmd{
						Domain: "some.sender",
					},
					smtp.MailCmd{
						From: getMailWithoutError("guy@somewhere.test"),
					},
					smtp.RcptCmd{
						To: getMailWithoutError("someone@somewhere.test"),
					},
					smtp.MailCmd{
						From: getMailWithoutError("someguy@somewhere.test"),
					},
					smtp.QuitCmd{},
				},
				answers: []interface{}{
					smtp.Answer{
						Status:  smtp.Ready,
						Message: cfg.Hostname + " Service Ready",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: cfg.Hostname,
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.BadSequence,
						Message: "Sender already specified",
					},
					smtp.Answer{
						Status:  smtp.Closing,
						Message: "Bye!",
					},
				},
			}
			mta.HandleClient(proto)
		})

	})
}

// Tests if our state gets reset correctly.
func TestReset(t *testing.T) {
	cfg := Config{
		Hostname: "home.sweet.home",
	}

	mta := New(cfg, HandlerFunc(dummyHandler))
	if mta == nil {
		t.Fatal("Could not create mta server")
	}

	c.Convey("Testing reset", t, func(ctx c.C) {

		c.Convey("Test reset after sending mail.", func() {
			proto := &testProtocol{
				t:   t,
				ctx: ctx,
				cmds: []smtp.Cmd{
					smtp.HeloCmd{
						Domain: "some.sender",
					},
					smtp.MailCmd{
						From: getMailWithoutError("someone@somewhere.test"),
					},
					smtp.RcptCmd{
						To: getMailWithoutError("guy1@somewhere.test"),
					},
					smtp.DataCmd{
						R: *smtp.NewDataReader(bufio.NewReader(bytes.NewReader([]byte("Some email content\n.\n")))),
					},
					smtp.RcptCmd{
						To: getMailWithoutError("someguy@somewhere.test"),
					},
					smtp.QuitCmd{},
				},
				answers: []interface{}{
					smtp.Answer{
						Status:  smtp.Ready,
						Message: cfg.Hostname + " Service Ready",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: cfg.Hostname,
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.StartData,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.BadSequence,
						Message: "Need mail before RCPT",
					},
					smtp.Answer{
						Status:  smtp.Closing,
						Message: "Bye!",
					},
				},
			}
			mta.HandleClient(proto)
		})

		c.Convey("Manually reset", func() {
			proto := &testProtocol{
				t:   t,
				ctx: ctx,
				cmds: []smtp.Cmd{
					smtp.HeloCmd{
						Domain: "some.sender",
					},
					smtp.MailCmd{
						From: getMailWithoutError("someone@somewhere.test"),
					},
					smtp.RcptCmd{
						To: getMailWithoutError("guy1@somewhere.test"),
					},
					smtp.RsetCmd{},
					smtp.MailCmd{
						From: getMailWithoutError("someone@somewhere.test"),
					},
					smtp.RcptCmd{
						To: getMailWithoutError("guy1@somewhere.test"),
					},
					smtp.DataCmd{
						R: *smtp.NewDataReader(bufio.NewReader(bytes.NewReader([]byte("Some email\n.\n")))),
					},
					smtp.QuitCmd{},
				},
				answers: []interface{}{
					smtp.Answer{
						Status:  smtp.Ready,
						Message: cfg.Hostname + " Service Ready",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: cfg.Hostname,
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.StartData,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Closing,
						Message: "Bye!",
					},
				},
			}
			mta.HandleClient(proto)
		})

		// EHLO should reset state.
		c.Convey("Reset with EHLO", func() {
			proto := &testProtocol{
				t:   t,
				ctx: ctx,
				cmds: []smtp.Cmd{
					smtp.EhloCmd{
						Domain: "some.sender",
					},
					smtp.MailCmd{
						From: getMailWithoutError("someone@somewhere.test"),
					},
					smtp.RcptCmd{
						To: getMailWithoutError("guy1@somewhere.test"),
					},
					smtp.EhloCmd{
						Domain: "some.sender",
					},
					smtp.MailCmd{
						From: getMailWithoutError("someone@somewhere.test"),
					},
					smtp.RcptCmd{
						To: getMailWithoutError("guy1@somewhere.test"),
					},
					smtp.DataCmd{
						R: *smtp.NewDataReader(bufio.NewReader(bytes.NewReader([]byte("Some email\n.\n")))),
					},
					smtp.QuitCmd{},
				},
				answers: []interface{}{
					smtp.Answer{
						Status:  smtp.Ready,
						Message: cfg.Hostname + " Service Ready",
					},
					smtp.MultiAnswer{
						Status: smtp.Ok,
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.MultiAnswer{
						Status: smtp.Ok,
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.StartData,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Ok,
						Message: "OK",
					},
					smtp.Answer{
						Status:  smtp.Closing,
						Message: "Bye!",
					},
				},
			}
			mta.HandleClient(proto)
		})

	})
}

// Tests answers if we send an unknown command.
func TestAnswersUnknownCmd(t *testing.T) {
	cfg := Config{
		Hostname: "home.sweet.home",
	}

	mta := New(cfg, HandlerFunc(dummyHandler))
	if mta == nil {
		t.Fatal("Could not create mta server")
	}

	c.Convey("Testing answers for unknown cmds.", t, func(ctx c.C) {
		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.HeloCmd{
					Domain: "some.sender",
				},
				smtp.UnknownCmd{
					Cmd: "someinvalidcmd",
				},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: "OK",
				},
				smtp.Answer{
					Status:  smtp.SyntaxError,
					Message: cfg.Hostname,
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}
		mta.HandleClient(proto)
	})
}

// Tests STARTTLS
func TestStartTls(t *testing.T) {
	cfg := Config{
		Hostname: "home.sweet.home",
	}

	mta := New(cfg, HandlerFunc(dummyHandler))
	mta.TlsConfig = &tls.Config{}
	if mta == nil {
		t.Fatal("Could not create mta server")
	}

	c.Convey("Testing STARTTLS", t, func(ctx c.C) {
		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.EhloCmd{
					Domain: "some.sender",
				},
				smtp.StartTlsCmd{},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.MultiAnswer{
					Status: smtp.Ok,
				},
				smtp.Answer{
					Status: smtp.Ready,
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}
		proto.expectTLS = true
		mta.HandleClient(proto)
	})

	c.Convey("Testing if STARTTLS resets state", t, func(ctx c.C) {
		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.EhloCmd{
					Domain: "some.sender",
				},
				smtp.MailCmd{
					From: getMailWithoutError("someone@somewhere.test"),
				},
				smtp.StartTlsCmd{},
				smtp.MailCmd{
					From: getMailWithoutError("someone@somewhere.test"),
				},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.MultiAnswer{
					Status: smtp.Ok,
				},
				smtp.Answer{
					Status: smtp.Ok,
				},
				smtp.Answer{
					Status: smtp.Ready,
				},
				smtp.Answer{
					Status: smtp.Ok,
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}
		proto.expectTLS = true
		mta.HandleClient(proto)
	})

	c.Convey("Testing if we can STARTTLS twice", t, func(ctx c.C) {
		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.EhloCmd{
					Domain: "some.sender",
				},
				smtp.StartTlsCmd{},
				smtp.StartTlsCmd{},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.MultiAnswer{
					Status: smtp.Ok,
				},
				smtp.Answer{
					Status: smtp.Ready,
				},
				smtp.Answer{
					Status: smtp.NotImplemented,
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}
		proto.expectTLS = true
		mta.HandleClient(proto)
	})
}

// Simple test for representation of SessionId
func TestSessionId(t *testing.T) {
	c.Convey("Testing Session ID String()", t, func() {
		id := smtp.Id{Timestamp: 1446302030, Counter: 42}
		c.So(id.String(), c.ShouldEqual, "5634d14e2a")

		id = smtp.Id{Timestamp: 2147483648, Counter: 4294967295}
		c.So(id.String(), c.ShouldEqual, "80000000ffffffff")
	})
}

// Test whether error in handle() is correctly handled in the DATA command
func TestErrorInHandler(t *testing.T) {
	cfg := Config{
		Hostname: "home.sweet.home",
	}

	mta := New(cfg, HandlerFunc(dummyHandlerError))
	if mta == nil {
		t.Fatal("Could not create mta server")
	}

	c.Convey("Testing with error in handle()", t, func(ctx c.C) {

		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.HeloCmd{
					Domain: "some.sender",
				},
				smtp.MailCmd{
					From: getMailWithoutError("someone@somewhere.test"),
				},
				smtp.RcptCmd{
					To: getMailWithoutError("guy1@somewhere.test"),
				},
				smtp.DataCmd{
					R: *smtp.NewDataReader(bufio.NewReader(bytes.NewReader([]byte("Some test email\n.\n")))),
				},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: cfg.Hostname,
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: "OK",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: "OK",
				},
				smtp.Answer{
					Status:  smtp.StartData,
					Message: "OK",
				},
				smtp.Answer{
					Status:  smtp.SMTPErrorPermanentMailboxNotAvailable.Status,
					Message: smtp.SMTPErrorPermanentMailboxNotAvailable.Message,
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}
		mta.HandleClient(proto)
	})
}

func TestAuth(t *testing.T) {
	cfg := Config{
		Hostname: "home.sweet.home",
	}

	mta := New(cfg, HandlerFunc(dummyHandler))
	if mta == nil {
		t.Fatal("Could not create mta server")
	}

	mta.AuthBackend = NewAuthBackendMemory(map[string]string{"some-username": "password1234"})

	c.Convey("Testing AUTH with correct credentials", t, func(ctx c.C) {

		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.HeloCmd{
					Domain: "some.sender",
				},
				smtp.AuthCmd{
					Mechanism:       "PLAIN",
					InitialResponse: "AHNvbWUtdXNlcm5hbWUAcGFzc3dvcmQxMjM0",
				},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: cfg.Hostname,
				},
				smtp.Answer{
					Status:  smtp.AuthenticationSucceeded,
					Message: "2.7.0 Authentication successful",
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}

		c.So(proto.GetState().Authenticated, c.ShouldEqual, false)

		mta.HandleClient(proto)
		c.So(proto.GetState().Hostname, c.ShouldEqual, "some.sender")
		c.So(proto.GetState().Authenticated, c.ShouldEqual, true)
	})

	c.Convey("Testing AUTH with incorrect credentials", t, func(ctx c.C) {

		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.HeloCmd{
					Domain: "some.sender",
				},
				smtp.AuthCmd{
					Mechanism:       "PLAIN",
					InitialResponse: "AHNvbWUtdXNlcm5hbWUAc29tZS1pbmNvcnJlY3QtcGFzc3dvcmQ=",
				},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: cfg.Hostname,
				},
				smtp.Answer{
					Status:  smtp.AuthenticationCredentialsInvalid,
					Message: "5.7.8  Authentication credentials invalid",
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}

		c.So(proto.GetState().Authenticated, c.ShouldEqual, false)

		mta.HandleClient(proto)
		c.So(proto.GetState().Hostname, c.ShouldEqual, "some.sender")
		c.So(proto.GetState().Authenticated, c.ShouldEqual, false)
	})

	c.Convey("Testing AUTH with credentials in different command", t, func(ctx c.C) {

		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.HeloCmd{
					Domain: "some.sender",
				},
				smtp.AuthCmd{
					Mechanism:       "PLAIN",
					InitialResponse: "",
					R:               *bufio.NewReader(bytes.NewReader([]byte("AHNvbWUtdXNlcm5hbWUAcGFzc3dvcmQxMjM0\r\n"))),
				},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: cfg.Hostname,
				},
				smtp.Answer{
					Status:  smtp.AuthenticationSucceeded,
					Message: "2.7.0 Authentication successful",
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}

		c.So(proto.GetState().Authenticated, c.ShouldEqual, false)

		mta.HandleClient(proto)
		c.So(proto.GetState().Hostname, c.ShouldEqual, "some.sender")
		c.So(proto.GetState().Authenticated, c.ShouldEqual, true)
	})

	c.Convey("Testing AUTH with unknown mechanism", t, func(ctx c.C) {

		proto := &testProtocol{
			t:   t,
			ctx: ctx,
			cmds: []smtp.Cmd{
				smtp.HeloCmd{
					Domain: "some.sender",
				},
				smtp.AuthCmd{
					Mechanism:       "SOME_UNKNOWN_MECHANISM",
					InitialResponse: "",
				},
				smtp.QuitCmd{},
			},
			answers: []interface{}{
				smtp.Answer{
					Status:  smtp.Ready,
					Message: cfg.Hostname + " Service Ready",
				},
				smtp.Answer{
					Status:  smtp.Ok,
					Message: cfg.Hostname,
				},
				smtp.Answer{
					Status:  smtp.UnrecognizedAuthenticationType,
					Message: "5.7.4 Unrecognized authentication type",
				},
				smtp.Answer{
					Status:  smtp.Closing,
					Message: "Bye!",
				},
			},
		}

		c.So(proto.GetState().Authenticated, c.ShouldEqual, false)

		mta.HandleClient(proto)
		c.So(proto.GetState().Hostname, c.ShouldEqual, "some.sender")
		c.So(proto.GetState().Authenticated, c.ShouldEqual, false)
	})

}
