package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"vitess.io/vitess/go/mysql/collations"
	"vitess.io/vitess/go/vt/schemadiff"
	"vitess.io/vitess/go/vt/vtenv"
)

var (
	ErrIdenticalSourceTarget = errors.New("--source and --target must be different")

	timeout = time.Minute * 5
)

const mysqlVersion = "8.0.35"

// Exec is the main execution entry for this app, called by the main() function.
// Teh function returns a textual output, which is later send to standard output.
func Exec(ctx context.Context, command string, source string, target string) (output string, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	collEnv := collations.NewEnvironment(mysqlVersion)
	vtenv, err := vtenv.New(vtenv.Options{
		MySQLServerVersion: mysqlVersion,
	})
	if err != nil {
		return "", err
	}
	env := schemadiff.NewEnv(vtenv, collEnv.DefaultConnectionCharset())
	var bld strings.Builder
	getDiffs := func(ordered bool) (err error) {
		if source == target {
			return ErrIdenticalSourceTarget
		}
		diff, err := DiffSchemas(env, source, target)
		if err != nil {
			return err
		}

		var diffs []schemadiff.EntityDiff
		if ordered {
			diffs, err = diff.OrderedDiffs(ctx)
			if err != nil {
				return err
			}
		} else {
			diffs = diff.UnorderedDiffs()
		}
		for _, d := range diffs {
			bld.WriteString(d.CanonicalStatementString())
			bld.WriteString(";\n")
		}
		return nil
	}
	switch command {
	case "load":
		schema, err := LoadSchema(env, source)
		if err != nil {
			return "", err
		}
		for _, e := range schema.Entities() {
			bld.WriteString(e.Create().CanonicalStatementString())
			bld.WriteString(";\n")
		}
	case "diff":
		if err := getDiffs(false); err != nil {
			return "", err
		}
	case "ordered-diff":
		if err := getDiffs(true); err != nil {
			return "", err
		}
	case "diff-table":
		if source == target {
			return "", ErrIdenticalSourceTarget
		}
		diff, err := DiffTables(env, source, target)
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
		diff, err := DiffViews(env, source, target)
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
