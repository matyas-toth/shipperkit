package store

import "testing"

func TestDatabaseTransitionPreservesInitialization(t *testing.T) {
	old := Config{Database: &DatabaseResource{Engine: "postgresql", Version: "18"}, Variables: []Variable{{Name: "POSTGRES_PASSWORD", Value: "original", Secret: true}, {Name: "POSTGRES_DB", Value: "app"}}}
	same := old
	if e := ValidateDatabaseTransition(old, same); e != nil {
		t.Fatal(e)
	}
	changed := old
	changed.Database = &DatabaseResource{Engine: "postgresql", Version: "17"}
	if e := ValidateDatabaseTransition(old, changed); e == nil {
		t.Fatal("incompatible data format allowed")
	}
	changed = old
	changed.Database = nil
	if e := ValidateDatabaseTransition(old, changed); e == nil {
		t.Fatal("database identity removed")
	}
	changed = old
	changed.Variables = []Variable{{Name: "POSTGRES_PASSWORD", Value: "new"}, {Name: "POSTGRES_DB", Value: "app"}}
	if e := ValidateDatabaseTransition(old, changed); e == nil {
		t.Fatal("initialization password edit allowed")
	}
	changed = old
	changed.Database = &DatabaseResource{Engine: "postgresql", Version: "18", TLSHostname: "db.example.test"}
	if e := ValidateDatabaseTransition(old, changed); e != nil {
		t.Fatal("safe TLS hostname change rejected", e)
	}
}
