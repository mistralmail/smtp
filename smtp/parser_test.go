package smtp

import (
	"bufio"
	_ "fmt"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestParser(t *testing.T) {

	Convey("Testing parser", t, func() {
		commands := ""
		commands += "HELO relay.example.org\r\n"
		commands += "HeLo relay.example.org\r\n"
		commands += "helo relay.example.org\r\n"
		commands += "helO relay.example.org\r\n"
		commands += "EHLO other.example.org\r\n"
		commands += "MAIL FROM:<bob@example.org>\r\n"
		commands += "MAIL FROM:<BOB@example.org>\r\n"
		commands += "mail FROM:<bob@example.org>\r\n"
		commands += "MAIL FROM:<bob@example.org> body=8BITMIME\r\n"
		commands += "MAIL FROM:<bob@example.org> BODY=8bitmime\r\n"
		commands += "MAIL FROM:<bob@example.org> BODY=7bit\r\n"
		commands += "RCPT TO:<alice@example.com>\r\n"
		commands += "RCPT TO:<theboss@example.com>\r\n"
		commands += "RCPT to:<theboss@example.com>\r\n"
		commands += "rcpt to:<Theboss@example.com>\r\n"
		commands += "SEND\r\n"
		commands += "SOML\r\n"
		commands += "SAML\r\n"
		commands += "RSET\r\n"
		commands += "VRFY jones\r\n"
		commands += "EXPN staff\r\n"
		commands += "NOOP\r\n"
		commands += "QUIT\r\n"
		commands += "AUTH PLAIN\r\n"
		commands += "AUTH PLAIN dGVzdAB0ZXN0ADEyMzQ=\r\n"

		br := bufio.NewReader(strings.NewReader(commands))

		p := parser{}

		expectedCommands := []Cmd{
			HeloCmd{Domain: "relay.example.org"},
			HeloCmd{Domain: "relay.example.org"},
			HeloCmd{Domain: "relay.example.org"},
			HeloCmd{Domain: "relay.example.org"},
			EhloCmd{Domain: "other.example.org"},
			MailCmd{From: &MailAddress{Address: "bob@example.org"}},
			MailCmd{From: &MailAddress{Address: "BOB@example.org"}},
			MailCmd{From: &MailAddress{Address: "bob@example.org"}},
			MailCmd{From: &MailAddress{Address: "bob@example.org"}, EightBitMIME: true},
			MailCmd{From: &MailAddress{Address: "bob@example.org"}, EightBitMIME: true},
			MailCmd{From: &MailAddress{Address: "bob@example.org"}},
			RcptCmd{To: &MailAddress{Address: "alice@example.com"}},
			RcptCmd{To: &MailAddress{Address: "theboss@example.com"}},
			RcptCmd{To: &MailAddress{Address: "theboss@example.com"}},
			RcptCmd{To: &MailAddress{Address: "Theboss@example.com"}},
			SendCmd{},
			SomlCmd{},
			SamlCmd{},
			RsetCmd{},
			VrfyCmd{Param: "jones"},
			ExpnCmd{ListName: "staff"},
			NoopCmd{},
			QuitCmd{},
			AuthCmd{Mechanism: "PLAIN"},
			AuthCmd{Mechanism: "PLAIN", InitialResponse: "dGVzdAB0ZXN0ADEyMzQ="},
		}

		for _, expectedCommand := range expectedCommands {
			command, err := p.ParseCommand(br)
			So(err, ShouldEqual, nil)
			So(command, ShouldResemble, expectedCommand)
		}

	})

	Convey("Testing parser DATA cmd", t, func() {
		commands := ""
		commands += "DATA\r\n"
		commands += "quit\r\n"

		br := bufio.NewReader(strings.NewReader(commands))
		p := parser{}

		command, err := p.ParseCommand(br)
		So(err, ShouldEqual, nil)
		So(command, ShouldHaveSameTypeAs, DataCmd{})

		command, err = p.ParseCommand(br)
		So(err, ShouldEqual, nil)
		So(command, ShouldHaveSameTypeAs, QuitCmd{})

	})

	Convey("Testing parser with invalid commands", t, func() {

		commands := ""
		commands += "RCPT\r\n"
		commands += "helo\r\n"
		commands += "ehlo\r\n"
		commands += "\r\n"
		commands += "  \r\n"
		commands += "RCPT TO:some invalid email\r\n"
		commands += "rcpt :valid@mail.be\r\n"
		commands += "RCPT :valid@mail.be\r\n"
		commands += "RCPT TA:valid@mail.be\r\n"
		commands += "MAIL\r\n"
		commands += "MAIL from:some invalid email\r\n"
		commands += "MAIL :valid@mail.be\r\n"
		commands += "MAIL FROA:valid@mail.be\r\n"
		commands += "MAIL To some@invalid\r\n"
		commands += "MAIL FROM:some@valid.be BODY:8bitmime\r\n"
		commands += "UNKN some unknown command\r\n"

		br := bufio.NewReader(strings.NewReader(commands))

		p := parser{}

		expectedCommands := []Cmd{
			InvalidCmd{},
			InvalidCmd{},
			InvalidCmd{},
			UnknownCmd{},
			UnknownCmd{},
			InvalidCmd{},
			InvalidCmd{},
			InvalidCmd{},
			InvalidCmd{},
			InvalidCmd{},
			InvalidCmd{},
			InvalidCmd{},
			InvalidCmd{},
			InvalidCmd{},
			InvalidCmd{},
			UnknownCmd{},
		}

		for _, expectedCommand := range expectedCommands {
			command, err := p.ParseCommand(br)
			So(err, ShouldEqual, nil)
			So(command, ShouldHaveSameTypeAs, expectedCommand)
		}

	})

	Convey("Testing parseLine()", t, func() {

		tests := []struct {
			line string
			verb string
			args map[string]Argument
		}{
			{
				line: "HELO\r\n",
				verb: "HELO",
				args: map[string]Argument{},
			},
			{
				line: "HELO relay.example.org\r\n",
				verb: "HELO",
				args: map[string]Argument{"RELAY.EXAMPLE.ORG": Argument{Key: "relay.example.org"}},
			},
			{
				line: "MAIL FROM:<bob@example.org>\r\n",
				verb: "MAIL",
				args: map[string]Argument{"FROM": Argument{Key: "FROM", Value: "<bob@example.org>", Operator: ":"}},
			},
			{
				line: "HELO some_ctrl_char\r\n",
				verb: "HELO",
				args: map[string]Argument{"SOME_CTRL_CHAR": Argument{Key: "some_ctrl_char"}},
			},
			{
				line: "HELO some_ctrl_char\n",
				verb: "HELO",
				args: map[string]Argument{"SOME_CTRL_CHAR": Argument{Key: "some_ctrl_char"}},
			},
			{
				line: "AUTH PLAIN\r\n",
				verb: "AUTH",
				args: map[string]Argument{"PLAIN": {Key: "PLAIN"}},
			},
			{
				line: "AUTH PLAIN dGVzdAB0ZXN0ADEyMzQ=\r\n",
				verb: "AUTH",
				args: map[string]Argument{"PLAIN": {Key: "PLAIN"}, "DGVZDAB0ZXN0ADEYMZQ": {Key: "dGVzdAB0ZXN0ADEyMzQ", Operator: "="}},
			},
			{
				line: "SOME_verb     a	b    c test1=value1 test2:value2\n",
				verb: "SOME_VERB",
				args: map[string]Argument{
					"A\tB":  Argument{Key: "a\tb"},
					"C":     Argument{Key: "c"},
					"TEST1": Argument{Key: "test1", Value: "value1", Operator: "="},
					"TEST2": Argument{Key: "test2", Value: "value2", Operator: ":"},
				},
			},
		}

		for _, test := range tests {
			br := bufio.NewReader(strings.NewReader(test.line))
			verb, args, err := parseLine(br)
			So(err, ShouldEqual, nil)
			So(verb, ShouldEqual, test.verb)
			So(args, ShouldResemble, test.args)
		}

	})

	Convey("Testing parseTo()", t, func() {

		tests := []struct {
			line          string
			addressString string
		}{
			{
				line:          "RCPT TO:<alice@example.com>\r\n",
				addressString: "alice@example.com",
			},
		}

		for _, test := range tests {
			br := bufio.NewReader(strings.NewReader(test.line))
			_, args, err := parseLine(br)
			So(err, ShouldEqual, nil)

			toArg := args["TO"]
			addr, err := parseTO(toArg.Key + toArg.Operator + toArg.Value)
			So(err, ShouldEqual, nil)
			So(addr.GetAddress(), ShouldEqual, test.addressString)
		}

	})

	Convey("Testing parseFROM()", t, func() {

		tests := []struct {
			line          string
			addressString string
		}{
			{
				line:          "MAIL from:<alice@example.com>\r\n",
				addressString: "alice@example.com",
			},
		}

		for _, test := range tests {
			br := bufio.NewReader(strings.NewReader(test.line))
			_, args, err := parseLine(br)
			So(err, ShouldEqual, nil)

			fromArg := args["FROM"]
			addr, err := parseFROM(fromArg.Key + fromArg.Operator + fromArg.Value)
			So(err, ShouldEqual, nil)
			So(addr.GetAddress(), ShouldEqual, test.addressString)
		}

	})

	Convey("Testing parseAuthPlainInitialRespone()", t, func() {

		tests := []struct {
			initialResponse       string
			authenticationIdenity string
			authorizationIdentity string
			password              string
		}{
			{
				initialResponse:       "dGVzdAB0ZXN0ADEyMzQ=",
				authenticationIdenity: "test",
				authorizationIdentity: "test",
				password:              "1234",
			},
			{
				initialResponse:       "dGVzdAB0ZXN0AHRlc3RwYXNz",
				authenticationIdenity: "test",
				authorizationIdentity: "test",
				password:              "testpass",
			},
			{
				initialResponse:       "YXV0aHoAYXV0aG4AcGFzcw==",
				authenticationIdenity: "authn",
				authorizationIdentity: "authz",
				password:              "pass",
			},
			{
				initialResponse:       "AGF1dGhuAHBhc3M=",
				authenticationIdenity: "authn",
				authorizationIdentity: "", // empty authorization identity
				password:              "pass",
			},
		}

		for _, test := range tests {
			authorizationIdentity, authenticationIdenity, password, err := ParseAuthPlainInitialRespone(test.initialResponse)
			So(err, ShouldEqual, nil)
			So(authenticationIdenity, ShouldEqual, test.authenticationIdenity)
			So(authorizationIdentity, ShouldEqual, test.authorizationIdentity)
			So(password, ShouldEqual, test.password)
		}

		_, _, _, err := ParseAuthPlainInitialRespone("test")
		So(err, ShouldBeError)

		_, _, _, err = ParseAuthPlainInitialRespone("YXV0aHoAYXV0aG4=") // "authz\0authn"
		So(err, ShouldBeError)
	})
}
