package utils

import (
	"errors"
	"testing"
)

func TestDependencyNotFoundInArtifactory(t *testing.T) {
	tests := []struct {
		name       string
		noRegistry bool
		err        error
		expected   bool
	}{
		{"withRegistryAndError", true, errors.New("404 Not Found: github.com/package@v1.0.0"), false},
		{"withoutRegistryAndError", false, errors.New("404 Not Found: github.com/package@v1.0.0"), true},
		{"withoutResponseMessageWithoutRegistry", false, errors.New("404: github.com/package@v1.0.0"), true},
		{"withoutResponseMessageWithoutSpaceWithoutRegistry", false, errors.New("404 : github.com/package@v1.0.0"), true},
		{"withoutResponseMessageWithRegistry", true, errors.New("404: github.com/package@v1.0.0"), false},
		{"withoutResponseMessageWithSpaceWithoutRegistry", true, errors.New("404 : github.com/package@v1.0.0"), false},
		{"withRegistryNotContainsError", true, errors.New("This error doesn't contain the error message"), false},
		{"withoutRegistryNotContainsError", false, errors.New("This error doesn't contain the error message"), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := DependencyNotFoundInArtifactory(test.err, test.noRegistry)
			if test.expected != actual {
				t.Errorf("Test name: %s: Expected: %v, Got: %v", test.name, test.expected, actual)
			}
		})
	}
}
