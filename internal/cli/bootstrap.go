package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var bootstrapHost = regexp.MustCompile(`^(?:[a-z_][a-z0-9_-]*@)?[a-zA-Z0-9][a-zA-Z0-9.-]{0,252}$`)
var bootstrapDomain = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,252}$`)
var bootstrapVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9.-]+)?$`)
var bootstrapImage = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,250}$`)
var bootstrapArchive = regexp.MustCompile(`^/[a-zA-Z0-9/_-]+[a-zA-Z0-9._-]+$`)

// Bootstrap invokes the ordinary installer over one explicitly pinned SSH
// connection. It does not retain access or introduce remote worker targets.
func Bootstrap(args []string, out, stderr io.Writer) error {
	flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	flags.SetOutput(stderr)
	host := flags.String("host", "", "Authorized [user@]host")
	port := flags.Int("port", 22, "SSH port")
	known := flags.String("known-hosts", "", "Known hosts file with the verified host key")
	identity := flags.String("identity", "", "SSH identity file; otherwise use ssh-agent or OpenSSH defaults")
	script := flags.String("installer", "", "Reviewed local install.sh")
	checksum := flags.String("sha256", "", "Installer SHA-256")
	domain := flags.String("domain", "", "Dashboard hostname")
	version := flags.String("version", "", "Pinned release version")
	image := flags.String("image", "", "Image name for a verified staging archive")
	archive := flags.String("remote-archive", "", "Verified image archive already on the authorized host")
	archiveSHA := flags.String("archive-sha256", "", "Remote archive SHA-256")
	apply := flags.Bool("apply", false, "Apply the reviewed installer effects")
	if e := flags.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if flags.NArg() != 0 || !bootstrapHost.MatchString(*host) || !bootstrapDomain.MatchString(*domain) ||
		*port < 1 || *port > 65535 || *known == "" || *script == "" {
		return errors.New("Provide one host, verified known-hosts file, installer and dashboard hostname")
	}
	if _, e := os.Stat(*known); e != nil {
		return errors.New("The verified known-hosts file is unavailable")
	}
	knownFile, e := filepath.Abs(*known)
	if e != nil || strings.ContainsAny(knownFile, "\"\r\n") {
		return errors.New("Use a conventional known-hosts file path")
	}
	artifact := []string{}
	if *archive != "" {
		if *version != "" || !bootstrapArchive.MatchString(*archive) || strings.Contains(*archive, "..") ||
			!bootstrapImage.MatchString(*image) || !sha256Hex(*archiveSHA) {
			return errors.New("Provide a remote archive, image name and archive SHA-256")
		}
		artifact = append(artifact, "--image", *image, "--archive", *archive, "--sha256", *archiveSHA)
	} else {
		if !bootstrapVersion.MatchString(*version) || *image != "" || *archiveSHA != "" {
			return errors.New("Provide a pinned release version")
		}
		artifact = append(artifact, "--version", *version)
	}
	file, e := os.Open(*script)
	if e != nil {
		return e
	}
	body, e := io.ReadAll(io.LimitReader(file, 128<<10))
	file.Close()
	if e != nil {
		return e
	}
	sum := sha256.Sum256(body)
	if !sha256Hex(*checksum) || !strings.EqualFold(*checksum, hex.EncodeToString(sum[:])) {
		return errors.New("Installer checksum did not match")
	}
	sshArgs := []string{"-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes",
		"-o", "UserKnownHostsFile=\"" + filepath.ToSlash(knownFile) + "\"", "-o", "GlobalKnownHostsFile=none", "-o", "ClearAllForwardings=yes", "-o", "ForwardAgent=no",
		"-o", "ConnectTimeout=10", "-p", strconv.Itoa(*port)}
	if *identity != "" {
		sshArgs = append(sshArgs, "-i", *identity)
	}
	command := "sudo -n bash -s --"
	if strings.HasPrefix(*host, "root@") {
		command = "bash -s --"
	}
	// Every appended argument has been checked against a narrow shell-safe form.
	command += " --domain " + *domain + " " + strings.Join(artifact, " ")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	run := func(mode string) error {
		options := append(append([]string(nil), sshArgs...), *host, command+" "+mode)
		process := exec.CommandContext(ctx, "ssh", options...)
		process.Stdin, process.Stdout, process.Stderr = bytes.NewReader(body), out, stderr
		if e := process.Run(); e != nil {
			return fmt.Errorf("SSH installer %s: %w", mode, e)
		}
		return nil
	}
	if e = run("--check"); e != nil {
		return e
	}
	if *apply {
		return run("--apply")
	}
	return nil
}

func sha256Hex(value string) bool {
	decoded, e := hex.DecodeString(value)
	return e == nil && len(decoded) == sha256.Size
}
