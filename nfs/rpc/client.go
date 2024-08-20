// Copyright © 2017 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: BSD-2-Clause
package rpc

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	"github.com/vmware/go-nfs-client/nfs/util"
	"github.com/vmware/go-nfs-client/nfs/xdr"
)

const (
	MsgAccepted = iota
	MsgDenied
)

const (
	Success = iota
	ProgUnavail
	ProgMismatch
	ProcUnavail
	GarbageArgs
	SystemErr
)

const (
	RpcMismatch = iota
)

var xid uint32

func init() {
	// seed the XID (which is set by the client)
	xid = rand.New(rand.NewSource(time.Now().UnixNano())).Uint32()
}

type Client struct {
	*tcpTransport
}

func DialTCP(network string, ldr *net.TCPAddr, addr string) (*Client, error) {
	a, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTCP(a.Network(), ldr, a)
	if err != nil {
		return nil, err
	}

	t := &tcpTransport{
		r:  bufio.NewReader(conn),
		wc: conn,
	}

	return &Client{t}, nil
}

type message struct {
	Xid     uint32
	Msgtype uint32
	Body    interface{}
}

func (c *Client) Call(call interface{}) (io.ReadSeeker, error) {
	retries := 1

	msg := &message{
		Xid:  atomic.AddUint32(&xid, 1),
		Body: call,
	}

retry:
	w := new(bytes.Buffer)
	if err := xdr.Write(w, msg); err != nil {
		return nil, err
	}

	if _, err := c.Write(w.Bytes()); err != nil {
		return nil, err
	}

	res, err := c.recv()
	if err != nil {
		return nil, err
	}

	xid, err := xdr.ReadUint32(res)
	if err != nil {
		return nil, err
	}

	if xid != msg.Xid {
		return nil, fmt.Errorf("xid did not match, expected: %x, received: %x", msg.Xid, xid)
	}

	mtype, err := xdr.ReadUint32(res)
	if err != nil {
		return nil, err
	}

	if mtype != 1 {
		return nil, fmt.Errorf("message as not a reply: %d", mtype)
	}

	status, err := xdr.ReadUint32(res)
	if err != nil {
		return nil, err
	}

	switch status {
	case MsgAccepted:

		// padding
		_, err = xdr.ReadUint32(res)
		if err != nil {
			panic(err.Error())
		}

		opaque_len, err := xdr.ReadUint32(res)
		if err != nil {
			panic(err.Error())
		}

		_, err = res.Seek(int64(opaque_len), io.SeekCurrent)
		if err != nil {
			panic(err.Error())
		}

		acceptStatus, _ := xdr.ReadUint32(res)

		switch acceptStatus {
		case Success:
			return res, nil
		case ProgUnavail:
			return nil, fmt.Errorf("rpc: PROG_UNAVAIL - server does not recognize the program number")
		case ProgMismatch:
			return nil, fmt.Errorf("rpc: PROG_MISMATCH - program version does not exist on the server")
		case ProcUnavail:
			return nil, fmt.Errorf("rpc: PROC_UNAVAIL - unrecognized procedure number")
		case GarbageArgs:
			// emulate Linux behaviour for GARBAGE_ARGS
			if retries > 0 {
				util.Debugf("Retrying on GARBAGE_ARGS per linux semantics")
				retries--
				goto retry
			}

			return nil, fmt.Errorf("rpc: GARBAGE_ARGS - rpc arguments cannot be XDR decoded")
		case SystemErr:
			return nil, fmt.Errorf("rpc: SYSTEM_ERR - unknown error on server")
		default:
			return nil, fmt.Errorf("rpc: unknown accepted status error: %d", acceptStatus)
		}

	case MsgDenied:
		rejectStatus, _ := xdr.ReadUint32(res)
		switch rejectStatus {
		case RpcMismatch:

		default:
			return nil, fmt.Errorf("rejectedStatus was not valid: %d", rejectStatus)
		}

	default:
		return nil, fmt.Errorf("rejectedStatus was not valid: %d", status)
	}

	panic("unreachable")
}

