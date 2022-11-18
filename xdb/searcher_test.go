package xdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const defaultDbPathForTestOnly = "../data/igr.xdb"

func TestSearch(t *testing.T) {
	searcher, _ := Create(defaultDbPathForTestOnly, CACHE_POLICY_FILE)
	// 2.12.133.0|2.12.139.255|法国|0|Ille-et-Vilaine|0|橘子电信
	region, _ := searcher.SearchByStr("2.12.133.0")
	assert.Equal(t, "法国|0|Ille-et-Vilaine|0|橘子电信", region, "set error")
}
