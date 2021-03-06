// Copyright (c) 2015 Uber Technologies, Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package tchannel

import (
	"encoding/binary"
	"fmt"
	"time"
)

const (
	_flagsIndex       = 0
	_ttlIndex         = 1
	_ttlLen           = 4
	_serviceLenIndex  = 1 /* flags */ + _ttlLen + 25 /* tracing */
	_serviceNameIndex = _serviceLenIndex + 1
)

type lazyCallReq struct {
	*Frame
}

func newLazyCallReq(f *Frame) lazyCallReq {
	if msgType := f.Header.messageType; msgType != messageTypeCallReq {
		panic(fmt.Errorf("newLazyCallReq called for wrong messageType: %v", msgType))
	}
	return lazyCallReq{f}
}

// Service returns the name of the destination service for this callReq.
func (f lazyCallReq) Service() string {
	l := f.Payload[_serviceLenIndex]
	return string(f.Payload[_serviceNameIndex : _serviceNameIndex+l])
}

// Method returns the name of the method being called. It panics if called for
// a non-callReq frame.
func (f lazyCallReq) Method() string {
	serviceLen := f.Payload[_serviceLenIndex]

	// nh:1 (hk~1 hv~1){nh}
	headerStart := _serviceLenIndex + 1 /* length byte */ + serviceLen
	numHeaders := int(f.Payload[headerStart])
	cur := int(headerStart) + 1
	for i := 0; i < numHeaders*2; i++ {
		sLen := f.Payload[cur]
		cur += 1 + int(sLen)
	}

	// csumtype:1 (csum:4){0,1} arg1~2 arg2~2 arg3~2
	checkSumType := ChecksumType(f.Payload[cur])
	cur += 1 /* checksum */ + checkSumType.ChecksumSize()

	// arg1~2
	arg1Len := int(binary.BigEndian.Uint16(f.Payload[cur : cur+2]))
	cur += 2
	arg1 := f.Payload[cur : cur+arg1Len]
	return string(arg1)
}

// TTL returns the time to live for this callReq.
func (f lazyCallReq) TTL() time.Duration {
	ttl := binary.BigEndian.Uint32(f.Payload[_ttlIndex : _ttlIndex+_ttlLen])
	return time.Duration(ttl) * time.Millisecond
}

// finishesCall checks whether this frame is the last one we should expect for
// this RPC req-res.
func finishesCall(f *Frame) bool {
	switch f.messageType() {
	case messageTypeError:
		return true
	case messageTypeCallRes, messageTypeCallResContinue:
		flags := f.Payload[_flagsIndex]
		return flags&hasMoreFragmentsFlag == 0
	default:
		return false
	}
}
