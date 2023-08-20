package base

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// FYI ReadSQLsFromSource() and ReadSchemaFromSource() are (indirectly) tested in exec_test.go
func TestWriteEscapedString(t *testing.T) {
	expectedEscaping := map[string]string{
		"mydb":  "`mydb`",
		"my db": "`my db`",
		"my`db": "`my``db`",
	}
	for k, v := range expectedEscaping {
		t.Run(k, func(t *testing.T) {
			assert.Equal(t, v, writeEscapedString(k))
		})
	}
}
