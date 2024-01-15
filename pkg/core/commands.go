package core

import (
	"fmt"

	"vitess.io/vitess/go/vt/schemadiff"

	"github.com/planetscale/schemadiff/pkg/base"
)

var (
	defaultDiffHints = &schemadiff.DiffHints{
		AutoIncrementStrategy: schemadiff.AutoIncrementIgnore,
		RangeRotationStrategy: schemadiff.RangeRotationDistinctStatements,
	}
)

// LoadSchema returns a Schema, loaded from given input. The Schema is loaded, validated and normalized.
// Input can be stdin, file, directory, or MySQL URI.
func LoadSchema(env *schemadiff.Environment, inputSourceValue string) (*schemadiff.Schema, error) {
	return base.ReadSchemaFromSource(env, inputSourceValue)
}

// DiffSchemas returns a rich diff between two given schemas.
// Inputs can be stdin, file, directory, or MySQL URI.
func DiffSchemas(env *schemadiff.Environment, inputSourceValue string, targetInputSourceValue string) (*schemadiff.SchemaDiff, error) {
	sourceSchema, err := base.ReadSchemaFromSource(env, inputSourceValue)
	if err != nil {
		return nil, err
	}
	targetSchema, err := base.ReadSchemaFromSource(env, targetInputSourceValue)
	if err != nil {
		return nil, err
	}
	return sourceSchema.SchemaDiff(targetSchema, defaultDiffHints)
}

// DiffTables returns a rich diff between two given tables. The function expect the inputs to each
// contain a single CREATE TABLE statement, and returns with error if not so. The two tables are allowed to have different names.
// Inputs can be stdin, file, directory, or MySQL URI.
func DiffTables(env *schemadiff.Environment, inputSourceValue string, targetInputSourceValue string) (schemadiff.EntityDiff, error) {
	readTableSQL := func(sourceValue string) (string, error) {
		sqls, err := base.ReadSQLsFromSource(env, sourceValue)
		if err != nil {
			return "", err
		}
		if len(sqls) != 1 {
			return "", fmt.Errorf("expected one CREATE TABLE statement, found %d entities in %v", len(sqls), sourceValue)
		}
		return sqls[0], nil
	}
	sourceTable, err := readTableSQL(inputSourceValue)
	if err != nil {
		return nil, err
	}
	targetTable, err := readTableSQL(targetInputSourceValue)
	if err != nil {
		return nil, err
	}
	return schemadiff.DiffCreateTablesQueries(env, sourceTable, targetTable, defaultDiffHints)
}

// DiffViews returns a rich diff between two given views. The function expect the inputs to each
// contain a single CREATE VIEW statement, and returns with error if not so. The two views are allowed to have different names.
// Inputs can be stdin, file, directory, or MySQL URI.
func DiffViews(env *schemadiff.Environment, inputSourceValue string, targetInputSourceValue string) (schemadiff.EntityDiff, error) {
	readViewSQL := func(sourceValue string) (string, error) {
		sqls, err := base.ReadSQLsFromSource(env, sourceValue)
		if err != nil {
			return "", err
		}
		if len(sqls) != 1 {
			return "", fmt.Errorf("expected one CREATE VIEW statement, found %d entities in %v", len(sqls), sourceValue)
		}
		return sqls[0], nil
	}
	sourceView, err := readViewSQL(inputSourceValue)
	if err != nil {
		return nil, err
	}
	targetView, err := readViewSQL(targetInputSourceValue)
	if err != nil {
		return nil, err
	}
	return schemadiff.DiffCreateViewsQueries(env, sourceView, targetView, defaultDiffHints)
}
