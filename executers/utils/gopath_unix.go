// +build linux darwin freebsd

package utils

import "strings"

func parseGoPath(goPath string) string {
	goPathSlice := strings.Split(goPath, ":")
	return goPathSlice[0]
}