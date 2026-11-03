package deploy

import (
	"archive/tar"
	"bytes"
	"context"
	"github.com/moby/moby/client"
	"io"
	"net"
	"os"
	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/startup"
	"shipper/internal/store"
	"strings"
	"testing"
	"time"
)

func TestDockerLifecycleTasks(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_TASKS") != "1" {
		t.Skip("set SHIPPER_TEST_TASKS=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	docker, e := engine.New()
	if e != nil {
		t.Fatal(e)
	}
	defer docker.Close()
	instance := auth.Token()
	dir := t.TempDir()
	os.MkdirAll(dir+"/work", 0700)
	free := func() int {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		p := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return p
	}
	r := &DockerRuntime{Client: docker.Client(), Instance: instance, DataDir: dir, Config: ProxyConfig{BindIP: "127.0.0.1", HTTPPort: free(), HTTPSPort: free()}}
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { db.Close() }()
	w := NewWorker(db, r, events.New(), dir)
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		items, _ := r.Client.ContainerList(clean, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, c := range items.Items {
			r.removeOwned(clean, c.ID, "")
		}
		networks, _ := r.Client.NetworkList(clean, client.NetworkListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, n := range networks.Items {
			r.Client.NetworkRemove(clean, n.ID, client.NetworkRemoveOptions{})
		}
		volumes, _ := r.Client.VolumeList(clean, client.VolumeListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, v := range volumes.Items {
			r.Client.VolumeRemove(clean, v.Name, client.VolumeRemoveOptions{})
		}
		images, _ := r.Client.ImageList(clean, client.ImageListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, i := range images.Items {
			r.Client.ImageRemove(clean, i.ID, client.ImageRemoveOptions{})
		}
	})
	yaml := `services:
  web:
    expose: ['80']
    image: ` + ApacheImage + `
    volumes: ['state:/state']
    depends_on:
      migration: {condition: service_completed_successfully}
  migration:
    image: postgres:17-alpine
    entrypoint: ['sh','-c','true']
    environment:
      PGPASSWORD: ${DATABASE_PASSWORD:?required}
    volumes: ['state:/state']
    depends_on:
      db: {condition: service_healthy}
  db:
    image: postgres:17-alpine
    environment:
      POSTGRES_PASSWORD: ${DATABASE_PASSWORD:?required}
    volumes: ['database:/var/lib/postgresql/data']
    healthcheck:
      test: ['CMD-SHELL','pg_isready -U postgres']
      interval: 1s
      retries: 30
volumes:
  database:
  state:
`
	a := store.Application{ID: auth.Token()[:24], Name: "Tasks fixture", Config: store.Config{Method: "compose", ComposeSource: "paste", ComposeYAML: yaml, ComposeFile: "compose.yaml", RootDirectory: ".", Variables: []store.Variable{{Name: "DATABASE_PASSWORD", Value: "task-password-" + auth.Token()[:12], Secret: true, Scope: "runtime"}}, Hooks: []store.Hook{{Stage: "before_deploy", Command: store.Command{Service: "migration", Shell: "psql -h db -U postgres -v ON_ERROR_STOP=1 -c 'CREATE TABLE IF NOT EXISTS task_schema(value text);' && touch /state/schema", Mode: "job", Timeout: 90}}, {Stage: "before_start", Command: store.Command{Service: "web", Shell: "test -f /state/schema && echo start >> /state/starts", Mode: "job", Timeout: 30}}}}}
	run := func(op string, create bool, want string) store.Deployment {
		t.Helper()
		d, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: op}, "", create)
		if e != nil {
			t.Fatal(e)
		}
		w.process(ctx, d)
		got, e := db.Deployment(ctx, d.ID)
		if e != nil {
			t.Fatal(e)
		}
		if got.State != want {
			logs, _ := db.Logs(ctx, d.ID, 0)
			t.Fatalf("%s: %s %s logs=%+v", op, got.State, got.Error, logs)
		}
		a, _ = db.Application(ctx, a.ID)
		return got
	}
	t.Log("First deploy runs candidate migration before the web startup hook")
	first := run("deploy", true, "succeeded")
	webID, e := r.taskService(ctx, first, "web")
	if e != nil {
		t.Fatal(e)
	}
	starts := func() string {
		out, e := r.exec(ctx, webID, []string{"cat", "/state/starts"})
		if e != nil {
			t.Fatal(e)
		}
		return string(out)
	}
	if strings.Count(starts(), "start") != 1 {
		t.Fatal("startup did not run once")
	}
	t.Log("Actual Docker restart executes startup hook without rerunning migration")
	if _, e = r.Client.ContainerRestart(ctx, webID, client.ContainerRestartOptions{}); e != nil {
		t.Fatal(e)
	}
	if e = r.Healthy(ctx, first); e != nil {
		t.Fatal(e)
	}
	if strings.Count(starts(), "start") != 2 {
		t.Fatal("restart skipped startup hook")
	}
	runs, _ := db.TaskRuns(ctx, a.ID)
	if len(runs) != 1 {
		t.Fatal("deploy-only migration ran on restart", runs)
	}
	t.Log("Runtime service env changes reuse all images and preserve the database")
	a.Config.Services = []store.ServiceConfig{{Name: "web", Variables: []store.Variable{{Name: "RUNTIME_MESSAGE", Value: "changed", Scope: "runtime"}}}}
	applied := run("apply", false, "succeeded")
	webID, _ = r.taskService(ctx, applied, "web")
	if out, e := r.exec(ctx, webID, []string{"printenv", "RUNTIME_MESSAGE"}); e != nil || !strings.Contains(string(out), "changed") {
		t.Fatal(string(out), e)
	}
	logs, _ := db.Logs(ctx, applied.ID, 0)
	reused := false
	for _, l := range logs {
		if strings.Contains(l.Line, "without cloning or rebuilding") {
			reused = true
		}
	}
	if !reused {
		t.Fatal("runtime update rebuilt source")
	}
	t.Log("Service controls enforce transitive dependencies")
	if e = r.CheckServiceAction(ctx, applied, "db", "stop"); e == nil {
		t.Fatal("allowed stopping a dependency of a running service")
	}
	if e = r.LifecycleService(ctx, applied, "web", "stop"); e != nil {
		t.Fatal(e)
	}
	if e = r.CheckServiceAction(ctx, applied, "db", "stop"); e != nil {
		t.Fatal(e)
	}
	if e = r.LifecycleService(ctx, applied, "web", "start"); e != nil {
		t.Fatal(e)
	}
	t.Log("Saved job snapshot runs against the active release")
	command := store.Command{Service: "web", Argv: []string{"cat", "/state/starts"}, Mode: "job", Timeout: 30}
	task, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "task", TaskID: "manual", TaskCommand: &command}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	w.process(ctx, task)
	task, _ = db.Deployment(ctx, task.ID)
	if task.State != "succeeded" {
		t.Fatal(task)
	}

	t.Log("Controller recovery reattaches a known job without repeating its side effects")
	recoverCommand := store.Command{Service: "web", Shell: "sleep 2; echo once >> /state/jobs", Mode: "job", Timeout: 30}
	recovering, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "task", TaskID: "recover", TaskCommand: &recoverCommand}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	interrupted, stopWorker := context.WithCancel(ctx)
	finished := make(chan struct{})
	go func() { defer close(finished); w.process(interrupted, recovering) }()
	ready := false
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		runs, _ := db.TaskRuns(ctx, a.ID)
		for _, run := range runs {
			if run.DeploymentID == recovering.ID && run.State == "running" {
				ready = true
			}
		}
		if ready {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	stopWorker()
	<-finished
	if !ready {
		t.Fatal("job never started")
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	db, e = store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	w = NewWorker(db, r, events.New(), dir)
	recovering, e = db.Deployment(ctx, recovering.ID)
	if e != nil {
		t.Fatal(e)
	}
	w.process(ctx, recovering)
	recovering, _ = db.Deployment(ctx, recovering.ID)
	if recovering.State != "succeeded" {
		t.Fatal(recovering)
	}
	if out, e := r.exec(ctx, webID, []string{"cat", "/state/jobs"}); e != nil || strings.Count(string(out), "once") != 1 {
		t.Fatal("job was repeated", string(out), e)
	}

	t.Log("Argument-list jobs execute in a shell-free image; shell mode fails clearly")
	imageInfo, e := r.Client.ImageInspect(ctx, ApacheImage)
	if e != nil {
		t.Fatal(e)
	}
	binary, e := startup.Binary(imageInfo.Architecture)
	if e != nil {
		t.Fatal(e)
	}
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	if e = tw.WriteHeader(&tar.Header{Name: "probe", Mode: 0555, Size: int64(len(binary))}); e != nil {
		t.Fatal(e)
	}
	tw.Write(binary)
	tw.Close()
	tag := "shipper/" + instance[:12] + "/no-shell:test"
	imported, e := r.Client.ImageImport(ctx, client.ImageImportSource{SourceName: "-", Source: &archive}, tag, client.ImageImportOptions{Changes: []string{"ENTRYPOINT [\"/probe\",\"--version\"]", "LABEL io.shipper.instance=" + instance}})
	if e != nil {
		t.Fatal(e)
	}
	io.Copy(io.Discard, imported)
	imported.Close()
	image, e := r.Client.ImageInspect(ctx, tag)
	if e != nil {
		t.Fatal(e)
	}
	tiny := store.Application{ID: auth.Token()[:24], Name: "No shell", Config: store.Config{Method: "dockerfile", MemoryMB: 64}}
	tinyRelease, e := db.Enqueue(ctx, tiny, store.Deployment{ID: auth.Token()[:24], Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	tinyRelease.ImageID = image.ID
	tinyRelease.ResolvedMethod = "dockerfile"
	tinyRelease.State = "succeeded"
	db.SaveDeployment(ctx, tinyRelease)
	db.SetActive(ctx, tiny.ID, tinyRelease.ID)
	tiny, _ = db.Application(ctx, tiny.ID)
	for _, mode := range []string{"argv", "shell"} {
		c := store.Command{Argv: []string{"/probe", "--version"}, Mode: "job", Timeout: 30}
		want := "succeeded"
		if mode == "shell" {
			c.Argv = nil
			c.Shell = "echo hello"
			want = "failed"
		}
		job, e := db.Enqueue(ctx, tiny, store.Deployment{ID: auth.Token()[:24], Operation: "task", TaskCommand: &c}, "", false)
		if e != nil {
			t.Fatal(e)
		}
		w.process(ctx, job)
		job, _ = db.Deployment(ctx, job.ID)
		if job.State != want {
			t.Fatal(mode, job)
		}
	}

	t.Log("A failed candidate migration blocks promotion and preserves the active release")
	a, _ = db.Application(ctx, a.ID)
	a.Config.Hooks[0].Command.Shell = "exit 42"
	failed := run("deploy", false, "failed")
	if a.ActiveDeploymentID != applied.ID || !strings.Contains(failed.Error, "42") {
		t.Fatal("failed migration replaced release", failed, a.ActiveDeploymentID)
	}
}
