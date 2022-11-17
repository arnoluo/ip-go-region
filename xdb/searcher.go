// Copyright 2022 The Ip2Region Authors. All rights reserved.
// Use of this source code is governed by a Apache2.0-style
// license that can be found in the LICENSE file.

// ---
// ip2region database v2.0 searcher.
// @Note this is a Not thread safe implementation.
//
// @Author Lion <chenxin619315@gmail.com>
// @Date   2022/06/16

package xdb

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

const (
	VersionNo            = 2
	HeaderInfoLength     = 256
	VectorIndexRows      = 256
	VectorIndexCols      = 256
	VectorIndexSize      = 8
	VectorIndexLength    = VectorIndexRows * VectorIndexCols * VectorIndexSize
	RegionIndexBlockSize = 8
)

const (
	IP_TAIL_PATTERN = 0xFFFF
	// bytes
	REGION_BASE_BLOCK_SIZE = 64
	REGION_STR_SEP         = "|"
	// 地域信息串中信息位长度 2B for headOffset, 1B for tail len
	REGION_BLOCK_INFO_SIZE = 3
)

type CachePolicy int

const (
	CACHE_POLICY_FILE CachePolicy = iota
	CACHE_POLICY_VECTOR
	CACHE_POLICY_MEMORY
)

// --- Index policy define

type IndexPolicy int

const (
	VectorIndexPolicy IndexPolicy = 1
	BTreeIndexPolicy  IndexPolicy = 2
)

func (i IndexPolicy) String() string {
	switch i {
	case VectorIndexPolicy:
		return "VectorIndex"
	case BTreeIndexPolicy:
		return "BtreeIndex"
	default:
		return "unknown"
	}
}

// --- Header define

type Header struct {
	// data []byte
	Version            uint16
	IndexPolicy        IndexPolicy
	CreatedAt          uint32
	StartIndexPtr      uint32
	EndIndexPtr        uint32
	RegionHeadStartPtr uint32
}

func NewHeader(input []byte) (*Header, error) {
	if len(input) < 16 {
		return nil, fmt.Errorf("invalid input buffer")
	}

	return &Header{
		Version:            binary.LittleEndian.Uint16(input),
		IndexPolicy:        IndexPolicy(binary.LittleEndian.Uint16(input[2:])),
		CreatedAt:          binary.LittleEndian.Uint32(input[4:]),
		StartIndexPtr:      binary.LittleEndian.Uint32(input[8:]),
		EndIndexPtr:        binary.LittleEndian.Uint32(input[12:]),
		RegionHeadStartPtr: binary.LittleEndian.Uint32(input[16:]),
	}, nil
}

// --- searcher implementation

type Searcher struct {
	handle *os.File

	// header info
	header  *Header
	ioCount int

	// use it only when this feature enabled.
	// Preload the vector index will reduce the number of IO operations
	// thus speedup the search process
	vectorIndex []byte

	// content buffer.
	// running with the whole xdb file cached
	contentBuff []byte

	// enable full search or just find (64-3)B tail string
	searchMode bool
}

func (s *Searcher) SetSearchMode(isFullSearch bool) *Searcher {
	s.searchMode = isFullSearch
	return s
}

func (s *Searcher) GetSearchMode() bool {
	return s.searchMode
}

func (s *Searcher) LoadVectorIndex() error {
	// loaded already
	if s.vectorIndex != nil {
		return nil
	}

	// load all the vector index block
	_, err := s.handle.Seek(HeaderInfoLength, 0)
	if err != nil {
		return fmt.Errorf("seek to vector index: %w", err)
	}

	var buff = make([]byte, VectorIndexLength)
	rLen, err := s.handle.Read(buff)
	if err != nil {
		return err
	}

	if rLen != len(buff) {
		return fmt.Errorf("incomplete read: readed bytes should be %d", len(buff))
	}

	s.vectorIndex = buff
	return nil
}

