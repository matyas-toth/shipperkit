package deploy

import (
	"regexp"
	"strings"
	"testing"
)

func TestReadableContainerNames(t *testing.T) {
	release := "abcdef1234567890abcdef12"
	if got := ContainerName("ReigHome Local", "", release); got != "shp-reighome-local-abcdef123456" {
		t.Fatal(got)
	}
	if got := ContainerName("My App", "Background Worker", release); got != "shp-my-app-background-worker-abcdef123456" {
		t.Fatal(got)
	}
	for _, name := range []string{"", " App / Weird__ Name! ", "日本語", strings.Repeat("very-long-", 20)} {
		first := ContainerName(name, "", release)
		if !regexp.MustCompile(`^shp-[a-z0-9-]+-[a-f0-9]{12}$`).MatchString(first) || len(first) > 57 {
			t.Fatal(first)
		}
		if first == ContainerName(name, "", "0123456789abcdef01234567") {
			t.Fatal("release names collide")
		}
	}
}
