package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/mail"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
	"shipper/internal/auth"
	"shipper/internal/store"
)

func claimOwner(db *store.Store, dir string, args []string) error {
	flags := flag.NewFlagSet("claim", flag.ContinueOnError)
	email := flags.String("email", "", "First administrator email")
	stdin := flags.Bool("password-stdin", false, "Read the password from standard input")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: shipper claim [--email ADDRESS --password-stdin]")
	}
	ctx := context.Background()
	configured, err := db.Configured(ctx)
	if err != nil {
		return err
	}
	if configured {
		return store.ErrAlreadyConfigured
	}
	if *email == "" && !*stdin && term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Print("Administrator email: ")
		var value string
		if _, err = fmt.Fscanln(os.Stdin, &value); err != nil {
			return err
		}
		*email = value
	}
	*email = strings.ToLower(strings.TrimSpace(*email))
	address, err := mail.ParseAddress(*email)
	if err != nil || address.Address != *email || len(*email) > 254 {
		return errors.New("Enter a valid administrator email")
	}
	var password []byte
	if *stdin {
		password, err = io.ReadAll(io.LimitReader(os.Stdin, 75))
		password = []byte(strings.TrimSuffix(strings.TrimSuffix(string(password), "\n"), "\r"))
	} else {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("Use a terminal or --password-stdin")
		}
		fmt.Print("New password: ")
		password, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
	}
	if err != nil {
		return err
	}
	hash, err := auth.Password(string(password))
	if err != nil {
		return err
	}
	if err = db.CreateAdmin(ctx, store.User{ID: auth.Token()[:24], Email: *email, PasswordHash: hash}, "", "", 0); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(dir, "setup-token"))
	fmt.Println("Administrator created. Automatic HTTPS will use the administrator email.")
	return nil
}

func resetFactor(db *store.Store) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("factor recovery requires an interactive terminal")
	}
	input := bufio.NewReader(os.Stdin)
	fmt.Print("Administrator email: ")
	email, err := input.ReadString('\n')
	if err != nil {
		return err
	}
	fmt.Print("Type Reset two-factor authentication: ")
	value, err := input.ReadString('\n')
	if err != nil || strings.TrimSpace(value) != "Reset two-factor authentication" {
		return errors.New("factor recovery cancelled")
	}
	if err = db.RecoverFactor(context.Background(), strings.TrimSpace(email)); err != nil {
		return err
	}
	fmt.Println("Factor and recovery codes removed. Sessions and operations tokens were revoked.")
	return nil
}
