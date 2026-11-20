package deploy

import (
	"strings"
	"testing"
)

func TestPlainSQLDumpsArePreparedForAScopedImport(t *testing.T) {
	dump := strings.Join([]string{
		`--`,
		`-- PostgreSQL database dump`,
		`--`,
		`\restrict abc123`,
		`SET statement_timeout = 0;`,
		`SELECT pg_catalog.set_config('search_path', '', false);`,
		`CREATE FUNCTION public.touch() RETURNS trigger LANGUAGE plpgsql AS $$`,
		`BEGIN`,
		`GRANT SELECT ON t TO nobody;`,
		`RETURN NEW;`,
		`END $$;`,
		`ALTER FUNCTION public.touch() OWNER TO someone;`,
		`CREATE TABLE public.items (id integer, note text);`,
		`ALTER TABLE public.items OWNER TO someone;`,
		`COPY public.items (id, note) FROM stdin;`,
		`1	GRANT ALL ON everything TO everyone;`,
		`2	\N`,
		`\.`,
		`GRANT ALL ON TABLE public.items TO reader;`,
		`ALTER DEFAULT PRIVILEGES FOR ROLE someone IN SCHEMA public GRANT ALL ON TABLES TO reader;`,
		`\unrestrict abc123`,
		``,
	}, "\n")
	var out strings.Builder
	if err := filterPlainSQL(strings.NewReader(dump), &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, dropped := range []string{`\restrict`, `\unrestrict`, "OWNER TO", "GRANT ALL ON TABLE", "DEFAULT PRIVILEGES"} {
		if strings.Contains(got, dropped) {
			t.Fatalf("kept %q:\n%s", dropped, got)
		}
	}
	// Function bodies and COPY data are data, not statements to filter.
	for _, kept := range []string{"GRANT SELECT ON t TO nobody;", "1\tGRANT ALL ON everything TO everyone;", "2\t\\N", "\\.\n", "CREATE TABLE public.items"} {
		if !strings.Contains(got, kept) {
			t.Fatalf("dropped %q:\n%s", kept, got)
		}
	}
	for _, bad := range []string{"\\! id\nSELECT 1;\n", "\\connect other\nSELECT 1;\n", "CREATE DATABASE other;\n", "-- only a comment\n", "PGDMP\x00binary"} {
		if err := filterPlainSQL(strings.NewReader(bad), &strings.Builder{}); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestRecoveryNamesStayValid(t *testing.T) {
	for _, tc := range []struct{ database, suffix string }{
		{"app", "_before_20261120_101500"},
		{"my-shop", "_restore_abcdef12"},
		{"1st", "_restore_abcdef12"},
		{strings.Repeat("x", 63), "_before_20261120_101500"},
	} {
		if name := recoveryName(tc.database, tc.suffix); !ValidRecoveryDatabase(name) || !strings.HasSuffix(name, tc.suffix) {
			t.Fatalf("%q produced %q", tc.database, name)
		}
	}
}
