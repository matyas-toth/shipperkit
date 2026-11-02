package deploy

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os/exec"
	"sort"
	"strings"
	"time"

	"shipper/internal/store"
)

type BranchList struct {
	Items   []string `json:"items"`
	Default string   `json:"default"`
}

func PublicBranches(ctx context.Context, repository string) (BranchList, error) {
	c := store.Config{Repository: repository, Domain: "probe.localhost"}
	if e := Validate(&c); e != nil {
		return BranchList{}, e
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	u, _ := url.Parse(c.Repository)
	addresses, e := net.DefaultResolver.LookupNetIP(ctx, "ip", u.Hostname())
	if e != nil || len(addresses) == 0 {
		return BranchList{}, errors.New("The Git host could not be resolved")
	}
	for _, ip := range addresses {
		if !publicAddress(ip) {
			return BranchList{}, errors.New("Git repositories must resolve to public internet addresses")
		}
	}
	ip := addresses[0].String()
	if addresses[0].Is6() {
		ip = "[" + ip + "]"
	}
	args := []string{"-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "credential.helper=", "-c", "http.followRedirects=false", "-c", "http.curloptResolve=" + u.Hostname() + ":443:" + ip, "ls-remote", "--symref", c.Repository, "HEAD", "refs/heads/*"}
	command := exec.CommandContext(ctx, "git", args...)
	command.Env = gitEnvironment()
	command.WaitDelay = time.Second
	var output cappedBuffer
	command.Stdout = &output
	command.Stderr = &output
	if e = command.Run(); e != nil {
		return BranchList{}, errors.New("Branches could not be loaded from this public repository")
	}
	result := BranchList{Items: []string{}}
	seen := map[string]bool{}
	for _, line := range strings.Split(output.String(), "\n") {
		if strings.HasPrefix(line, "ref: refs/heads/") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				result.Default = strings.TrimPrefix(fields[1], "refs/heads/")
			}
			continue
		}
		_, ref, ok := strings.Cut(line, "\t")
		if !ok || !strings.HasPrefix(ref, "refs/heads/") {
			continue
		}
		name := strings.TrimPrefix(strings.TrimSpace(ref), "refs/heads/")
		if !seen[name] && branchPattern.MatchString(name) {
			result.Items = append(result.Items, name)
			seen[name] = true
		}
		if len(result.Items) >= 500 {
			break
		}
	}
	sort.Strings(result.Items)
	return result, nil
}