// ClearVectorIndex clear preloaded vector index cache
func (s *Searcher) ClearVectorIndex() {
	s.vectorIndex = nil
}

func Create(dbPath string, cachePolicy CachePolicy) (*Searcher, error) {
	switch cachePolicy {
	case CACHE_POLICY_FILE:
		return NewWithFileOnly(dbPath)
	case CACHE_POLICY_VECTOR:
		vIndex, err := LoadVectorIndexFromFile(dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load vector index from `%s`: %w", dbPath, err)
		}

		return NewWithVectorIndex(dbPath, vIndex)
	case CACHE_POLICY_MEMORY:
		cBuff, err := LoadContentFromFile(dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load content from '%s': %w", dbPath, err)
		}

		return NewWithBuffer(cBuff)
	default:
		return nil, fmt.Errorf("invalid cache policy `%d`", cachePolicy)
	}
}

func baseNew(dbFile string, vIndex []byte, cBuff []byte) (*Searcher, error) {
	var err error

	// content buff first
	if cBuff != nil {
		header, err := LoadHeaderFromBuff(cBuff)
		if err != nil {
			return nil, err
		}
		return &Searcher{
			vectorIndex: nil,
			header:      header,
			contentBuff: cBuff,
			searchMode:  true,
		}, nil
	}

	// open the xdb binary file
	handle, err := os.OpenFile(dbFile, os.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}

	header, err := LoadHeader(handle)
	if err != nil {
		return nil, err
	}

	return &Searcher{
		handle:      handle,
		header:      header,
		searchMode:  true,
		vectorIndex: vIndex,
	}, nil
}

func NewWithFileOnly(dbFile string) (*Searcher, error) {
	return baseNew(dbFile, nil, nil)
}

func NewWithVectorIndex(dbFile string, vIndex []byte) (*Searcher, error) {
	return baseNew(dbFile, vIndex, nil)
}

func NewWithBuffer(cBuff []byte) (*Searcher, error) {
	return baseNew("", nil, cBuff)
}

func (s *Searcher) Close() {
	if s.handle != nil {
		err := s.handle.Close()
		if err != nil {
			return
		}
	}
}

// GetIOCount return the global io count for the last search
func (s *Searcher) GetIOCount() int {
	return s.ioCount
}

// SearchByStr find the region for the specified ip string
func (s *Searcher) SearchByStr(str string) (string, error) {
	ip, err := CheckIP(str)
	if err != nil {
		return "", err
	}

	return s.Search(ip)
}

