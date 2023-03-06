// region structure
// regionHead(country|zone|province) + regionTail(city|isp)

// region head:
// the blocks start ptr(4Bytes) saved in header[16:]
// Marked head location with a 2B offset value in tail block
// block structure:
// +----------------+-------------------+
// |	1 Byte		|		n Bytes		|
// +----------------+-------------------+
// 	byte len(<64)		head string bytes
//

// region tail:
// first 2B for region head offset, 1 addition io to get head data
// then 1B for tail str byte num
// last n Bytes for tail str(n <= 255)
// block structure:
// +--------------------+-----------------------+-------------------+
// |		2 Bytes		|		1 Byte			|		n Bytes		|
// +--------------------+-----------------------+-------------------+
// 	head offset(<=65535)	 tail byte num			tail string bytes(n <= 255)
//

package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/arnoluo/ip-go-region/xdb"
)

type regionHeadPart = int

const (
	// 舍弃地址头部中的地域信息
	REGION_HEAD_TYPE_NO_AREA regionHeadPart = iota
	// 包含全部信息
	REGION_HEAD_TYPE_ALL
)

const REGION_HEAD_TYPE = REGION_HEAD_TYPE_NO_AREA
const RESERVED_TAIL_ADDR = "内网IP|内网IP"
const RESERVED_HEAD_ADDR = "0|0"

type region struct {
	startPtr uint32
	// head string => head info & tails
	treeMap         map[string]*regionTree
	totalTail       int
	totalTree       int
	reservedTailPtr uint32
}

type regionTree struct {
	headOffset uint16
	// tail string => tail info ptr
	tailPtrMap map[string]uint32
}

func headAndTail(region string) (head string, tail string, err error) {
	pieces := strings.SplitN(region, xdb.REGION_STR_SEP, 4)
	if REGION_HEAD_TYPE == REGION_HEAD_TYPE_ALL {
		head = strings.Join(pieces[:3], xdb.REGION_STR_SEP)
	} else {
		head = strings.Join([]string{
			pieces[0],
			pieces[2],
		}, xdb.REGION_STR_SEP)
	}

	tail = pieces[3]
	err = nil
	if err = checkRegionTail(head); err == nil {
		err = checkRegionTail(tail)
	}
	return
}

func checkRegionHead(regionHead string) (err error) {
	err = nil
	if len(regionHead) >= xdb.REGION_BASE_BLOCK_SIZE {
		err = fmt.Errorf("too long region info `%s`(%dB): should be less than %d bytes", regionHead, len(regionHead), xdb.REGION_BASE_BLOCK_SIZE)
	}
	return
}

func checkRegionTail(regionTail string) (err error) {
	err = nil
	if len(regionTail) > 0xFF {
		err = fmt.Errorf("too long region tail info `%s`(%dB): should be less than or equal %d bytes", regionTail, len(regionTail), 0xFF)
	}
	return
}

func (r *region) seed(headStr string, tailStr string) {
	// headStr, tailStr, err = headAndTail(region)
	// if err != nil {
	// 	return
	// }

	tree, has := r.treeMap[headStr]

	if has {
		_, hasKey := tree.tailPtrMap[tailStr]
		if !hasKey {
			tree.tailPtrMap[tailStr] = 0
		}
	} else {
		r.treeMap[headStr] = &regionTree{
			headOffset: 0,
			tailPtrMap: map[string]uint32{
				tailStr: 0,
			},
		}
	}
}

func (r *region) write(dstHandle *os.File) error {
	pos, err := dstHandle.Seek(0, 1)
	if err != nil {
		return fmt.Errorf("seek to current ptr: %s", err)
	}
	r.startPtr = uint32(pos)
	var offset uint16
	for regionHead, tree := range r.treeMap {
		tree.headOffset = offset
		var headByteAll = make([]byte, 1)
		headByteAll[0] = uint8(len(regionHead))
		headByteAll = append(headByteAll, []byte(regionHead)...)
		writedByteNum, err := dstHandle.Write(headByteAll)
		if err != nil {
			return fmt.Errorf("write region '%s': %w", regionHead, err)
		}

		offset += uint16(writedByteNum)
		if offset > 0xFFFF {
			return fmt.Errorf("seek head offset overflowed: %d", offset)
		}
	}
	r.totalTree = len(r.treeMap)

	for _, tree := range r.treeMap {
		for tailStr := range tree.tailPtrMap {
			tailPos, err := dstHandle.Seek(0, 1)
			// _, err = dstHandle.Write(tailBody)
			if err != nil {
				return fmt.Errorf("write tail pos '%d': %w", pos, err)
			}

			var tailAll = make([]byte, 3)
			binary.LittleEndian.PutUint16(tailAll, tree.headOffset)
			tailAll[2] = uint8(len([]byte(tailStr)))
			tailAll = append(tailAll, []byte(tailStr)...)

			_, err = dstHandle.Write(tailAll)
			if err != nil {
				return fmt.Errorf("write tail string'%s': %w", tailStr, err)
			}

			tree.tailPtrMap[tailStr] = uint32(tailPos)
			log.Printf(" --[Added] with ptr=%d", pos)
		}
		r.totalTail += len(tree.tailPtrMap)
	}

	reservedTree, has := r.treeMap[RESERVED_HEAD_ADDR]
	if has {
		reservedTail, hasTail := reservedTree.tailPtrMap[RESERVED_TAIL_ADDR]
		if hasTail {
			r.reservedTailPtr = reservedTail
		}
	}
	if r.reservedTailPtr == 0 {
		return fmt.Errorf("reserve region info not set")
	}

	return nil
}