func (c *Client) WriteCall(xid2 uint32, w2 *bytes.Buffer) (io.ReadSeeker, error) {
	retries := 1

	// msg := &message{
	// 	Xid:  atomic.AddUint32(util.GetXid(), 1),
	// 	Body: call,
	// }
	// fmt.Println("In client")
	// fmt.Println(msg)
	// retry:
	// w := new(bytes.Buffer)
	// if err := xdr.Write(w, msg); err != nil {
	// 	return nil, err
	// }
	// fmt.Println("w1")
	// fmt.Println([]byte(w.Bytes()))

	// fmt.Println("w2")
	// fmt.Println(w2)
	// w3, ok := w2.(bytes.Buffer)
	// if !ok {
	// 	fmt.Println("cant convert")
	// }
	// fmt.Println([]byte(w2.Bytes()))
	// fmt.Println(bytes.Compare(w.Bytes(), w2.Bytes()))

	// var times []time.Duration
	// start := time.Now()

	if _, err := c.Write(w2.Bytes()); err != nil {
		return nil, err
	}
	// times = append(times, time.Since(start))
	// fmt.Printf("Here %d\n", len(times))

retry:

	// start = time.Now()
	res, err := c.recv()
	if err != nil {
		return nil, err
	}
	// times = append(times, time.Since(start))
	// fmt.Printf("Here %d\n", len(times))

	// start = time.Now()
	dec := xdr.NewDecoder(res)

	xid, _, err := dec.DecodeUint()
	if err != nil {
		return nil, err
	}

	// xid, err := xdr.ReadUint32(res)
	// if err != nil {
	// 	return nil, err
	// }

	if xid != xid2 {
		return nil, fmt.Errorf("xid did not match, expected: %x, received: %x", xid2, xid)
	}

	mtype, _, err := dec.DecodeUint()
	if err != nil {
		return nil, err
	}

	// mtype, err := xdr.ReadUint32(res)
	// if err != nil {
	// 	return nil, err
	// }

	if mtype != 1 {
		return nil, fmt.Errorf("message as not a reply: %d", mtype)
	}

	status, _, err := dec.DecodeUint()
	if err != nil {
		return nil, err
	}

	// status, err := xdr.ReadUint32(res)
	// if err != nil {
	// 	return nil, err
	// }
	// times = append(times, time.Since(start))
	// fmt.Printf("Here %d\n", len(times))
	// start = time.Now()
	switch status {
	case MsgAccepted:

		// padding

		_, _, err := dec.DecodeUint()
		if err != nil {
			return nil, err
		}
		// _, err = xdr.ReadUint32(res)
		// if err != nil {
		// 	panic(err.Error())
		// }

		opaque_len, _, err := dec.DecodeUint()
		if err != nil {
			return nil, err
		}

		// opaque_len, err := xdr.ReadUint32(res)
		// if err != nil {
		// 	panic(err.Error())
		// }

		_, err = res.Seek(int64(opaque_len), io.SeekCurrent)
		if err != nil {
			panic(err.Error())
		}

		acceptStatus, _, _ := dec.DecodeUint()

		// acceptStatus, _ := xdr.ReadUint32(res)

		switch acceptStatus {
		case Success:
			// times = append(times, time.Since(start))
			// fmt.Printf("Here %d\n", len(times))
			return res, nil
		case ProgUnavail:
			return nil, fmt.Errorf("rpc: PROG_UNAVAIL - server does not recognize the program number")
		case ProgMismatch:
			return nil, fmt.Errorf("rpc: PROG_MISMATCH - program version does not exist on the server")
		case ProcUnavail:
			return nil, fmt.Errorf("rpc: PROC_UNAVAIL - unrecognized procedure number")
		case GarbageArgs:
			// emulate Linux behaviour for GARBAGE_ARGS
			if retries > 0 {
				fmt.Println("Garbage args")
				util.Debugf("Retrying on GARBAGE_ARGS per linux semantics")
				retries--
				goto retry
			}

			return nil, fmt.Errorf("rpc: GARBAGE_ARGS - rpc arguments cannot be XDR decoded")
		case SystemErr:
			return nil, fmt.Errorf("rpc: SYSTEM_ERR - unknown error on server")
		default:
			return nil, fmt.Errorf("rpc: unknown accepted status error: %d", acceptStatus)
		}

	case MsgDenied:
		rejectStatus, _, _ := dec.DecodeUint()
		// rejectStatus, _ := xdr.ReadUint32(res)
		switch rejectStatus {
		case RpcMismatch:

		default:
			return nil, fmt.Errorf("rejectedStatus was not valid: %d", rejectStatus)
		}

	default:
		return nil, fmt.Errorf("rejectedStatus was not valid: %d", status)
	}

	panic("unreachable")
}
