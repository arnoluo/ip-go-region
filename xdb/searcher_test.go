package xdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const defaultDbPathForTestOnly = "../data/igr.xdb"

func TestFileSearcher(t *testing.T) {
	fileSe, _ := Create(defaultDbPathForTestOnly, CACHE_POLICY_FILE)
	// 2.12.133.0|2.12.139.255|法国|0|Ille-et-Vilaine|0|橘子电信
	region, _ := fileSe.SearchByStr("2.12.133.0")
	assert.Equal(t, "法国|0|Ille-et-Vilaine|0|橘子电信", region, "set error")
}

func TestVectorSearcher(t *testing.T) {
	vectorSe, _ := Create(defaultDbPathForTestOnly, CACHE_POLICY_VECTOR)
	// 2.12.133.0|2.12.139.255|法国|0|Ille-et-Vilaine|0|橘子电信
	region, _ := vectorSe.SearchByStr("2.12.133.0")
	assert.Equal(t, "法国|0|Ille-et-Vilaine|0|橘子电信", region, "set error")
}

func TestMemorySearcher(t *testing.T) {
	memorySe, _ := Create(defaultDbPathForTestOnly, CACHE_POLICY_MEMORY)
	// 2.12.133.0|2.12.139.255|法国|0|Ille-et-Vilaine|0|橘子电信
	region, _ := memorySe.SearchByStr("2.12.133.0")
	assert.Equal(t, "法国|0|Ille-et-Vilaine|0|橘子电信", region, "set error")
}
