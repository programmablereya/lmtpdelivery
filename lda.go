package main

import (
	"flag"
	"fmt"
	"github.com/emersion/go-smtp"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
)

func sendMail(from string, to string, socket string) (err error) {
	conn, err := net.Dial("unix", socket)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed opening the socket <%s>: %s", socket, err)
		return err
	}
	client, err := smtp.NewClientLMTP(conn, "localhost")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed establishing the connection: %s", err)
		return err
	}
	defer func() {
		quitErr := client.Quit()
		if quitErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed closing the connection: %s", quitErr)
		}
	}()
	err = client.Mail(from, &smtp.MailOptions{})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed establishing the envelope FROM <%s>: %s", from, err)
		return err
	}
	err = client.Rcpt(to)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed establishing the envelope FROM <%s> TO <%s>: %s", from, to, err)
		return err
	}
	var sendError *smtp.SMTPError
	writer, err := client.LMTPData(func(rcpt string, status *smtp.SMTPError) {
		if status != nil {
			sendError = status
			_, _ = fmt.Fprintf(os.Stderr, "Failed sending the message FROM <%s> TO <%s>: %s", from, rcpt, status)
		}
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed opening the writing stream FROM <%s> TO <%s>: %s", from, to, err)
		return err
	}
	_, err = io.Copy(writer, os.Stdin)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed copying the message FROM <%s> TO <%s> data: %s", from, to, err)
		return err
	}
	err = writer.Close()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed closing the writing stream FROM <%s> TO <%s>: %s", from, to, err)
		return err
	}
	if sendError != nil {
		return sendError
	}
	return nil
}

var UnwrapRe = regexp.MustCompile("^.*\\s*<([^<>]+)>\\s*$")

func unwrap(from string) string {
	result := UnwrapRe.FindStringSubmatch(from)
	if result == nil {
		return from
	}
	return result[1]
}

func escapeAts(from string) string {
	count := strings.Count(from, "@")
	if count <= 1 {
		return from
	}
	return strings.Replace(from, "@", ".at.", count - 1)
}

func main() {
	from := flag.String("from", "", "The original envelope sender of the message.")
	to := flag.String("to", "", "The envelope recipient of the message.")
	socket := flag.String("socket", "/var/run/dovecot/lmtp", "The LMTP Unix socket to send on.")
	escapeDoubledAts := flag.Bool("escapeDoubledAts", true, "True to escape multiple @ symbols.")
	unwrapFrom := flag.Bool("unwrapBrackets", true, "True to unwrap unnecessary <> symbols.")
	replaceUnknown := flag.Bool("replaceUnknown", true, "True to replace 'unknown' with a default value")
	flag.Parse()

	if *unwrapFrom {
		*from = unwrap(*from)
	}
	if *escapeDoubledAts {
		*from = escapeAts(*from)
	}
	if *replaceUnknown && *from == "unknown" {
		*from = "unknown@unknown.invalid"
	}
	if err := sendMail(*from, *to, *socket); err != nil {
		os.Exit(1)
	}
}
