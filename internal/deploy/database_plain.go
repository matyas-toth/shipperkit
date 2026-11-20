package deploy

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"regexp"
	"strings"
)

var (
	plainCopyStart = regexp.MustCompile(`(?i)^COPY\s.+\sFROM\s+stdin;\s*$`)
	// Ownership and privileges name roles of the source server. The import runs
	// as a temporary role that owns everything it creates, so these are dropped.
	plainOwnership = regexp.MustCompile(`(?i)^(ALTER\s.+\sOWNER\s+TO\s.+|ALTER\s+DEFAULT\s+PRIVILEGES\s.+|GRANT\s.+|REVOKE\s.+);\s*$`)
	plainDatabase  = regexp.MustCompile(`(?i)^(CREATE|DROP|ALTER)\s+DATABASE\s`)
	dollarQuote    = regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*\$|\$\$`)
)

// filterPlainSQL prepares a plain pg_dump script for a scoped import. It drops
// ownership, privilege and pg_dump's own restrict lines, keeps COPY data and
// function bodies intact, and rejects psql client commands and database-level
// statements. The import additionally runs in psql's restricted mode, so any
// client command this line check cannot see still fails.
func filterPlainSQL(src io.Reader, dst io.Writer) error {
	reader := bufio.NewReaderSize(src, 64<<10)
	writer := bufio.NewWriterSize(dst, 64<<10)
	inCopy, tag, statements := false, "", 0
	for {
		line, e := reader.ReadString('\n')
		if e != nil && e != io.EOF {
			return e
		}
		if line == "" && e == io.EOF {
			break
		}
		if strings.ContainsRune(line, 0) {
			return errors.New("Upload a plain SQL dump from pg_dump or a custom-format dump created with pg_dump -Fc")
		}
		text := strings.TrimSpace(line)
		keep := true
		switch {
		case inCopy:
			if strings.TrimRight(line, "\r\n") == `\.` {
				inCopy = false
			}
		case tag != "":
			// Function bodies pass through untouched.
		case strings.HasPrefix(text, `\restrict `) || strings.HasPrefix(text, `\unrestrict `):
			keep = false
		case strings.HasPrefix(text, `\`):
			command, _, _ := strings.Cut(text, " ")
			return errors.New("Export one database without psql commands such as " + command)
		case plainDatabase.MatchString(text):
			return errors.New("Export one database without database creation or removal")
		case plainOwnership.MatchString(text):
			keep = false
		case plainCopyStart.MatchString(text):
			inCopy = true
			statements++
		default:
			if strings.HasSuffix(text, ";") {
				statements++
			}
		}
		if !inCopy && keep {
			for _, quote := range dollarQuote.FindAllString(stripLineComment(line), -1) {
				if tag == "" {
					tag = quote
				} else if quote == tag {
					tag = ""
				}
			}
		}
		if keep {
			if _, err := writer.WriteString(line); err != nil {
				return err
			}
		}
		if e == io.EOF {
			break
		}
	}
	if statements == 0 {
		return errors.New("The dump contains no SQL statements")
	}
	return writer.Flush()
}

func stripLineComment(line string) string {
	if strings.HasPrefix(strings.TrimSpace(line), "--") {
		return ""
	}
	return line
}

// plainSQL reports whether a dump is a plain SQL script rather than a
// custom-format archive.
func plainSQL(head []byte) bool { return !bytes.HasPrefix(head, []byte("PGDMP")) }
