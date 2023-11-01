package smtp

import (
	"net/mail"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestState(t *testing.T) {

	Convey("AddHeader()", t, func() {

		// Create a state object with a message
		message := `From: sender@example.com
To: recipient@example.com
Subject: Test Subject

This is the body of the email.`

		state := &State{
			Data: []byte(message),
		}

		// Add the header
		state.AddHeader("MessageId", "some-value@localhost")

		// If we now parse the state data again, it should be valid and the header should be present
		parsedMessage, err := mail.ReadMessage(strings.NewReader(string(state.Data)))
		So(err, ShouldBeNil)
		So(parsedMessage.Header.Get("MessageId"), ShouldEqual, "some-value@localhost")

		// and make sure the rest is also still there...
		So(parsedMessage.Header.Get("From"), ShouldEqual, "sender@example.com")

	})

	Convey("GetHeader()", t, func() {
		// Create a state object with a message
		message := `From: sender@example.com
To: recipient@example.com
X-Spam-Score: -5.1
Subject: Test Subject

This is the body of the email.`

		state := &State{
			Data: []byte(message),
		}

		headerValue, ok := state.GetHeader("To")
		So(ok, ShouldBeTrue)
		So(headerValue, ShouldEqual, "recipient@example.com")

		headerValue, ok = state.GetHeader("to")
		So(ok, ShouldBeTrue)
		So(headerValue, ShouldEqual, "recipient@example.com")

		headerValue, ok = state.GetHeader("TO")
		So(ok, ShouldBeTrue)
		So(headerValue, ShouldEqual, "recipient@example.com")

		headerValue, ok = state.GetHeader("From")
		So(ok, ShouldBeTrue)
		So(headerValue, ShouldEqual, "sender@example.com")

		headerValue, ok = state.GetHeader("X-Spam-Score")
		So(ok, ShouldBeTrue)
		So(headerValue, ShouldEqual, "-5.1")

		_, ok = state.GetHeader("Date")
		So(ok, ShouldBeFalse)
	})
}
