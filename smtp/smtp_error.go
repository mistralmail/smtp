package smtp

import "fmt"

// SMTPError describes an SMTP error with a Status and a Message
// list of SMTP errors: https://datatracker.ietf.org/doc/html/rfc5321#section-4.2.3
type SMTPError Answer

func (err SMTPError) Error() string {
	return fmt.Sprintf("smtp error %d %q", err.Status, err.Message)
}

var (
	// 4yz  Transient Negative Completion reply
	// The command was not accepted, and the requested action did not
	// occur.  However, the error condition is temporary, and the action
	// may be requested again.  The sender should return to the beginning
	// of the command sequence (if any).  It is difficult to assign a
	// meaning to "transient" when two different sites (receiver- and
	// sender-SMTP agents) must agree on the interpretation.  Each reply
	// in this category might have a different time value, but the SMTP
	// client SHOULD try again.  A rule of thumb to determine whether a
	// reply fits into the 4yz or the 5yz category (see below) is that
	// replies are 4yz if they can be successful if repeated without any
	// change in command form or in properties of the sender or receiver
	// (that is, the command is repeated identically and the receiver
	// does not put up a new implementation).
	SMTPErrorTransientServiceNotAvailable           = SMTPError{Status: 421, Message: "Service not available, closing transmission channel"}
	SMTPErrorTransientMailboxNotAvailable           = SMTPError{Status: 450, Message: "Requested mail action not taken: mailbox unavailable"}
	SMTPErrorTransientLocalError                    = SMTPError{Status: 451, Message: "Requested action aborted: local error in processing"}
	SMTPErrorTransientInsufficientSystemStorage     = SMTPError{Status: 452, Message: "Requested action not taken: insufficient system storage"}
	SMTPErrorTransientUnableToAccommodateParameters = SMTPError{Status: 455, Message: "Server unable to accommodate parameters"}

	// 5yz  Permanent Negative Completion reply
	// The command was not accepted and the requested action did not
	// occur.  The SMTP client SHOULD NOT repeat the exact request (in
	// the same sequence).  Even some "permanent" error conditions can be
	// corrected, so the human user may want to direct the SMTP client to
	// reinitiate the command sequence by direct action at some point in
	// the future (e.g., after the spelling has been changed, or the user
	// has altered the account status).
	SMTPErrorPermanentSyntaxError             = SMTPError{Status: 500, Message: "Syntax error, command unrecognized"}
	SMTPErrorPermanentSyntaxErrorInParameters = SMTPError{Status: 501, Message: "Syntax error in parameters or arguments"}
	SMTPErrorPermanentCommandNotImplemented   = SMTPError{Status: 502, Message: "Command not implemented"}
	SMTPErrorPermanentBadSequence             = SMTPError{Status: 503, Message: "Bad sequence of commands"}
	SMTPErrorPermanentParameterNotImplemented = SMTPError{Status: 504, Message: "Command parameter not implemented"}
	SMTPErrorPermanentMailboxNotAvailable     = SMTPError{Status: 550, Message: "Requested action not taken: mailbox unavailable"}
	SMTPErrorPermanentUserNotLocal            = SMTPError{Status: 551, Message: "User not local"}
	SMTPErrorPermanentExceededStorage         = SMTPError{Status: 552, Message: "Requested mail action aborted: exceeded storage allocation"}
	SMTPErrorPermanentMailboxNameNotAllowed   = SMTPError{Status: 553, Message: "Requested action not taken: mailbox name not allowed"}
	SMTPErrorPermanentTransactionFailed       = SMTPError{Status: 554, Message: "Transaction failed"}
	SMTPErrorMailParametersNotImplemented     = SMTPError{Status: 555, Message: "MAIL FROM/RCPT TO parameters not recognized or not implemented"}
)
