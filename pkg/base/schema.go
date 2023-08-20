package base

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	mysql "github.com/go-sql-driver/mysql"
	"golang.org/x/sync/errgroup"

	"vitess.io/vitess/go/vt/proto/vtrpc"
	"vitess.io/vitess/go/vt/schemadiff"
	"vitess.io/vitess/go/vt/sqlparser"
	"vitess.io/vitess/go/vt/vterrors"
)

// ReadSQLsFromSource returns a list of CREATE TABLE|VIEW statements as read from given input.
func ReadSQLsFromSource(inputSourceValue string) (sqls []string, err error) {
	inputSourceType, err := DetectInputSource(inputSourceValue)
	if err != nil {
		return nil, vterrors.Wrapf(err, "cannot read schema")
	}
	switch inputSourceType {
	case StdInputSource:
		// Read standard input. It may contain any number (zero included) number of CREATE TABLE|VIEW statements,
		// delimtied by ';'
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, vterrors.Wrapf(err, "reading standard output")
		}
		return sqlparser.SplitStatementToPieces(strings.TrimSpace(string(b)))
	case FileInputSource:
		// Read given file. It may contain any number (zero included) number of CREATE TABLE|VIEW statements,
		// delimtied by ';'
		b, err := os.ReadFile(inputSourceValue)
		if err != nil {
			return nil, vterrors.Wrapf(err, "reading file %s", inputSourceValue)
		}
		return sqlparser.SplitStatementToPieces(strings.TrimSpace(string(b)))
	case DirectoryInputSource:
		// Read all .sql files in this directory. Each file assumed to contain a single CREATE TABLE}VIEW statememt.
		var sqls []string
		files, err := os.ReadDir(inputSourceValue)
		if err != nil {
			return nil, vterrors.Wrapf(err, "reading directory %s", inputSourceValue)
		}
		for _, f := range files {
			if f.Type().IsRegular() && strings.ToLower(filepath.Ext(f.Name())) == ".sql" {
				fullPath := filepath.Join(inputSourceValue, f.Name())
				b, err := os.ReadFile(fullPath)
				if err != nil {
					return nil, vterrors.Wrapf(err, "reading file %s", fullPath)
				}
				sqls = append(sqls, string(b))
			}
		}
		return sqls, nil
	case UriInputSource:
		// Read schema from database:
		return readDatabaseSchema(inputSourceValue)
	default:
		return nil, vterrors.Errorf(vtrpc.Code_UNIMPLEMENTED, "input source %v unimplemented", inputSourceValue)
	}
}

// ReadSchemaFromSource returns a loaded, validated, normalized formal Schema from the given source,
// or an error if either the source or the schema are invalid.
func ReadSchemaFromSource(inputSourceValue string) (*schemadiff.Schema, error) {
	sqls, err := ReadSQLsFromSource(inputSourceValue)
	if err != nil {
		return nil, err
	}
	return schemadiff.NewSchemaFromQueries(sqls)
}

// writeEscapedString escapes a table or db name with backtick quotes.
// This code is taken from Vitess, https://github.com/vitessio/vitess/blob/main/go/vt/sqlparser/ast_funcs.go
func writeEscapedString(original string) string {
	var b strings.Builder
	b.WriteByte('`')
	for _, c := range original {
		b.WriteRune(c)
		if c == '`' {
			b.WriteByte('`')
		}
	}
	b.WriteByte('`')
	return b.String()
}

// Given a MySQL connection config (which includes a database name), read CREATE statements for all tables and views
// from given database.
// The given DSN must incidcate a database name, e.g.:
// - "myuser:mypass@unix(/var/lib/mysql/sandbox8032.sock)/mydb"
// It may optionally include a specific table name, in the following way:
// - "myuser:mypass@unix(/var/lib/mysql/sandbox8032.sock)/mydb?#mytable"
func readDatabaseSchema(inputSourceValue string) ([]string, error) {
	cfg, err := mysql.ParseDSN(inputSourceValue)
	if err != nil {
		return nil, vterrors.Wrapf(err, "parsing DSN %s", inputSourceValue)
	}
	if cfg.DBName == "" {
		return nil, vterrors.Errorf(vtrpc.Code_INVALID_ARGUMENT, "DNS must contain schema name")
	}
	cfg.InterpolateParams = true

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	var explicitEntity string
	if idx := strings.Index(inputSourceValue, "#"); idx >= 0 {
		explicitEntity = inputSourceValue[idx+1:]
	}
	names := map[string]bool{} // key for table/view name, 'true' for table, 'false' for view

	// readNames reads names of all tables and views in the given database
	readNames := func() error {
		query := `SELECT TABLE_NAME, TABLE_TYPE FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = ?`
		args := []any{cfg.DBName}
		if explicitEntity != "" {
			query = query + " AND TABLE_NAME = ?"
			args = append(args, explicitEntity)
		}
		rows, err := db.Query(query, args...)

		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var entityName string
			var entityType string
			if err := rows.Scan(&entityName, &entityType); err != nil {
				return err
			}
			isTable := (entityType == "BASE TABLE")
			names[entityName] = isTable
		}
		return nil
	}
	if err := readNames(); err != nil {
		return nil, vterrors.Wrapf(err, "reading %s table and view names", writeEscapedString(cfg.DBName))
	}
	var sqls = make([]string, 0, len(names))
	var mu sync.Mutex

	var errs errgroup.Group
	errs.SetLimit(20)
	for name, isTable := range names {
		name := name
		isTable := isTable
		errs.Go(func() error {
			var query string
			if isTable {
				query = fmt.Sprintf("SHOW CREATE TABLE %s", writeEscapedString(name))
			} else {
				query = fmt.Sprintf("SHOW CREATE VIEW %s", writeEscapedString(name))
			}
			rows, err := db.Query(query)
			if err != nil {
				return vterrors.Wrapf(err, "showing CREATE statement for %s", name)
			}
			defer rows.Close()

			var createStatement string
			var placeholder1, placeholder2, placeholder3 string
			for rows.Next() { // There really is a single row in the result
				if isTable {
					err = rows.Scan(&placeholder1, &createStatement)
				} else {
					err = rows.Scan(&placeholder1, &createStatement, &placeholder2, &placeholder3)
				}
				if err != nil {
					return vterrors.Wrapf(err, "reading CREATE statement for %s", name)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			sqls = append(sqls, createStatement)
			return nil
		})
	}
	if err := errs.Wait(); err != nil {
		return nil, err
	}
	return sqls, nil
}
