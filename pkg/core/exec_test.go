package core

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"vitess.io/vitess/go/vt/schemadiff"
	"vitess.io/vitess/go/vt/sqlparser"
)

// This unit-test file validates the high level operation of the Exec function, and specifically its
// ability to load and diff schemas and tables from file and from directory sources.
// The purpose here is to verify the logistics of loading/diffing/output.
// This is not a comprehensive SQL synaxt load/diff coverage test. Please refer to upstream Vitess
// `schemadiff` package, https://github.com/vitessio/vitess/tree/main/go/vt/schemadiff.
// Note: stdin and MySQL sources are tested in the CI test found in script/ci-github.sh
var (
	schemaFrom = []string{
		"create table t1 (id int primary key)",
		"create view v1 as select id from t1",
		"create table t2 (id int, name varchar(12), primary key(id), key name_idx(name))",
	}
	schemaTo = []string{
		"create table t1 (id int unsigned primary key)",
		"create view vone as select id from t1",
		"create table t2 (id int, name varchar(12), primary key(id), key name_idx(name))",
		"create table t3 (id int, age int unsigned, primary key(id))",
	}
	loadFrom = []string{
		"CREATE TABLE `t1` (\n\t`id` int,\n\tPRIMARY KEY (`id`)\n)",
		"CREATE TABLE `t2` (\n\t`id` int,\n\t`name` varchar(12),\n\tPRIMARY KEY (`id`),\n\tKEY `name_idx` (`name`)\n)",
		"CREATE VIEW `v1` AS SELECT `id` FROM `t1`",
	}
	loadTo = []string{
		"CREATE TABLE `t1` (\n\t`id` int unsigned,\n\tPRIMARY KEY (`id`)\n)",
		"CREATE TABLE `t2` (\n\t`id` int,\n\t`name` varchar(12),\n\tPRIMARY KEY (`id`),\n\tKEY `name_idx` (`name`)\n)",
		"CREATE TABLE `t3` (\n\t`id` int,\n\t`age` int unsigned,\n\tPRIMARY KEY (`id`)\n)",
		"CREATE VIEW `vone` AS SELECT `id` FROM `t1`",
	}
	diffsFromTo = []string{
		"DROP VIEW `v1`",
		"ALTER TABLE `t1` MODIFY COLUMN `id` int unsigned",
		"CREATE TABLE `t3` (\n\t`id` int,\n\t`age` int unsigned,\n\tPRIMARY KEY (`id`)\n)",
		"CREATE VIEW `vone` AS SELECT `id` FROM `t1`",
	}
	diffsToFrom = []string{
		"DROP VIEW `vone`",
		"DROP TABLE `t3`",
		"ALTER TABLE `t1` MODIFY COLUMN `id` int",
		"CREATE VIEW `v1` AS SELECT `id` FROM `t1`",
	}
)

func sqlsToMultiStatementText(sqls []string) string {
	var b strings.Builder
	for _, sql := range sqls {
		b.WriteString(sql)
		b.WriteString(";\n")
	}
	return b.String()
}

func writeSchemaFile(t *testing.T, schema []string) (fileName string) {
	sql := sqlsToMultiStatementText(schema)

	f, err := os.CreateTemp(os.TempDir(), "schemadiff-unittest-file-*")
	require.NoError(t, err)
	require.NotNil(t, f)
	n, err := f.Write([]byte(sql))
	if schema != nil {
		require.NotZero(t, n)
	}
	require.NoError(t, err)

	return f.Name()
}

func writeSchemaDir(t *testing.T, schema []string) (fileName string) {
	dir, err := os.MkdirTemp(os.TempDir(), "schemadiff-unittest-dir-*")
	require.NoError(t, err)
	require.NotEmpty(t, dir)

	for _, sql := range schema {
		f, err := os.CreateTemp(dir, "*.sql")
		require.NoError(t, err)
		require.NotNil(t, f)
		n, err := f.Write([]byte(sql))
		require.NotZero(t, n)
		require.NoError(t, err)
	}
	return dir
}

