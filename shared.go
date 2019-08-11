package go_virtual_host

import (
	"bytes"
	"io"
	"net"
	"sync"
)

const initVbuffSize = 1024

//为了提前从conn中读取一些字节来获取相关信息但是使外部使用起来就像没有消耗过任何
//字节一样.需要将从conn中消耗的字节暂时缓存起来并自己覆盖conn的Read方法使之先从
//自身的buffer中读取,然后再从conn中读取

type sharedConn struct{
	mu sync.Mutex
	net.Conn
	vbuff *bytes.Buffer
}

func (s *sharedConn) Read(b []byte) (n int, err error){
    s.mu.Lock()
    defer s.mu.Unlock()

    //vbuff中字节消耗完了则释放内存
    if s.vbuff == nil{
    	return s.Conn.Read(b)
	}

    n, err = s.vbuff.Read(b)

    //1.err == nil
    //2.err == EOF
    //3.err != nil && err != EOF
    //对于1、3都不需要从Conn中继续读,直接返回即可
    //对于2则将vbuff设置为nil
    if err == io.EOF{
    	err = nil
    	s.vbuff = nil
	}//if

	return
}

//sharedConn提供给外部正常使用
//返回一个reader,从中读取需要peek的字节,并将读取的字节放入sharedConn的vbuff中
//用teeReader即可

func newSharedConn(conn net.Conn) (sc *sharedConn, reader io.Reader){
	sc = &sharedConn{
		Conn: conn,
		vbuff: bytes.NewBuffer(make([]byte, 0, initVbuffSize)),
	}

	return sc, io.TeeReader(conn, sc.vbuff)
}