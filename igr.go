package goitr

import "goitr/xdb"

const DEFAULT_DB_PATH = "./data/igr.xdb"

func DefaultFileSearcher() (*xdb.Searcher, error) {
	return xdb.Create(DEFAULT_DB_PATH, xdb.CACHE_POLICY_FILE)
}

func DefaultVectorSearcher() (*xdb.Searcher, error) {
	return xdb.Create(DEFAULT_DB_PATH, xdb.CACHE_POLICY_VECTOR)
}

func DefaultMemorySearcher() (*xdb.Searcher, error) {
	return xdb.Create(DEFAULT_DB_PATH, xdb.CACHE_POLICY_MEMORY)
}