func TestExecLoad(t *testing.T) {
	ctx := context.Background()
	t.Run("from-file", func(t *testing.T) {
		fileFrom := writeSchemaFile(t, schemaFrom)
		require.NotEmpty(t, fileFrom)
		defer os.RemoveAll(fileFrom)
		schema, err := Exec(ctx, "load", fileFrom, "", false)
		assert.NoError(t, err)
		assert.Equal(t, sqlsToMultiStatementText(loadFrom), schema)
	})
	t.Run("from-file-textual", func(t *testing.T) {
		fileFrom := writeSchemaFile(t, schemaFrom)
		require.NotEmpty(t, fileFrom)
		defer os.RemoveAll(fileFrom)
		schema, err := Exec(ctx, "load", fileFrom, "", true)
		assert.NoError(t, err)
		expect := []string{}
		for _, sql := range loadFrom {
			lines := strings.Split(sql, "\n")
			for i := range lines {
				lines[i] = "+" + lines[i]
			}
			sql = strings.Join(lines, "\n")
			expect = append(expect, sql)
		}
		assert.Equal(t, sqlsToMultiStatementText(expect), schema)
	})

	t.Run("from-dir", func(t *testing.T) {
		dirFrom := writeSchemaDir(t, schemaFrom)
		require.NotEmpty(t, dirFrom)
		defer os.RemoveAll(dirFrom)
		schema, err := Exec(ctx, "load", dirFrom, "", false)
		assert.NoError(t, err)
		assert.Equal(t, sqlsToMultiStatementText(loadFrom), schema)
	})

	t.Run("to-file", func(t *testing.T) {
		fileTo := writeSchemaFile(t, schemaTo)
		require.NotEmpty(t, fileTo)
		defer os.RemoveAll(fileTo)
		schema, err := Exec(ctx, "load", fileTo, "", false)
		assert.NoError(t, err)
		assert.Equal(t, sqlsToMultiStatementText(loadTo), schema)
	})

	t.Run("to-dir", func(t *testing.T) {
		dirTo := writeSchemaDir(t, schemaTo)
		require.NotEmpty(t, dirTo)
		defer os.RemoveAll(dirTo)
		schema, err := Exec(ctx, "load", dirTo, "", false)
		assert.NoError(t, err)
		assert.Equal(t, sqlsToMultiStatementText(loadTo), schema)
	})
	t.Run("empty", func(t *testing.T) {
		emptyFile := writeSchemaFile(t, nil)
		require.NotEmpty(t, emptyFile) // testing that the *name* is not empty...
		defer os.RemoveAll(emptyFile)

		schema, err := Exec(ctx, "load", emptyFile, "", false)
		assert.NoError(t, err)
		assert.Equal(t, "", schema)
	})
}

func TestExecDiff(t *testing.T) {
	ctx := context.Background()

	fileFrom := writeSchemaFile(t, schemaFrom)
	require.NotEmpty(t, fileFrom)
	defer os.RemoveAll(fileFrom)

	fileTo := writeSchemaFile(t, schemaTo)
	require.NotEmpty(t, fileTo)
	defer os.RemoveAll(fileTo)

	dirFrom := writeSchemaDir(t, schemaFrom)
	require.NotEmpty(t, dirFrom)
	defer os.RemoveAll(dirFrom)

	dirTo := writeSchemaDir(t, schemaTo)
	require.NotEmpty(t, dirTo)
	defer os.RemoveAll(dirTo)

	emptyFile := writeSchemaFile(t, nil)
	defer os.RemoveAll(emptyFile)

	tcases := []struct {
		name        string
		source      string
		target      string
		textual     bool
		expectDiff  []string
		expectError string
	}{
		{
			name:       "file-file",
			source:     fileFrom,
			target:     fileTo,
			expectDiff: diffsFromTo,
		},
		{
			name:       "file-dir",
			source:     fileFrom,
			target:     dirTo,
			expectDiff: diffsFromTo,
		},
		{
			name:       "dir-file",
			source:     dirFrom,
			target:     fileTo,
			expectDiff: diffsFromTo,
		},
		{
			name:       "dir-dir",
			source:     dirFrom,
			target:     dirTo,
			expectDiff: diffsFromTo,
		},
		{
			name:       "file-file-reverse",
			source:     fileTo,
			target:     fileFrom,
			expectDiff: diffsToFrom,
		},
		{
			name:       "file-dir-reverse",
			source:     dirTo,
			target:     fileFrom,
			expectDiff: diffsToFrom,
		},
		{
			name:       "dir-file-reverse",
			source:     fileTo,
			target:     dirFrom,
			expectDiff: diffsToFrom,
		},
		{
			name:       "dir-dir-reverse",
			source:     dirTo,
			target:     dirFrom,
			expectDiff: diffsToFrom,
		},
		{
			name:       "diff-empty-from",
			source:     emptyFile,
			target:     fileFrom,
			expectDiff: loadFrom,
		},
		{
			name:       "diff-empty-to",
			source:     emptyFile,
			target:     fileTo,
			expectDiff: loadTo,
		},
		{
			name:        "dir-nodir",
			source:      dirFrom,
			target:      "/no/such/directory/to/be/found",
			expectError: "unknown input source",
		},
		{
			name:        "nodir-dir",
			source:      "/no/such/directory/to/be/found",
			target:      dirTo,
			expectError: "unknown input source",
		},
		{
			name:        "same dir",
			source:      dirTo,
			target:      dirTo,
			expectError: ErrIdenticalSourceTarget.Error(),
		},
		{
			name:        "same stdin",
			source:      "",
			target:      "",
			expectError: ErrIdenticalSourceTarget.Error(),
		},
	}
	parser := sqlparser.NewTestParser()
	for _, cmd := range []string{"diff", "ordered-diff"} {
		t.Run(cmd, func(t *testing.T) {
			for _, tcase := range tcases {
				t.Run(tcase.name, func(t *testing.T) {
					diff, err := Exec(ctx, cmd, tcase.source, tcase.target, tcase.textual)
					if tcase.expectError == "" {
						assert.NoError(t, err)
						switch cmd {
						case "diff":
							assert.Equal(t, sqlsToMultiStatementText(tcase.expectDiff), diff)
						case "ordered-diff":
							// Remember this unit test is not about validating schemadiff logic, we only care about
							// how we read information from schemadiff. The result of OrderedDiffs(), when successful,
							// is the same set of diffs as UnorderedDiffs(), but in different order.
							// In the below we do some plumbing to extract and normalize all the queries, then
							// compare the diffs ignoring order.
							sqls, err := parser.SplitStatementToPieces(diff)
							require.NoError(t, err)
							for i := range sqls {
								stmt, err := parser.Parse(sqls[i])
								require.NoError(t, err)
								sqls[i] = sqlparser.CanonicalString(stmt)
							}

							expects := []string{}
							for i := range tcase.expectDiff {
								stmt, err := parser.Parse(tcase.expectDiff[i])
								require.NoError(t, err)
								expects = append(expects, sqlparser.CanonicalString(stmt))
							}

							assert.ElementsMatch(t, expects, sqls)
						default:
							assert.Failf(t, "unknown command: %v", cmd)
						}
					} else {
						assert.Error(t, err)
						assert.ErrorContains(t, err, tcase.expectError)
					}
				})
			}
		})
	}
}

