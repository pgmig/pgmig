package pgmig

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testHasGit bool

func init() {
	if _, err := exec.LookPath("git"); err == nil {
		testHasGit = true
	}
}

func TestGitVersion(t *testing.T) {
	if !testHasGit {
		t.Log("git not found, skipping")
		t.Skip()
	}

	var rv string
	if err := GitVersion(".", &rv); err != nil {
		t.Fatalf("err: %s", err)
	}
	assert.NotEqual(t, rv, "")

	if err := GitVersion("/var", &rv); err == nil {
		t.Fatalf("Call must return error")
	}
}

func TestGitRepo(t *testing.T) {
	if !testHasGit {
		t.Log("git not found, skipping")
		t.Skip()
	}

	var rv string
	if err := GitRepo(".", &rv); err != nil {
		t.Fatalf("err: %s", err)
	}
	assert.NotEqual(t, rv, "")

	if err := GitRepo("/var", &rv); err == nil {
		t.Fatalf("Call must return error")
	}
}
