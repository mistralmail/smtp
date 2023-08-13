package server

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"sync"
	"time"

	"github.com/gopistolet/gopistolet/helpers"
	"github.com/gopistolet/gopistolet/log"
	"github.com/gopistolet/smtp/smtp"
)

type Config struct {
	Ip          string
	Hostname    string
	Port        uint32
	TlsCert     string
	TlsKey      string
	Blacklist   helpers.Blacklist
	DisableAuth bool
}

// Session id

var globalCounter uint32 = 0
var globalCounterLock = &sync.Mutex{}

func generateSessionId() smtp.Id {
	globalCounterLock.Lock()
	defer globalCounterLock.Unlock()
	globalCounter++
	return smtp.Id{Timestamp: time.Now().Unix(), Counter: globalCounter}

}

// Handler is the interface that will be used when a mail was received.
type Handler interface {
	Handle(*smtp.State) error
}

// HandlerFunc is a wrapper to allow normal functions to be used as a handler.
type HandlerFunc func(*smtp.State) error

func (h HandlerFunc) Handle(state *smtp.State) error {
	return h(state)
}

// Server Represents an SMTP server
type Server struct {
	config Config
	// The handler to be called when a mail is received.
	MailHandler Handler
	// The config for tls connection. Nil if not supported.
	TlsConfig   *tls.Config
	AuthBackend AuthBackend
	// When shutting down this channel is closed, no new connections should be handled then.
	// But existing connections can continue untill quitC is closed.
	shutDownC chan bool
	// When this is closed existing connections should stop.
	quitC chan bool
	wg    sync.WaitGroup
}

// New Create a new SMTP server that doesn't handle the protocol.
func New(c Config, h Handler) *Server {
	mta := &Server{
		config:      c,
		MailHandler: h,
		quitC:       make(chan bool),
		shutDownC:   make(chan bool),
	}

	if c.TlsCert != "" && c.TlsKey != "" {
		cert, err := tls.LoadX509KeyPair(c.TlsCert, c.TlsKey)
		if err != nil {
			log.Warnf("Could not load keypair: %v", err)
		} else {
			mta.TlsConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
		}
	}

	// TODO what if authbackend is nil?

	return mta
}

func (s *Server) Stop() {
	log.Printf("Received stop command. Sending shutdown event...")
	close(s.shutDownC)
	// Give existing connections some time to finish.
	t := time.Duration(10)
	log.Printf("Waiting for a maximum of %d seconds...", t)
	time.Sleep(t * time.Second)
	log.Printf("Sending force quit event...")
	close(s.quitC)
}

func (s *Server) hasTls() bool {
	return s.TlsConfig != nil
}

// Same as the Mta struct but has methods for handling socket connections.
type DefaultMta struct {
	Server *Server
}

// NewDefault Create a new SMTP server with a
// socket protocol implementation.
func NewDefault(c Config, h Handler) *DefaultMta {
	mta := &DefaultMta{
		Server: New(c, h),
	}
	if mta == nil {
		return nil
	}

	return mta
}

func (s *DefaultMta) Stop() {
	s.Server.Stop()
}

func (s *DefaultMta) ListenAndServe() error {
	log.Printf("Starting SMTP server at port %d", s.Server.config.Port)
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.Server.config.Ip, s.Server.config.Port))
	if err != nil {
		log.Errorf("Could not start listening: %v", err)
		return err
	}

	// Close the listener so that listen well return from ln.Accept().
	go func() {
		_, ok := <-s.Server.shutDownC
		if !ok {
			ln.Close()
		}
	}()

	err = s.listen(ln)
	log.Printf("Waiting for connections to close...")
	s.Server.wg.Wait()
	return err
}

func (s *DefaultMta) listen(ln net.Listener) error {
	defer ln.Close()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.Printf("Accept error: %v", err)
				continue
			}
			// Assume this means listener was closed.
			if noe, ok := err.(*net.OpError); ok && !noe.Temporary() {
				log.Printf("Listener is closed, stopping listen loop...")
				return nil
			}
			return err
		}

		s.Server.wg.Add(1)
		go s.serve(c)
	}

}

func (s *DefaultMta) serve(c net.Conn) {
	defer s.Server.wg.Done()

	proto := smtp.NewMtaProtocol(c)
	if proto == nil {
		log.Errorf("Could not create Mta protocol")
		c.Close()
		return
	}
	s.Server.HandleClient(proto)
}

