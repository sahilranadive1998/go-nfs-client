// Copyright © 2017 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: BSD-2-Clause
package rpc

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"
)

type tcpTransport struct {
	r       io.Reader
	wc      net.Conn
	timeout time.Duration

	rlock, wlock sync.Mutex
}

// Get the response from the conn, buffer the contents, and return a reader to
// it.
func (t *tcpTransport) recv() (io.ReadSeeker, error) {
	t.rlock.Lock()
	defer t.rlock.Unlock()
	if t.timeout != 0 {
		deadline := time.Now().Add(t.timeout)
		t.wc.SetReadDeadline(deadline)
	}
	var hdr uint32
	if err := binary.Read(t.r, binary.BigEndian, &hdr); err != nil {
		return nil, err
	}
	buf := make([]byte, hdr&0x7fffffff)
	if _, err := io.ReadFull(t.r, buf); err != nil {
		return nil, err
	}
	return bytes.NewReader(buf), nil
}

// Get the response from the conn, buffer the contents, and return a reader to
// it.
// func (t *tcpTransport) writeRecv() (io.ReadSeeker, error) {

// 	start := time.Now()
// 	t.rlock.Lock()
// 	defer t.rlock.Unlock()
// 	if t.timeout != 0 {
// 		deadline := time.Now().Add(t.timeout)
// 		t.wc.SetReadDeadline(deadline)
// 	}
// 	t1 := time.Since(start)
// 	start2 := time.Now()

// 	bufHeader := make([]byte, 4)
// 	t.r.Read(bufHeader)
// 	// if _, err := io.ReadFull(t.r, bufHeader); err != nil {
// 	// 	return nil, err
// 	// }
// 	t2 := time.Since(start2)
// 	start3 := time.Now()
// 	// var hdr uint32
// 	// if err := binary.Read(t.r, binary.BigEndian, &hdr); err != nil {
// 	// 	return nil, err
// 	// }
// 	hdr := binary.BigEndian.Uint32(bufHeader)
// 	t3 := time.Since(start3)
// 	start4 := time.Now()
// 	buf := make([]byte, hdr&0x7fffffff)
// 	fmt.Println(len(buf))
// 	if _, err := io.ReadFull(t.r, buf); err != nil {
// 		return nil, err
// 	}
// 	t4 := time.Since(start4)
// 	fmt.Printf("t1: %v, t2: %v, t3: %v, t4: %v, length: %v, header: %d\n", t1, t2, t3, t4, len(buf), hdr)
// 	return bytes.NewReader(buf), nil
// }

// func (t *tcpTransport) recv() (io.ReadSeeker, error) {
// 	// Lock only the critical section
// 	t.rlock.Lock()
// 	if t.timeout != 0 {
// 		fmt.Printf("Timeout is not zero: %v\n", t.timeout)
// 		deadline := time.Now().Add(t.timeout)
// 		t.wc.SetReadDeadline(deadline)
// 	}
// 	t.rlock.Unlock()

// 	// Allocate hdr on the stack and read directly
// 	var hdr uint32
// 	if err := binary.Read(t.r, binary.BigEndian, &hdr); err != nil {
// 		return nil, err
// 	}

// 	// Use buffer pool to avoid allocations
// 	bufSize := int(hdr & 0x7fffffff)
// 	buf := t.getBuffer(bufSize)
// 	defer t.putBuffer(buf)

// 	if _, err := io.ReadFull(t.r, buf[:bufSize]); err != nil {
// 		return nil, err
// 	}

// 	return bytes.NewReader(buf[:bufSize]), nil
// }

// // Use a sync.Pool for buffer pooling
// var bufferPool = sync.Pool{
// 	New: func() interface{} {
// 		return make([]byte, 0, 1024*1024) // initial capacity, adjust as needed
// 	},
// }

// func (t *tcpTransport) getBuffer(size int) []byte {
// 	buf := bufferPool.Get().([]byte)
// 	if cap(buf) < size {
// 		buf = make([]byte, size)
// 	}
// 	return buf[:size]
// }

// func (t *tcpTransport) putBuffer(buf []byte) {
// 	bufferPool.Put(buf[:0]) // Reset buffer before putting it back in the pool
// }

func (t *tcpTransport) Write(buf []byte) (int, error) {
	t.wlock.Lock()
	defer t.wlock.Unlock()

	var hdr uint32 = uint32(len(buf)) | 0x80000000
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, hdr)
	if t.timeout != 0 {
		deadline := time.Now().Add(t.timeout)
		t.wc.SetWriteDeadline(deadline)
	}
	n, err := t.wc.Write(append(b, buf...))

	return n, err
}

func (t *tcpTransport) Close() error {
	return t.wc.Close()
}

func (t *tcpTransport) SetTimeout(d time.Duration) {
	t.timeout = d
	if d == 0 {
		var zeroTime time.Time
		t.wc.SetDeadline(zeroTime)
	}
}
