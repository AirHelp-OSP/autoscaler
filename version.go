package main

import (
	_ "embed"
	"fmt"
)

//go:embed version.txt
var version string

func versionString() string {
	return fmt.Sprintf("Autoscaler version: %v", version)
}