// HandleClient Start communicating with a client
func (s *Server) HandleClient(proto smtp.Protocol) {
	//log.Printf("Received connection")

	// Hold state for this client connection
	state := proto.GetState()
	state.Reset()
	state.SessionId = generateSessionId()
	state.Ip = proto.GetIP()

	log.WithFields(log.Fields{
		"SessionId": state.SessionId.String(),
		"Ip":        state.Ip.String(),
	}).Debug("Received connection")

	if s.config.Blacklist != nil {
		if s.config.Blacklist.CheckIp(state.Ip.String()) {
			log.WithFields(log.Fields{
				"SessionId": state.SessionId.String(),
				"Ip":        state.Ip.String(),
			}).Warn("IP found in Blacklist, closing handler")
			proto.Close()
		} else {
			log.WithFields(log.Fields{
				"SessionId": state.SessionId.String(),
				"Ip":        state.Ip.String(),
			}).Debug("IP not found in Blacklist")
		}
	}

	// Start with welcome message
	proto.Send(smtp.Answer{
		Status:  smtp.Ready,
		Message: s.config.Hostname + " Service Ready",
	})

	var c *smtp.Cmd
	var err error

	quit := false
	cmdC := make(chan bool)

	nextCmd := func() bool {
		go func() {
			for {
				c, err = proto.GetCmd()

				if err != nil {
					if err == smtp.ErrLtl {
						proto.Send(smtp.Answer{
							Status:  smtp.SyntaxError,
							Message: "Line too long.",
						})
					} else {
						// Not a line too long error. What to do?
						cmdC <- true
						return
					}
				} else {
					break
				}
			}
			cmdC <- false
		}()

		select {
		case _, ok := <-s.quitC:
			if !ok {
				proto.Send(smtp.Answer{
					Status:  smtp.ShuttingDown,
					Message: "Server is going down.",
				})
				return true
			}
		case q := <-cmdC:
			return q

		}

		return false
	}

	quit = nextCmd()

	for quit == false {

		//log.Printf("Received cmd: %#v", *c)

		switch cmd := (*c).(type) {
		case smtp.HeloCmd:
			state.Hostname = cmd.Domain
			proto.Send(smtp.Answer{
				Status:  smtp.Ok,
				Message: s.config.Hostname,
			})

		case smtp.EhloCmd:
			state.Reset()
			state.Hostname = cmd.Domain

			messages := []string{s.config.Hostname, "8BITMIME"}
			if s.hasTls() && !state.Secure {
				messages = append(messages, "STARTTLS")
			}

			if !s.config.DisableAuth && s.AuthBackend != nil {
				messages = append(messages, "AUTH PLAIN")
			}

			messages = append(messages, "OK")

			proto.Send(smtp.MultiAnswer{
				Status:   smtp.Ok,
				Messages: messages,
			})

		case smtp.QuitCmd:
			proto.Send(smtp.Answer{
				Status:  smtp.Closing,
				Message: "Bye!",
			})
			quit = true

		case smtp.MailCmd:
			if ok, reason := state.CanReceiveMail(); !ok {
				proto.Send(smtp.Answer{
					Status:  smtp.BadSequence,
					Message: reason,
				})
				break
			}
			if !s.config.DisableAuth && !state.Authenticated {
				proto.Send(smtp.Answer{
					Status:  smtp.AuthenticationRequired,
					Message: "Authentication Required",
				})
				break
			}

			state.From = cmd.From
			state.EightBitMIME = cmd.EightBitMIME
			message := "Sender"
			if state.EightBitMIME {
				message += " and 8BITMIME"
			}
			message += " ok"

			proto.Send(smtp.Answer{
				Status:  smtp.Ok,
				Message: message,
			})

		case smtp.RcptCmd:
			if ok, reason := state.CanReceiveRcpt(); !ok {
				proto.Send(smtp.Answer{
					Status:  smtp.BadSequence,
					Message: reason,
				})
				break
			}

			state.To = append(state.To, cmd.To)

			if !s.config.DisableAuth {
				// TODO check if to/from email address allowed
				if ok, reason := state.AuthMatchesRcptAndMail(); !ok {
					proto.Send(smtp.Answer{
						Status:  smtp.SMTPErrorPermanentMailboxNameNotAllowed.Status,
						Message: reason,
					})
					state.Reset()
					break
				}
			}

			proto.Send(smtp.Answer{
				Status:  smtp.Ok,
				Message: "OK",
			})

		case smtp.DataCmd:
			if ok, reason := state.CanReceiveData(); !ok {
				/*
					RFC 5321 3.3

					If there was no MAIL, or no RCPT, command, or all such commands were
					rejected, the server MAY return a "command out of sequence" (503) or
					"no valid recipients" (554) reply in response to the DATA command.
					If one of those replies (or any other 5yz reply) is received, the
					client MUST NOT send the message data; more generally, message data
					MUST NOT be sent unless a 354 reply is received.
				*/
				proto.Send(smtp.Answer{
					Status:  smtp.BadSequence,
					Message: reason,
				})
				break
			}

			message := "Start"
			if state.EightBitMIME {
				message += " 8BITMIME"
			}
			message += " mail input; end with <CRLF>.<CRLF>"
			proto.Send(smtp.Answer{
				Status:  smtp.StartData,
				Message: message,
			})

		tryAgain:
			tmpData, err := ioutil.ReadAll(&cmd.R)
			state.Data = append(state.Data, tmpData...)
			if err == smtp.ErrLtl {
				proto.Send(smtp.Answer{
					// SyntaxError or 552 error? or something else?
					Status:  smtp.SyntaxError,
					Message: "Line too long",
				})
				goto tryAgain
			} else if err == smtp.ErrIncomplete {
				// I think this can only happen on a socket if it gets closed before receiving the full data.
				proto.Send(smtp.Answer{
					Status:  smtp.SyntaxError,
					Message: "Could not parse mail data",
				})
				state.Reset()
				break

			} else if err != nil {
				//panic(err)
				log.WithFields(log.Fields{
					"SessionId": state.SessionId.String(),
				}).Panic(err)
			}

			// Handle mail
			err = s.MailHandler.Handle(state)
			if err != nil {
				smtpErr, ok := err.(smtp.SMTPError)
				if ok {
					// known SMTP error, just return it
					proto.Send(smtp.Answer(smtpErr))
				} else {
					// unknown internal server error
					proto.Send(smtp.Answer{Status: 451, Message: "local error: something went wrong"})
				}
			} else {
				// mail successfully handled!
				proto.Send(smtp.Answer{
					Status:  smtp.Ok,
					Message: "Mail delivered",
				})
			}

			// Reset state after mail was handled so we can start from a clean slate.
			state.Reset()

		case smtp.RsetCmd:
			state.Reset()
			proto.Send(smtp.Answer{
				Status:  smtp.Ok,
				Message: "OK",
			})

		case smtp.StartTlsCmd:
			if !s.hasTls() {
				proto.Send(smtp.Answer{
					Status:  smtp.NotImplemented,
					Message: "STARTTLS is not implemented",
				})
				break
			}

			if state.Secure {
				proto.Send(smtp.Answer{
					Status:  smtp.NotImplemented,
					Message: "Already in TLS mode",
				})
				break
			}

			proto.Send(smtp.Answer{
				Status:  smtp.Ready,
				Message: "Ready for TLS handshake",
			})

			err := proto.StartTls(s.TlsConfig)
			if err != nil {
				log.WithFields(log.Fields{
					"Ip":        state.Ip.String(),
					"SessionId": state.SessionId.String(),
				}).Warningf("Could not enable TLS: %v", err)
				break
			}

			log.WithFields(log.Fields{
				"Ip":        state.Ip.String(),
				"SessionId": state.SessionId.String(),
			}).Debug("TLS enabled")
			state.Reset()
			state.Secure = true

		case smtp.NoopCmd:
			proto.Send(smtp.Answer{
				Status:  smtp.Ok,
				Message: "OK",
			})

		case smtp.VrfyCmd, smtp.ExpnCmd, smtp.SendCmd, smtp.SomlCmd, smtp.SamlCmd:
			proto.Send(smtp.Answer{
				Status:  smtp.NotImplemented,
				Message: "Command not implemented",
			})

		case smtp.InvalidCmd:
			// TODO: Is this correct? An InvalidCmd is a known command with
			// invalid arguments. So we should send smtp.SyntaxErrorParam?
			// Is InvalidCmd a good name for this kind of error?
			proto.Send(smtp.Answer{
				Status:  smtp.SyntaxErrorParam,
				Message: cmd.Info,
			})

		case smtp.UnknownCmd:
			proto.Send(smtp.Answer{
				Status:  smtp.SyntaxError,
				Message: "Command not recognized",
			})

		case smtp.AuthCmd:

			// make sure to add auth mechanisms to the EHLO command
			if cmd.Mechanism != "PLAIN" {
				proto.Send(smtp.Answer{
					Status:  smtp.UnrecognizedAuthenticationType,
					Message: "5.7.4 Unrecognized authentication type",
				})
				break
			}

			initialResponse := ""

			// If no credentials are not present in AUTH command, prompt the client for them.
			if cmd.InitialResponse == "" {
				//tmpData, err := ioutil.ReadAll(&cmd.R)
				tmpData, err := smtp.ReadUntill('\n', smtp.MAX_CMD_LINE, &cmd.R)
				initialResponse = string(tmpData)
				if err != nil {
					// I think this can only happen on a socket if it gets closed before receiving the full data.
					log.WithFields(log.Fields{
						"SessionId": state.SessionId.String(),
					}).Warnln(err)
					proto.Send(smtp.Answer{
						Status:  smtp.MalformedAuthInput,
						Message: "Could not parse auth data",
					})
					break

				}
			} else {
				initialResponse = cmd.InitialResponse
			}

			authorizationIdentity, authenticationIdenity, password, err := smtp.ParseAuthPlainInitialRespone(initialResponse)
			if err != nil {

				log.WithFields(log.Fields{
					"Ip":        state.Ip.String(),
					"SessionId": state.SessionId.String(),
				}).Warningf("Could not decode base64: %v", err)

				proto.Send(smtp.Answer{
					Status:  smtp.SyntaxErrorParam,
					Message: "Invalid initial response for PLAIN auth",
				})

				break

			}

			log.WithFields(log.Fields{
				"authorization-identity":  authorizationIdentity,
				"authentication-identity": authenticationIdenity,
				// "password":                "password",
				// let's not log user passwords....
			}).Debugln("received auth")

			// Check if AuthBackend is initialized
			if s.AuthBackend == nil {
				log.Errorln("AuthBackend not initialized")
				proto.Send(smtp.Answer{
					Status:  smtp.TemporaryAuthenticationFailure,
					Message: "4.7.0  Temporary authentication failure",
				})
				break
			}

			user, err := s.AuthBackend.Login(state, authenticationIdenity, password)
			if err == ErrInvalidCredentials {
				// Invalid credentials
				state.Authenticated = false

				log.WithFields(log.Fields{
					"Ip":        state.Ip.String(),
					"SessionId": state.SessionId.String(),
				}).Printf("invalid auth for user: %s", authenticationIdenity)

				proto.Send(smtp.Answer{
					Status:  smtp.AuthenticationCredentialsInvalid,
					Message: "5.7.8  Authentication credentials invalid",
				})

				break
			}
			if err != nil {
				// Other error
				state.Authenticated = false

				log.WithFields(log.Fields{
					"Ip":        state.Ip.String(),
					"SessionId": state.SessionId.String(),
				}).Printf("authentication failed for user: %s with error: %v", authenticationIdenity, err)

				proto.Send(smtp.Answer{
					Status:  smtp.TemporaryAuthenticationFailure,
					Message: "4.7.0 Temporary authentication failure",
				})

				break
			}

			// Valid auth

			state.Authenticated = true
			state.User = user

			log.WithFields(log.Fields{
				"Ip":        state.Ip.String(),
				"SessionId": state.SessionId.String(),
			}).Printf("valid auth for user: %s", authenticationIdenity)

			proto.Send(smtp.Answer{
				Status:  smtp.AuthenticationSucceeded,
				Message: "2.7.0 Authentication successful",
			})

			//initialResponseText := string(initialResponseByte)
			//log.Fatalf("initial-response: %v", initialResponseText)

		default:
			// TODO: We get here if the switch does not handle all Cmd's defined
			// in protocol.go. That means we forgot to add it here. This should ideally
			// be checked at compile time. But if we get here anyway we probably shouldn't
			// crash...
			log.Fatalf("Command not implemented: %#v", cmd)
		}

		if quit {
			break
		}

		quit = nextCmd()
	}

	proto.Close()
	log.WithFields(log.Fields{
		"SessionId": state.SessionId.String(),
		"Ip":        state.Ip.String(),
	}).Debug("Closed connection")
}
