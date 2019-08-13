package go_virtual_host

import (
	"bytes"
	"io"
	"net"
	"strconv"
)

const (
	maxReadBlock = 1024
)

type HttpConn struct {
	*sharedConn

	Request *Request
}

func HTTP(conn net.Conn) (* HttpConn, error){
    sc, tee := newSharedConn(conn)

    var err error
    request, err := ReadRequest(tee)

    if err != nil{
    	return nil, err
	}

	return &HttpConn{sharedConn:sc, Request:request}, nil
}


func (hc *HttpConn) URI() string{
	return hc.Request.Header("Host")
}

func (hc *HttpConn) Free(){
	hc.Request = nil
}


type HttpCrack struct {
	bodyLen int
	readErr error
	operation int
	txReader *TextReader
	vbuff *bytes.Buffer
	net.Conn
	requestHandler func (*Request) *Request
}


func HTTPCRACK(conn net.Conn) *HttpCrack{
	return &HttpCrack{
		Conn:conn,
		txReader:NewTextReader(conn),
		vbuff:bytes.NewBuffer(make([]byte, 0, 1024)),
	}
}

func (hCrack *HttpCrack) SetRequestHandler(handler func(*Request) *Request){
	hCrack.requestHandler = handler
}


func (hCrack *HttpCrack) Read(p []byte) (n int, err error){
	//先从缓冲区中读取
	//再去读取request信息并更改存入缓冲区
	n, err = hCrack.vbuff.Read(p)

	if err != io.EOF{
		return
	}

	//err == io.EOF
    if hCrack.readErr != nil{
    	return
	}//if

	hCrack.readRequest()

	return hCrack.readBuffer(p)
}


func (hCrack *HttpCrack) readBuffer(p []byte) (n int, err error){
    n, err = hCrack.vbuff.Read(p)
    if err == io.EOF && hCrack.readErr == nil{
        err = nil
	}
    return n, err
}


func (hCrack *HttpCrack) readFullRequest(){
	//read body
	if hCrack.bodyLen > 0{
		hCrack.readRequestBody()
	}

	//read header
	hCrack.readRequest()
}


func (hCrack *HttpCrack) readRequest(){
	if hCrack.readErr != nil{
		return
	}//if

	//不能修改Content-Length
    request, err := hCrack.txReader.ReadRequest()

    if err != nil{
    	hCrack.readErr = err
		return
	}

    contentLen := request.ContentLength
    //将新的request写入到buff中

    if hCrack.requestHandler != nil{
    	request = hCrack.requestHandler(request)

    	if contentLen >= 0{
    		hCrack.bodyLen = contentLen
    		request.SetHeader("Content-Length", strconv.Itoa(contentLen))
		}

	}//if
	_, err = WriteRequest(request, hCrack.vbuff)
	if err != nil{
		hCrack.readErr = err
		return
	}//if
	return
}

func (hCrack *HttpCrack) readRequestBody() {
	var line []byte
	var err error

	if hCrack.bodyLen < maxReadBlock{
		line, err = hCrack.txReader.ReadUntilN(hCrack.bodyLen)
		hCrack.bodyLen = 0
	}else{
		line, err = hCrack.txReader.ReadUntilN(maxReadBlock)
		hCrack.bodyLen -= maxReadBlock
	}


    if err != nil{
    	hCrack.readErr = err
    	return
	}//if

	_, err = hCrack.vbuff.Write(line)

	if err != nil{
		hCrack.readErr = err
		return
	}//if

	return
}