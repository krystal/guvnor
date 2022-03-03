package guvnor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessStatuses_OrderedKeys(t *testing.T) {
	ps := ProcessStatuses{
		"z": {},
		"h": {},
		"a": {},
		"b": {},
	}

	got := ps.OrderedKeys()
	want := []string{"a", "b", "h", "z"}
	assert.Equal(t, want, got)
}
