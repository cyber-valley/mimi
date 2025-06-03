package tgscraper

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// Terminal implements auth.UserAuthenticator prompting the terminal for
// input.
//
// This is only example implementation, you should not use it in your code.
// Copy it and modify to fit your needs.
type terminalUserAuthenticator struct {
	PhoneNumber string // optional, will be prompted if empty
}

func (terminalUserAuthenticator) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("signing up not implemented in Terminal")
}

func (terminalUserAuthenticator) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	return &auth.SignUpRequired{TermsOfService: tos}
}

func (terminalUserAuthenticator) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	fmt.Print("Enter code: ")
	code, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(code), nil
}

func (a terminalUserAuthenticator) Phone(_ context.Context) (string, error) {
	if a.PhoneNumber != "" {
		return a.PhoneNumber, nil
	}
	fmt.Print("Enter phone in international format (e.g. +1234567890): ")
	phone, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(phone), nil
}

func (terminalUserAuthenticator) Password(_ context.Context) (string, error) {
	fmt.Print("Enter 2FA password: ")
	bytePwd, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytePwd)), nil
}
