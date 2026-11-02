package store

import (
	"context"
	"fmt"
	"testing"
)

func TestDeploymentLineNumbersAreLocalAndCursorsRemainGlobal(t *testing.T) {
	db, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	for _, id := range []string{"first", "second"} {
		_, e = db.Enqueue(ctx, Application{ID: id, Name: id, Config: Config{Domain: id + ".localhost"}}, Deployment{ID: id, Operation: "deploy"}, "", true)
		if e != nil {
			t.Fatal(e)
		}
	}
	for index := 0; index < 503; index++ {
		if e = db.AppendLog(ctx, "first", "building", fmt.Sprint(index)); e != nil {
			t.Fatal(e)
		}
		if e = db.AppendLog(ctx, "second", "building", fmt.Sprint(index)); e != nil {
			t.Fatal(e)
		}
	}
	lines, e := db.Logs(ctx, "second", 0)
	if e != nil {
		t.Fatal(e)
	}
	if len(lines) != 500 || lines[0].Number != 1 || lines[0].Sequence != 2 || lines[499].Number != 500 {
		t.Fatal("local numbering/global cursor mismatch", lines[0], lines[len(lines)-1])
	}
	next, e := db.Logs(ctx, "second", lines[499].Sequence)
	if e != nil || len(next) != 3 || next[0].Number != 501 || next[2].Number != 503 {
		t.Fatal("paging lost local numbering", next, e)
	}
}
