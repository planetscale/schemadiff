package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/planetscale/schemadiff/pkg/core"
	flag "github.com/spf13/pflag"
)

func exitWithError(err error) {
	fmt.Println(err)
	os.Exit(2)
}

func main() {
	ctx := context.Background()

	source := flag.String("source", "", "Input source (file name / directory / empty for stdin)")
	target := flag.String("target", "", "Input target (file name / directory / empty for stdin)")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		exitWithError(errors.New("one argument expected. Usage: schemadiff [flags...] <load|diff|ordered-diff|diff-table|diff-view>"))
	}
	command := args[0]
	output, err := core.Exec(ctx, command, *source, *target)
	if err != nil {
		exitWithError(err)
	}
	fmt.Print(output)
}