func TestExecDiffTable(t *testing.T) {
	ctx := context.Background()
	t.Run("t1-t1", func(t *testing.T) {
		from := writeSchemaFile(t, schemaFrom[0:1])
		require.NotEmpty(t, from)
		defer os.RemoveAll(from)

		to := writeSchemaFile(t, schemaTo[0:1])
		require.NotEmpty(t, to)
		defer os.RemoveAll(to)

		diff, err := Exec(ctx, "diff-table", from, to, false)
		assert.NoError(t, err)
		assert.Equal(t, "ALTER TABLE `t1` MODIFY COLUMN `id` int unsigned;\n", diff)
	})
	t.Run("t1-t3", func(t *testing.T) {
		from := writeSchemaFile(t, schemaFrom[0:1])
		require.NotEmpty(t, from)
		defer os.RemoveAll(from)

		to := writeSchemaFile(t, schemaTo[3:])
		require.NotEmpty(t, to)
		defer os.RemoveAll(to)

		diff, err := Exec(ctx, "diff-table", from, to, false)
		assert.NoError(t, err)
		assert.Equal(t, "ALTER TABLE `t1` ADD COLUMN `age` int unsigned;\n", diff)
	})
	t.Run("t1-t3-textual", func(t *testing.T) {
		from := writeSchemaFile(t, schemaFrom[0:1])
		require.NotEmpty(t, from)
		defer os.RemoveAll(from)

		to := writeSchemaFile(t, schemaTo[3:])
		require.NotEmpty(t, to)
		defer os.RemoveAll(to)

		diff, err := Exec(ctx, "diff-table", from, to, true)
		assert.NoError(t, err)
		assert.Equal(t, " CREATE TABLE `t1` (\n \t`id` int,\n+\t`age` int unsigned,\n \tPRIMARY KEY (`id`)\n );\n", diff)
	})
	t.Run("t1-vone", func(t *testing.T) {
		from := writeSchemaFile(t, schemaFrom[0:1])
		require.NotEmpty(t, from)
		defer os.RemoveAll(from)

		to := writeSchemaFile(t, schemaTo[1:2])
		require.NotEmpty(t, to)
		defer os.RemoveAll(to)

		_, err := Exec(ctx, "diff-table", from, to, false)
		assert.Error(t, err)
		assert.ErrorIs(t, err, schemadiff.ErrExpectedCreateTable)
	})
	t.Run("v1-vone", func(t *testing.T) {
		from := writeSchemaFile(t, schemaFrom[1:2])
		require.NotEmpty(t, from)
		defer os.RemoveAll(from)

		to := writeSchemaFile(t, schemaTo[1:2])
		require.NotEmpty(t, to)
		defer os.RemoveAll(to)

		diff, err := Exec(ctx, "diff-view", from, to, false)
		assert.NoError(t, err)
		assert.Empty(t, diff)
	})
	t.Run("v1-v2", func(t *testing.T) {
		from := writeSchemaFile(t, schemaFrom[1:2])
		require.NotEmpty(t, from)
		defer os.RemoveAll(from)

		to := writeSchemaFile(t, []string{"create view v2 as select id, 1 from t1"})
		require.NotEmpty(t, to)
		defer os.RemoveAll(to)

		diff, err := Exec(ctx, "diff-view", from, to, false)
		assert.NoError(t, err)
		assert.Equal(t, "ALTER VIEW `v1` AS SELECT `id`, 1 FROM `t1`;\n", diff)
	})
	t.Run("t1-schema", func(t *testing.T) {
		from := writeSchemaFile(t, schemaFrom[0:1])
		require.NotEmpty(t, from)
		defer os.RemoveAll(from)

		to := writeSchemaFile(t, schemaTo)
		require.NotEmpty(t, to)
		defer os.RemoveAll(to)

		{
			_, err := Exec(ctx, "diff-table", from, to, false)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "expected one CREATE TABLE statement")
		}
		{
			_, err := Exec(ctx, "diff-table", to, from, false)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "expected one CREATE TABLE statement")
		}
	})
}
