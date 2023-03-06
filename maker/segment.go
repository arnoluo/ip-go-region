// Copyright 2022 The Ip2Region Authors. All rights reserved.
// Use of this source code is governed by a Apache2.0-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"

	"github.com/arnoluo/ip-go-region/xdb"
)

type Segment struct {
	StartIP    uint32
	EndIP      uint32
	RegionHead string
	RegionTail string
}

func SegmentFrom(lineStr string) (seg *Segment, err error) {

	var ps = strings.SplitN(lineStr, "|", 3)
	if len(ps) != 3 {
		err = fmt.Errorf("invalid ip segment line `%s`", lineStr)
		return
	}

	sip, err := xdb.CheckIP(ps[0])
	if err != nil {
		return
	}

	eip, err := xdb.CheckIP(ps[1])
	if err != nil {
		return
	}

	if sip > eip {
		err = fmt.Errorf("start ip(%s) should not be greater than end ip(%s)", ps[0], ps[1])
		return
	}

	if len(ps[2]) < 1 {
		err = fmt.Errorf("empty region info in segment line `%s`", lineStr)
		return
	}

	regionHead, regionTail, err := headAndTail(ps[2])
	if err != nil {
		return
	}

	seg = &Segment{
		StartIP:    sip,
		EndIP:      eip,
		RegionHead: regionHead,
		RegionTail: regionTail,
	}
	return

	// return &Segment{
	// 	StartIP:    sip,
	// 	EndIP:      eip,
	// 	RegionHead: regionHead,
	// 	RegionTail: regionTail,
	// }, nil
}

// 为了符合vector索引逻辑，符合条件，且起止ip的后两段必须为 0.0，255.255 这样满区段的，才作为内部ip处理
func (s *Segment) IsReserved() bool {
	return s.StartIP&xdb.IP_TAIL_PATTERN == 0 && s.EndIP&xdb.IP_TAIL_PATTERN == xdb.IP_TAIL_PATTERN && strings.Index(s.RegionHead, RESERVED_HEAD_ADDR) == 0 && strings.Index(s.RegionTail, RESERVED_TAIL_ADDR) == 0

}

// Split the segment based on the pre-two bytes
func (s *Segment) Split() []*Segment {
	// 1, split the segment with the first byte
	var tList []*Segment
	var sByte1, eByte1 = (s.StartIP >> 24) & 0xFF, (s.EndIP >> 24) & 0xFF
	var nSip = s.StartIP
	for i := sByte1; i <= eByte1; i++ {
		sip := (i << 24) | (nSip & 0xFFFFFF)
		eip := (i << 24) | 0xFFFFFF
		if eip < s.EndIP {
			nSip = (i + 1) << 24
		} else {
			eip = s.EndIP
		}

		// append the new segment (maybe)
		tList = append(tList, &Segment{
			StartIP: sip,
			EndIP:   eip,
			// @Note: don't bother to copy the region
			/// Region: s.Region,
		})
	}

	// 2, split the segments with the second byte
	var segList []*Segment
	for _, seg := range tList {
		base := seg.StartIP & 0xFF000000
		nSip := seg.StartIP
		sb2, eb2 := (seg.StartIP>>16)&0xFF, (seg.EndIP>>16)&0xFF
		for i := sb2; i <= eb2; i++ {
			sip := base | (i << 16) | (nSip & 0xFFFF)
			eip := base | (i << 16) | 0xFFFF
			if eip < seg.EndIP {
				nSip = 0
			} else {
				eip = seg.EndIP
			}

			segList = append(segList, &Segment{
				StartIP:    sip,
				EndIP:      eip,
				RegionHead: s.RegionHead,
				RegionTail: s.RegionTail,
			})
		}
	}

	return segList
}

func (s *Segment) String() string {
	return strings.Join([]string{
		xdb.Long2IP(s.StartIP),
		xdb.Long2IP(s.EndIP),
		s.RegionHead,
		s.RegionTail,
	}, xdb.REGION_STR_SEP)
}

func (s *Segment) RegionStr() string {
	return strings.Join([]string{
		s.RegionHead,
		s.RegionTail,
	}, xdb.REGION_STR_SEP)
}
