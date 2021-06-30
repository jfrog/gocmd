package utils

import (
	"github.com/jfrog/jfrog-client-go/utils/log"
	"runtime"
	"strings"
	"testing"
)

func TestParseGoPathWindows(t *testing.T) {
	log.SetLogger(log.NewLogger(log.DEBUG, nil))
	if runtime.GOOS != "windows" {
		log.Debug("Skipping the test since not running on Windows OS")
		return
	}
	tests := []struct {
		name     string
		goPath   string
		expected string
	}{
		{"One go path", "C:\\Users\\JFrog\\go", "C:\\Users\\JFrog\\go"},
		{"Multiple go paths", "C:\\Users\\JFrog\\go;C:\\Users\\JFrog\\go2;C:\\Users\\JFrog\\go3", "C:\\Users\\JFrog\\go"},
		{"Empty path", "", ""},
	}

	runGoPathTests(tests, t)
}

func TestParseGoPathUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	tests := []struct {
		name     string
		goPath   string
		expected string
	}{
		{"One go path", "/Users/jfrog/go", "/Users/jfrog/go"},
		{"Multiple go paths", "/Users/jfrog/go:/Users/jfrog/go2:/Users/jfrog/go3", "/Users/jfrog/go"},
		{"Empty path", "", ""},
	}

	runGoPathTests(tests, t)
}

func runGoPathTests(tests []struct {
	name     string
	goPath   string
	expected string
}, t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := parseGoPath(test.goPath)
			if !strings.EqualFold(actual, test.expected) {
				t.Errorf("Test name: %s: Expected: %v, Got: %v", test.name, test.expected, actual)
			}
		})
	}
}
