package util

import (
	"io"
	"math/rand"
	"net"
	"sync"
	"time"
)

var Xid uint32

func Init() {
	// seed the XID (which is set by the client)
	Xid = rand.New(rand.NewSource(time.Now().UnixNano())).Uint32()
}

func GetXid() *uint32 {
	if Xid == 0 {
		Init()
	}
	return &Xid
}

var AuthNull Auth

type Auth struct {
	Flavor uint32
	Body   []byte
}

type Header struct {
	Rpcvers uint32
	Prog    uint32
	Vers    uint32
	Proc    uint32
	Cred    Auth
	Verf    Auth
}

type WriteArgs struct {
	Header
	FH     []byte
	Offset uint64
	Count  uint32

	// UNSTABLE(0), DATA_SYNC(1), FILE_SYNC(2) default
	How      uint32
	Contents []byte
}

type ReadlinkArgs struct {
	Header
	FH []byte
}

type ReadArgs struct {
	Header
	FH     []byte
	Offset uint64
	Count  uint32
}

type CommitArg struct {
	Header
	FH     []byte
	Offset uint64
	Count  uint32
}

type Mount struct {
	Header
	Dirpath string
}

type TcpTransport struct {
	r       io.Reader
	wc      net.Conn
	timeout time.Duration

	rlock, wlock sync.Mutex
}

type Client struct {
	*TcpTransport
}
