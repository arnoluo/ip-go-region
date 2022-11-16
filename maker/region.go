// Copyright 2022 The Ip2Region Authors. All rights reserved.
// Use of this source code is governed by a Apache2.0-style
// license that can be found in the LICENSE file.

// ----

// btree entry structure
// +--------------------+-------------------+-------------------+-------------------+--------------+
// | 		2bytes		|		2bytes		| 		2bytes		| 		2bytes		| 	4 bytes    |
// +--------------------+-------------------+-------------------+-------------------+--------------+
//  iphead		 		iptail2 start ip	iptail2 end ip	  	region head offset 	region tail ptr

// country+province
// start ptr(4Bytes) saved in header[16:]
// cell size 32B/64B
// end of str with 0b00000000
// region head structure:
// +------------------------+
// |		32/64 bytes 	|
// +------------------------+
//  country|zone|province

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/arnoluo/ip-go-region/xdb"
)

// end of str
const REGION_EOS_BYTE_NUM = 1

type Region struct {
	startPtr  uint32
	bodyMap   map[string]*regionBody
	totalTail int
	totalBody int
}

type regionBody struct {
	head       []byte
	headOffset uint16
	tailPtrMap map[string]uint32
}

func (r *Region) headAndTail(region string) (string, string) {
	pieces := strings.SplitN(region, xdb.REGION_STR_SEP, 4)
	// country := pieces[0]
	// province := pieces[2]
	tail := pieces[3]
	return strings.Join(pieces[:3], xdb.REGION_STR_SEP), tail
}

func (r *Region) seed(region string) error {
	headStr, tailStr := r.headAndTail(region)
	body, has := r.bodyMap[headStr]

	// 尾部字节，需小于 255B
	tailBytes := []byte(tailStr)
	if len(tailBytes) > 0xFF {
		return fmt.Errorf("too long region info `%s`: should be less than %d bytes", tailStr, 0xFF)
	}

	if has {
		_, hasKey := body.tailPtrMap[tailStr]
		if !hasKey {
			body.tailPtrMap[tailStr] = 0
		}
	} else {
		headBytes := []byte(headStr)
		headLen := len(headBytes)
		// 头部字节需小于 64B
		if headLen >= xdb.REGION_BASE_BLOCK_SIZE {
			return fmt.Errorf("too long region info `%s`: should be less than %d bytes", headStr, xdb.REGION_BASE_BLOCK_SIZE)
		}
		var headByteAll = make([]byte, 1)
		headByteAll[0] = uint8(headLen)
		headByteAll = append(headByteAll, headBytes...)

		r.bodyMap[headStr] = &regionBody{
			head:       headByteAll,
			headOffset: 0,
			tailPtrMap: map[string]uint32{
				tailStr: 0,
			},
		}
	}

	return nil
}

func (r *Region) write(dstHandle *os.File) error {
	pos, err := dstHandle.Seek(0, 1)
	if err != nil {
		return fmt.Errorf("seek to current ptr: %w", err)
	}
	r.startPtr = uint32(pos)
	log.Println(r.startPtr)
	var offset uint16
	for region, body := range r.bodyMap {
		body.headOffset = offset
		writedByteNum, err := dstHandle.Write(body.head)
		if err != nil {
			return fmt.Errorf("write region '%s': %w", region, err)
		}
		log.Printf("|||Test %d, %d, %d, %s\n", body.headOffset, writedByteNum, uint8(body.head[0]), string(body.head[1:]))

		offset += uint16(writedByteNum)
		if offset > 0xffff {
			return fmt.Errorf("seek head offset overflowed: %d", offset)
		}
		// pos, err = dstHandle.Seek(0, 1)
	}
	r.totalBody = len(r.bodyMap)

	for _, body := range r.bodyMap {
		for tailStr := range body.tailPtrMap {
			tailPos, err := dstHandle.Seek(0, 1)
			// _, err = dstHandle.Write(tailBody)
			if err != nil {
				return fmt.Errorf("write tail pos '%d': %w", pos, err)
			}

			var tailAll = make([]byte, 1)
			tailAll[0] = uint8(len([]byte(tailStr)))
			tailAll = append(tailAll, []byte(tailStr)...)

			_, err = dstHandle.Write(tailAll)
			if err != nil {
				return fmt.Errorf("write tail string'%s': %w", tailStr, err)
			}

			body.tailPtrMap[tailStr] = uint32(tailPos)
			log.Printf(" --[Added] with ptr=%d", pos)
		}
		r.totalTail += len(body.tailPtrMap)
	}

	return nil
}
