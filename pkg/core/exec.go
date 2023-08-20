package core

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrIdenticalSourceTarget = errors.New("--source and --target must be different")
)

// Exec is the main execution entry for this app, called by the main() function.
// Teh function returns a textual output, which is later send to standard output.
func Exec(command string, source string, target string) (output string, err error) {
	var bld strings.Builder
	switch command {
	case "load":
		schema, err := LoadSchema(source)
		if err != nil {
			return "", err
		}
		for _, e := range schema.Entities() {
			bld.WriteString(e.Create().CanonicalStatementString())
			bld.WriteString(";\n")
		}
	case "diff":
		if source == target {
			return "", ErrIdenticalSourceTarget
		}
		diff, err := DiffSchemas(source, target)
		if err != nil {
			return "", err
		}
		for _, d := range diff.UnorderedDiffs() {
			bld.WriteString(d.CanonicalStatementString())
			bld.WriteString(";\n")
		}
	case "diff-table":
		if source == target {
			return "", ErrIdenticalSourceTarget
		}
		diff, err := DiffTables(source, target)
		if err != nil {
			return "", err
		}
		if !diff.IsEmpty() {
			bld.WriteString(diff.CanonicalStatementString())
			bld.WriteString(";\n")
		}
	case "diff-view":
		if source == target {
			return "", ErrIdenticalSourceTarget
		}
		diff, err := DiffViews(source, target)
		if err != nil {
			return "", err
		}
		if !diff.IsEmpty() {
			bld.WriteString(diff.CanonicalStatementString())
			bld.WriteString(";\n")
		}
	case "apply":
	default:
		return "", fmt.Errorf("unknown command: %s", command)
	}
	return bld.String(), nil
}
