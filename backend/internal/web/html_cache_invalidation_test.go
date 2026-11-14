//go:build embed

package web

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTMLCache_RejectRenderAfterInvalidation(t *testing.T) {
	c := NewHTMLCache()
	c.SetBaseHTML([]byte("base"))
	version := c.Version()
	c.Invalidate()
	c.Invalidate() // repeated event is safe
	require.False(t, c.SetIfVersion([]byte("old menu"), []byte(`{}`), version))
	require.Nil(t, c.Get())
	require.True(t, c.SetIfVersion([]byte("new menu"), []byte(`{}`), c.Version()))
	require.Equal(t, []byte("new menu"), c.Get().Content)
}
