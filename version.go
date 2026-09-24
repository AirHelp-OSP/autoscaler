package main

import (
	"fmt"
)

var version = "development"

func versionString() string {
	return fmt.Sprintf("Autoscaler version: %v", version)
}
