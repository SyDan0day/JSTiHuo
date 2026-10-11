package main

import (
	"context"
	"flag"
	"fmt"
	"golang.org/x/term"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/bootstrap"
	"os"
	"strings"
	"time"
)

func main() {
	name := flag.String("login-name", "", "initial administrator login name")
	flag.Parse()
	if *name == "" || !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "Provide --login-name and an interactive terminal")
		os.Exit(1)
	}
	fmt.Fprint(os.Stderr, "Password (12..128 characters): ")
	password, e := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if e != nil {
		os.Exit(1)
	}
	fmt.Fprint(os.Stderr, "Confirm password: ")
	confirmation, e := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if e != nil || string(password) != string(confirmation) {
		fmt.Fprintln(os.Stderr, "Passwords differ")
		os.Exit(1)
	}
	b, e := bootstrap.ReadSecret("DB_DSN_FILE")
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	db, e := bootstrap.OpenDB(strings.TrimSpace(string(b)))
	if e != nil {
		fmt.Fprintln(os.Stderr, "Invalid database configuration")
		os.Exit(1)
	}
	defer db.Close()
	svc, e := identity.New(db, nil, nil, nil)
	if e != nil {
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if e = svc.CreateInitialAdmin(ctx, *name, string(password)); e != nil {
		fmt.Fprintln(os.Stderr, "Administrator initialization failed (store may already be initialized)")
		os.Exit(1)
	}
	fmt.Println("Administrator created; password change required at first login")
}
