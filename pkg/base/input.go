package base

import (
	"fmt"
	"os"

	mysql "github.com/go-sql-driver/mysql"
)

type InputSource int

const (
	UnknownInputSource InputSource = iota
	StdInputSource
	FileInputSource
	DirectoryInputSource
	UriInputSource
)

type ErrUnknownInputSource struct {
	sourceVal string
}

func (e *ErrUnknownInputSource) Error() string {
	return fmt.Sprintf("unknown input source: %s", e.sourceVal)
}

// DetectInputSource auto detects the type of source by the given value; it checks if the value
// is a file, directory, etc.
func DetectInputSource(inputSourceValue string) (InputSource, error) {
	if inputSourceValue == "" {
		return StdInputSource, nil
	}
	if fileInfo, err := os.Stat(inputSourceValue); err == nil {
		if fileInfo.IsDir() {
			return DirectoryInputSource, nil
		}
		return FileInputSource, nil
	}
	if _, err := mysql.ParseDSN(inputSourceValue); err == nil {
		return UriInputSource, nil
	}

	return UnknownInputSource, &ErrUnknownInputSource{sourceVal: inputSourceValue}
}
