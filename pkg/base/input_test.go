package base

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectInputSource(t *testing.T) {
	f, err := os.CreateTemp(os.TempDir(), "schemadiff-unit-")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	tcases := []struct {
		val       string
		expect    InputSource
		expectErr bool
	}{
		{
			"", StdInputSource, false,
		},
		{
			os.TempDir(), DirectoryInputSource, false,
		},
		{
			f.Name(), FileInputSource, false,
		},
		{
			"no/such/file/or/dir", UnknownInputSource, true,
		},
	}
	for _, tcase := range tcases {
		t.Run(tcase.val, func(t *testing.T) {
			result, err := DetectInputSource(tcase.val)
			if tcase.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expect, result)
			}
		})
	}
}
