package go_virtual_host

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var unexpectHttpMsg = errors.New("unexpected http message")

type Request struct {
	Method string

	URI string

	Version string

	header map[string] string

	//-1表示不存在
	ContentLength int

}

func (r *Request) Header(key string) string{
    if r.header == nil{
    	return ""
	}

    return r.header[key]
}

func (r *Request) SetHeader(key string, value string){
	if r.header == nil{
		r.header = map[string]string{key: value}
	}

	r.header[key] = value
}

//解析请求行
//返回Method,URI,Version

func parseRequestLine(requestLine string)(Method string, URI string, Version string, success bool){
	infos := strings.Split(requestLine, " ")

	if len(infos) != 3{
		return
	}

	Method = infos[0]
	URI = infos[1]
	Version = infos[2]
	success = true
	return
}


//解析请求头
func parseHeader(headerLine string) (key string ,value string, success bool){
	location := strings.Index(headerLine, ": ")
	if location == -1{
		return
	}

	key = headerLine[: location]
	value = headerLine[location+2: ]
	success = true
	return
}

//读取http协议
type TextReader struct {
	br *bufio.Reader
}

func NewTextReader(reader io.Reader) *TextReader{
	return &TextReader{br: bufio.NewReader(reader)}
}


//直到读取到delimiter或者error才返回
func (tr *TextReader) readBytes(delimiter byte) (line []byte, err error){
	var readline []byte

	for {
		readline, err = tr.br.ReadBytes(delimiter)
		line = append(line, readline...)
		if len(line) > 0 && line[len(line)-1] == delimiter{
			return line, nil
		}
		if err != nil{
			line = nil
			readline = nil
			return
		}
	}//for
}


//一直读到\n字符并返回不带\n字符的字符串
//如果为\r\n则\r字符也不返回
func (tr *TextReader) Readline() (string, error){
	//read util \n or error

	p, err := tr.readBytes(byte('\n'))

	if err != nil{
		return "", err
	}

	p = p[: len(p) - 1]

	//discard \r after \n
	if len(p) > 0 && p[len(p) - 1] == byte('\r'){
		p = p[: len(p) - 1]
	}

	return string(p), nil
}


func (tr *TextReader) ReadBytes(delimiter byte) (line []byte, err error){
	return tr.readBytes(delimiter)
}

//一直读到n个字节
func (tr *TextReader) ReadUntilN(until int) (line []byte, err error){
	line = make([]byte, until)
	var (
		n int
		read int
	)

	for {
		n, err = tr.br.Read(line[read: ])
		read += n
		if read == until{
			return line, nil
		}
		if err != nil{return }

	}//for

}

func (tr *TextReader) ReadRequest() (request *Request, err error){
	request = new(Request)
	//先读解析行
	var line string
	if line, err = tr.Readline(); err != nil{
		return
	}
	var success bool
	if request.Method, request.URI, request.Version, success = parseRequestLine(line); !success{
		fmt.Println("read request line error", err)
		err = unexpectHttpMsg
		return
	}

	//读header
	request.header = make(map[string] string)
	var (
		key string
		value string
	)

	for {
		line, err = tr.Readline()
		if err != nil{
			return
		}//if

		if line == ""{
			break
		}//if

		if key, value, success = parseHeader(line); ! success{
			err = unexpectHttpMsg
			return
		}//if

		request.header[key] = value
	}//for

    contentLenStr, ok := request.header["Content-Length"]
    if !ok{
    	request.ContentLength = -1
    	return
	}

    iNt64, err := strconv.ParseInt(contentLenStr, 10, 32)

    if err != nil{
    	err = unexpectHttpMsg
    	return
	}//if

    request.ContentLength = int(iNt64)
	return request, err
}


func ReadRequest(reader io.Reader) (*Request, error){
	tr := NewTextReader(reader)
	return tr.ReadRequest()
}


func WriteRequest(request *Request, writer io.Writer) (int, error){
    //先写请求行
	switch w := writer.(type) {
	case *bytes.Buffer:
		return writeRequestIntoBytesBuffer(w, request)
	}
	//
    bytesStr := fmt.Sprintf("%s %s %s\r\n", request.Method, request.URI, request.Version)

    //写header

    for key, value := range request.header{
    	bytesStr = fmt.Sprintf("%s%s: %s\r\n", bytesStr, key, value)
	}//for

	//\r\n
	bytesStr = fmt.Sprintf("%s\r\n", bytesStr)

	line := []byte(bytesStr)

	return writer.Write(line)
}


func writeRequestIntoBytesBuffer(b *bytes.Buffer, request *Request) (n int, err error){
	write := 0

	requestLine := fmt.Sprintf("%s %s %s\r\n", request.Method, request.URI, request.Version)

	if write, err = b.WriteString(requestLine); err != nil{
		return
	}

	n += write

	for key, value := range request.header{
		if write, err = b.WriteString(fmt.Sprintf("%s: %s\r\n", key, value)); err != nil{
			return
		}

		n += write
	}//for

	if write, err = b.WriteString("\r\n"); err != nil{
		return
	}

	n += write
	return

}