// Search find the region for the specified long ip
func (s *Searcher) Search(ip uint32) (string, error) {
	// reset the global ioCount
	s.ioCount = 0

	// locate the segment index block based on the vector index
	var il0 = (ip >> 24) & 0xFF
	var il1 = (ip >> 16) & 0xFF
	var idx = il0*VectorIndexCols*VectorIndexSize + il1*VectorIndexSize
	var ipTail = uint16(ip & IP_TAIL_PATTERN)
	var sPtr, ePtr = uint32(0), uint32(0)
	if s.vectorIndex != nil {
		sPtr = binary.LittleEndian.Uint32(s.vectorIndex[idx:])
		ePtr = binary.LittleEndian.Uint32(s.vectorIndex[idx+4:])
	} else if s.contentBuff != nil {
		sPtr = binary.LittleEndian.Uint32(s.contentBuff[HeaderInfoLength+idx:])
		ePtr = binary.LittleEndian.Uint32(s.contentBuff[HeaderInfoLength+idx+4:])
	} else {
		// read the vector index block
		var buff = make([]byte, VectorIndexSize)
		err := s.read(int64(HeaderInfoLength+idx), buff)
		if err != nil {
			return "", fmt.Errorf("read vector index block at %d: %w", HeaderInfoLength+idx, err)
		}

		sPtr = binary.LittleEndian.Uint32(buff)
		ePtr = binary.LittleEndian.Uint32(buff[4:])
	}

	// fmt.Printf("sPtr=%d, ePtr=%d", sPtr, ePtr)

	// binary search the segment index to get the region
	var regionPtr int64
	var buff = make([]byte, RegionIndexBlockSize)
	var l, h = 0, int((ePtr - sPtr) / RegionIndexBlockSize)
	for l <= h {
		m := (l + h) >> 1
		p := sPtr + uint32(m*RegionIndexBlockSize)
		err := s.read(int64(p), buff)
		if err != nil {
			return "", fmt.Errorf("read segment index at %d: %w", p, err)
		}

		// decode the data step by step to reduce the unnecessary operations
		sip := binary.LittleEndian.Uint16(buff)
		if ipTail < sip {
			h = m - 1
		} else {
			eip := binary.LittleEndian.Uint16(buff[2:])
			if ipTail > eip {
				l = m + 1
			} else {
				// regionHeadOffset = int64(binary.LittleEndian.Uint16(buff[4:]))
				regionPtr = int64(binary.LittleEndian.Uint32(buff[4:]))
				break
			}
		}
	}

	var regionBuff = make([]byte, REGION_BASE_BLOCK_SIZE+REGION_BLOCK_INFO_SIZE)
	err := s.read(regionPtr, regionBuff)
	if err != nil {
		return "", fmt.Errorf("read region tail data at %d: %w", regionPtr, err)
	}

	regionHeadOffset := int64(binary.LittleEndian.Uint16(regionBuff[:2]))
	// get region head string
	var regionHeadBuff = make([]byte, REGION_BASE_BLOCK_SIZE)
	err = s.read(int64(s.header.RegionHeadStartPtr)+regionHeadOffset, regionHeadBuff)
	if err != nil {
		return "", fmt.Errorf("read region head data at %d: %w", int64(s.header.RegionHeadStartPtr)+regionHeadOffset, err)
	}
	// return string(regionHeadBuff[18:]), nil

	regionHeadBuff, _ = ParseDynamicBytes(regionHeadBuff)

	// first byte is lenth of the string behind
	regionTailBuff, missingLen := ParseDynamicBytes(regionBuff[2:])
	if s.searchMode && missingLen > 0 {
		var missingTailBuff = make([]byte, missingLen)
		err = s.read(regionPtr+REGION_BASE_BLOCK_SIZE+REGION_BLOCK_INFO_SIZE, missingTailBuff)
		if err != nil {
			return "", fmt.Errorf("read region tail missing data at %d: %w", regionPtr+REGION_BASE_BLOCK_SIZE, err)
		}
		regionTailBuff = append(regionTailBuff, missingTailBuff...)
	}

	return strings.Join([]string{
		string(regionHeadBuff),
		string(regionTailBuff),
	}, REGION_STR_SEP), nil

	//fmt.Printf("dataLen: %d, dataPtr: %d", dataLen, dataPtr)

	// load and return the region data
	// var regionBuff = make([]byte, dataLen)
	// err := s.read(int64(dataPtr), regionBuff)
	// if err != nil {
	// 	return "", fmt.Errorf("read region at %d: %w", dataPtr, err)
	// }

	// return string(regionBuff), nil
}

// do the data read operation based on the setting.
// content buffer first or will read from the file.
// this operation will invoke the Seek for file based read.
func (s *Searcher) read(offset int64, buff []byte) error {
	if s.contentBuff != nil {
		cLen := copy(buff, s.contentBuff[offset:])
		if cLen != len(buff) {
			return fmt.Errorf("incomplete read: readed bytes should be %d", len(buff))
		}
	} else {
		_, err := s.handle.Seek(offset, 0)
		if err != nil {
			return fmt.Errorf("seek to %d: %w", offset, err)
		}

		s.ioCount++
		rLen, err := s.handle.Read(buff)
		if err != nil {
			return fmt.Errorf("handle read: %w", err)
		}

		if rLen != len(buff) {
			return fmt.Errorf("incomplete read: readed bytes should be %d", len(buff))
		}
	}

	return nil
}
