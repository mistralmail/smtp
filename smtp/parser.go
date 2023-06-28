package smtp

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

type parser struct {
}

func (p *parser) ParseCommand(br *bufio.Reader) (command Cmd, err error) {
	/*
		RFC 5321 2.3.8

		Lines consist of zero or more data characters terminated by the
		sequence ASCII character "CR" (hex value 0D) followed immediately by
		ASCII character "LF" (hex value 0A).  This termination sequence is
		denoted as <CRLF> in this document.  Conforming implementations MUST
		NOT recognize or generate any other character or character sequence
		as a line terminator.  Limits MAY be imposed on line lengths by
		servers (see Section 4).
	*/

	var address *MailAddress
	verb, args, err := parseLine(br)
	if err != nil {
		return nil, err
	}
	//conn.write(500, err.Error())
	//conn.c.Close()

	switch verb {

	case "HELO":
		{
			if len(args) != 1 {
				command = InvalidCmd{Cmd: "HELO", Info: "HELO requires exactly one valid domain"}
				break
			}
			domain := ""
			for _, arg := range args {
				domain = arg.Key
			}
			command = HeloCmd{Domain: domain}
		}

	case "EHLO":
		{
			if len(args) != 1 {
				command = InvalidCmd{Cmd: "EHLO", Info: "EHLO requires exactly one valid address"}
				break
			}
			domain := ""
			for _, arg := range args {
				domain = arg.Key
			}
			command = EhloCmd{Domain: domain}
		}

	case "MAIL":
		{
			fromArg := args["FROM"]
			address, err = parseFROM(fromArg.Key + fromArg.Operator + fromArg.Value)
			if err != nil {
				command = InvalidCmd{Cmd: verb, Info: err.Error()}
				err = nil
				break
			}

			eightBitMIME := false
			bodyArg, ok := args["BODY"]
			if ok {
				bodyArg.Value = strings.ToUpper(bodyArg.Value)
				if bodyArg.Operator != "=" || (bodyArg.Value != "8BITMIME" && bodyArg.Value != "7BIT") {
					command = InvalidCmd{Cmd: verb, Info: "Syntax is BODY=8BITMIME|7BIT"}
					break
				}

				if bodyArg.Value == "8BITMIME" {
					eightBitMIME = true
				}
			}

			command = MailCmd{From: address, EightBitMIME: eightBitMIME}
		}

	case "RCPT":
		{
			toArg := args["TO"]
			address, err = parseTO(toArg.Key + toArg.Operator + toArg.Value)
			if err != nil {
				command = InvalidCmd{Cmd: verb, Info: err.Error()}
				err = nil
			} else {
				command = RcptCmd{To: address}
			}
		}

	case "DATA":
		{
			// TODO: write tests for this
			command = DataCmd{
				R: *NewDataReader(br),
			}
		}

	case "RSET":
		{
			command = RsetCmd{}
		}

	case "SEND":
		{
			command = SendCmd{}
		}

	case "SOML":
		{
			command = SomlCmd{}
		}

	case "SAML":
		{
			command = SamlCmd{}
		}

	case "VRFY":
		{
			//conn.write(502, "Command not implemented")
			/*
					RFC 821
					SMTP provides as additional features, commands to verify a user
					name or expand a mailing list.  This is done with the VRFY and
					EXPN commands
					RFC 5321
					As discussed in Section 3.5, individual sites may want to disable
					either or both of VRFY or EXPN for security reasons (see below).  As
					a corollary to the above, implementations that permit this MUST NOT
					appear to have verified addresses that are not, in fact, verified.
					If a site disables these commands for security reasons, the SMTP
					server MUST return a 252 response, rather than a code that could be
					confused with successful or unsuccessful verification.
					Returning a 250 reply code with the address listed in the VRFY
					command after having checked it only for syntax violates this rule.
					Of course, an implementation that "supports" VRFY by always returning
					550 whether or not the address is valid is equally not in
					conformance.
				From what I have read, 502 is better than 252...
			*/
			user := ""
			for _, arg := range args {
				user = arg.Key
			}
			command = VrfyCmd{Param: user}
		}

	case "EXPN":
		{
			listName := ""
			for _, arg := range args {
				listName = arg.Key
			}
			command = ExpnCmd{ListName: listName}
		}

	case "NOOP":
		{
			command = NoopCmd{}
		}

	case "QUIT":
		{
			command = QuitCmd{}
		}

	case "STARTTLS":
		{
			command = StartTlsCmd{}
		}
	case "AUTH":
		{
			mechanism := ""
			initialResponse := ""
			// TODO: make this better
			count := 0
			for _, arg := range args {
				if count == 0 {
					mechanism = arg.Key
				}
				if count == 1 {
					initialResponse = arg.Key + arg.Operator + arg.Value
				}
				count++

			}
			command = AuthCmd{
				Mechanism:       mechanism,
				InitialResponse: initialResponse,
				R:               *br,
			}
		}

	default:
		{
			// TODO: CLEAN THIS UP
			command = UnknownCmd{Cmd: verb, Line: strings.TrimSuffix(verb, "\n")}
		}

	}

	return
}

