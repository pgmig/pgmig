package pgmig

import (
	"os/exec"
	"strings"
)

// GitVersion fills rv with package version from git
func GitVersion(path string, rv *string) error {
	out, err := exec.Command("git", "-C", path, "describe", "--tags", "--always").Output()
	if err != nil {
		return err
	}
	*rv = strings.TrimSuffix(string(out), "\n")
	return nil
}

// GitRepo fills rv with package repo from git
func GitRepo(path string, rv *string) error {
	out, err := exec.Command("git", "-C", path, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return err
	}
	*rv = strings.TrimSuffix(string(out), "\n")
	return nil
}
