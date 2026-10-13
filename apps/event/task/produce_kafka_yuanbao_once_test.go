package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnitNewHistoryAuthorityYuanbao(t *testing.T) {
	assert.NotNil(t, NewHistoryAuthorityYuanbao())
}

func TestUnit_historyAuthorityYuanbao_getReq(t *testing.T) {
	c := &historyAuthorityYuanbao{}
	assert.NotNil(t, c.getReq())
}
