//go:build linux

// shipper-start runs the configured startup command before replacing itself with
// the image's original entrypoint. It needs no shell unless the command uses one.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type command struct {
	Shell     string
	Argv      []string
	Directory string
	User      string
	Timeout   int
}

func run(c command) error {
	args := c.Argv
	if c.Shell != "" {
		args = []string{"/bin/sh", "-c", c.Shell}
	}
	if len(args) == 0 {
		return nil
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 300
	}
	parent, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = c.Directory
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if c.User != "" {
		uidText, gidText, _ := strings.Cut(c.User, ":")
		uid, e := strconv.Atoi(uidText)
		if e != nil {
			u, err := user.Lookup(uidText)
			if err != nil {
				return err
			}
			uidText = u.Uid
			uid, e = strconv.Atoi(uidText)
			if gidText == "" {
				gidText = u.Gid
			}
		}
		if e != nil {
			return e
		}
		gid := uid
		if gidText != "" {
			gid, e = strconv.Atoi(gidText)
			if e != nil {
				return e
			}
		}
		if uint32(uid) != uint32(os.Geteuid()) || uint32(gid) != uint32(os.Getegid()) {
			cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		}
	}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = time.Second
	fmt.Fprintln(os.Stdout, time.Now().UTC().Format(time.RFC3339Nano), "[Shipper] Before start")
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("before start failed: %w", e)
	}
	return nil
}
func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("shipper-start")
		return
	}
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Invalid Shipper startup configuration")
		os.Exit(127)
	}
	raw, e := base64.RawStdEncoding.DecodeString(os.Args[1])
	var c command
	if e == nil {
		e = json.Unmarshal(raw, &c)
	}
	if e == nil {
		e = run(c)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	path, e := exec.LookPath(os.Args[2])
	if e == nil {
		e = syscall.Exec(path, os.Args[2:], os.Environ())
	}
	fmt.Fprintln(os.Stderr, e)
	os.Exit(127)
}