type Argument struct {
	Key      string
	Value    string
	Operator string
}

// parseLine returns the verb of the line and a list of all comma separated arguments
func parseLine(br *bufio.Reader) (string, map[string]Argument, error) {
	/*
		RFC 5321
		4.5.3.1.4.  Command Line

		The maximum total length of a command line including the command word
		and the <CRLF> is 512 octets.  SMTP extensions may be used to
		increase this limit.
	*/
	buffer, err := ReadUntill('\n', MAX_CMD_LINE, br)
	if err != nil {
		if err == ErrLtl {
			SkipTillNewline(br)
			return string(buffer), map[string]Argument{}, err
		}

		return string(buffer), map[string]Argument{}, err
	}
	line := string(buffer)
	verb := ""
	argMap := map[string]Argument{}

	// Strip \n and \r
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")

	i := strings.Index(line, " ")
	if i == -1 {
		verb = strings.ToUpper(line)
		return verb, map[string]Argument{}, nil
	}

	verb = strings.ToUpper(line[:i])
	line = line[i+1:]

	tmpArgs := strings.Split(line, " ")
	for _, arg := range tmpArgs {
		argument := Argument{}
		i = strings.IndexAny(arg, ":=")
		if i == -1 {
			argument.Key = strings.TrimSpace(arg)
		} else {
			argument.Key = strings.TrimSpace(arg[:i])
			argument.Value = strings.TrimSpace(arg[i+1:])
			argument.Operator = arg[i : i+1]
		}

		if len(argument.Key) == 0 {
			continue
		}

		// Put key in the arguments map in uppercase to make sure
		// we are case insensitive
		argMap[strings.ToUpper(argument.Key)] = argument
	}

	return verb, argMap, nil
}

func parseFROM(from string) (*MailAddress, error) {
	index := strings.Index(from, ":")
	if index == -1 {
		return nil, errors.New("No FROM given (didn't find ':')")
	}
	if strings.ToLower(from[0:index]) != "from" {
		return nil, errors.New("No FROM given")
	}

	address_str := from[index+1:]

	address, err := ParseAddress(address_str)
	if err != nil {
		return nil, err
	}
	return &address, nil
}

func parseTO(to string) (*MailAddress, error) {
	index := strings.Index(to, ":")
	if index == -1 {
		return nil, errors.New("No TO given (didn't find ':')")
	}
	if strings.ToLower(to[0:index]) != "to" {
		return nil, errors.New("No TO given")
	}

	address_str := to[index+1:]

	address, err := ParseAddress(address_str)
	if err != nil {
		return nil, err
	}
	return &address, nil
}

// ParseAuthPlainInitialRespone parses the base64 encoded initial response of an Auth PLAIN request
//
// "The mechanism consists of a single message from the client to the server. The
// client sends the authorization identity (identity to login as), followed by a
// US-ASCII NulL character, followed by the authentication identity (identity whose
// password will be used), followed by a US-ASCII NulL character, followed by the
// clear-text password. The client may leave the authorization identity empty to indicate
// that it is the same as the authentication identity."
func ParseAuthPlainInitialRespone(initialResponse string) (authorizationIdentity string, authenticationIdenity string, password string, err error) {
	initialResponseByte, err := base64.StdEncoding.DecodeString(initialResponse)
	if err != nil {
		err = fmt.Errorf("couldn't decode base64 %v", err)
		return
	}
	initialResponseByteSplit := bytes.Split(initialResponseByte, []byte("\x00"))
	if len(initialResponseByteSplit) != 3 {
		err = fmt.Errorf("couldn't parse initial response: expected exactly 3 arguments")
		return
	}

	authorizationIdentity = string(initialResponseByteSplit[0])
	authenticationIdenity = string(initialResponseByteSplit[1])
	password = string(initialResponseByteSplit[2])
	return
}
