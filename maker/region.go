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
// last n Bytes for tail str(n < 255)
// block structure:
// +--------------------+-----------------------+-------------------+
// |		2 Bytes		|		1 Byte			|		n Bytes		|
// +--------------------+-----------------------+-------------------+
// 	head offset(<65535)	 tail byte num			tail string bytes(n < 255)
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
	tail := pieces[3]
	return strings.Join(pieces[:3], xdb.REGION_STR_SEP), tail
}

func (r *Region) seed(region string) error {
	headStr, tailStr := r.headAndTail(region)
	body, has := r.bodyMap[headStr]

	// tail Bytes <= 255
	tailBytes := []byte(tailStr)
	if len(tailBytes) > 0xFF {
		return fmt.Errorf("too long region tail info `%s`: should be less than %d bytes", tailStr, 0xFF)
	}

	if has {
		_, hasKey := body.tailPtrMap[tailStr]
		if !hasKey {
			body.tailPtrMap[tailStr] = 0
		}
	} else {
		headBytes := []byte(headStr)
		headLen := len(headBytes)
		// head Bytes < 64
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
	var offset uint16
	for region, body := range r.bodyMap {
		body.headOffset = offset
		writedByteNum, err := dstHandle.Write(body.head)
		if err != nil {
			return fmt.Errorf("write region '%s': %w", region, err)
		}

		offset += uint16(writedByteNum)
		if offset > 0xFFFF {
			return fmt.Errorf("seek head offset overflowed: %d", offset)
		}
	}
	r.totalBody = len(r.bodyMap)

	for _, body := range r.bodyMap {
		for tailStr := range body.tailPtrMap {
			tailPos, err := dstHandle.Seek(0, 1)
			// _, err = dstHandle.Write(tailBody)
			if err != nil {
				return fmt.Errorf("write tail pos '%d': %w", pos, err)
			}

			var tailAll = make([]byte, 3)
			binary.LittleEndian.PutUint16(tailAll, body.headOffset)
			tailAll[2] = uint8(len([]byte(tailStr)))
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
