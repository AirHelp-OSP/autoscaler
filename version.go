package main

import (
	"fmt"
)

var version string

func versionString() string {
	return fmt.Sprintf("Autoscaler version: %v", version)
}
