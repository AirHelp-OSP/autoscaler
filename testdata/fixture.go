package testdata

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// LoadFixture returns content of file from fixtures directory
func LoadFixture(filename string) string {
	rootPath := findProjectRootPath()

	if rootPath == "" {
		panic("Cannot determine root path")
	}

	path := fmt.Sprintf("%v/testdata/fixtures/%v", rootPath, filename)

	data, err := os.ReadFile(path)

	if err != nil {
		fmt.Println("Error while loading fixture file")
		panic(err)
	}

	return string(data)

}

// Current directory returns dir of test file. Due to project structure this can result in multiple nestings
// `findProjectRootPath` finds root directory of whole project by looking for `testdata` directory
func findProjectRootPath() string {
	maxDepth := 5

	currentDirname, _ := os.Getwd()

	d := strings.Builder{}

	for i := 1; i < maxDepth; i++ {
		path := path.Join(currentDirname, d.String())

		if _, err := os.Stat(fmt.Sprintf("%v/%v", path, "testdata")); os.IsNotExist(err) {
			d.WriteString("../")
		} else {
			return fmt.Sprintf("%v/%v", currentDirname, d.String())
		}
	}

	return ""
}
