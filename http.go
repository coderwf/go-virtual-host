package go_virtual_host

import (
	"bytes"
	"io"
	"net"
	"strconv"
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
	if hCrack.vbuff.Len() > 0{
		return hCrack.readBuffer(p)
	}//if

    err = hCrack.readFullRequest()

    if err != nil{
    	return
	}

	return hCrack.readBuffer(p)
}


func (hCrack *HttpCrack) readBuffer(p []byte) (n int, err error){
    n, err = hCrack.vbuff.Read(p)
    if err == io.EOF && hCrack.readErr == nil{
        err = nil
	}
    return n, err
}


func (hCrack *HttpCrack) readFullRequest() (err error){
	if hCrack.bodyLen > 0{
		return hCrack.readRequestBody()
	}

	return hCrack.readRequest()
}


func (hCrack *HttpCrack) readRequest() (err error){
	//不能修改Content-Length
    request, err := hCrack.txReader.ReadRequest()

    if err != nil{return }

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
		return
	}//if
	return nil
}

func (hCrack *HttpCrack) readRequestBody() (err error){
	//todo 设置最大读取字节数
    line, err := hCrack.txReader.ReadUntilN(hCrack.bodyLen)

    if err != nil{
    	return
	}//if

	_, err = hCrack.vbuff.Write(line)

	if err != nil{
		return
	}//if

	hCrack.bodyLen = 0

	return nil
}