package go_virtual_host

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

type converter interface {
	convert(net.Conn) (net.Conn, net.Conn, error)
}

type httpConverter struct {
    getProxy func(*Request) (net.Conn, error)
}

func (h *httpConverter) convert(conn net.Conn) (net.Conn, net.Conn, error){
    httpConn, err := HTTP(conn)
    if err != nil{
    	fmt.Printf("parset http request from %s error: %v", conn.RemoteAddr().String(), err)
    	return nil, nil, nil
	}

    var e error
    for i :=0;i < 5;i ++{
		proxy, e := h.getProxy(httpConn.Request)
		if e == nil{
			return httpConn, proxy, e
		}
	}
    return nil, nil, e
}

type crackConverter struct {
	getProxy func(*Request) (net.Conn, error)
	handlerRequest func(*Request) *Request
}

func (c *crackConverter) convert(conn net.Conn) (net.Conn, net.Conn, error){
	httpConn, err := HTTPCRACK(conn)

	if err != nil{
		fmt.Printf("parset http request from %s error: %v", conn.RemoteAddr().String(), err)
		return nil, nil, nil
	}

	httpConn.SetRequestHandler(c.handlerRequest)
	var e error
	for i :=0;i < 5;i ++{
		proxy, e := c.getProxy(httpConn.Request)
		if e == nil{
			return httpConn, proxy, e
		}
	}
	return nil, nil, e
}


type Proxy struct {
	net.Listener
	conns chan net.Conn
	err error
	name string
	converter
}

func (p *Proxy)logLn(format string ,values ...interface{}){
	format = fmt.Sprintf("[%s:%s]: %s\n", p.name, p.Addr().String(), format)
	fmt.Printf(format, values...)
}

func (p *Proxy) accept(){
	for {

		if p.err != nil{
			p.logLn("server %s stopped cause: %v", p.name, p.err)
			return
		}

		conn, err := p.Accept()
		if err != nil{
			p.logLn("accept conn error: %v", err)
			return
		}

		p.logLn("connection %s come", conn.RemoteAddr().String())
		p.conns <- conn
	}//for
}

func (p *Proxy) handle(conn net.Conn){
	defer func() {
		if r := recover(); r != nil{
			p.err = errors.New(fmt.Sprintf("%v", r))
		}
	}()
	//先获取proxy
	from, to, err := p.convert(conn)

	if err != nil{
		p.err = err
		return
	}

	var wait sync.WaitGroup
	pipe := func(from net.Conn, to net.Conn) {
		defer func() {
			var e error
			e = from.Close()
			e = to.Close()
			if e != nil{
				//do nothing
			}
		}()

		n, e := io.Copy(to, from)

		p.logLn("%d bytes copied from %s before broken, err: %v", n, from.RemoteAddr().String(), e)
	}
	wait.Add(2)
	wait.Add(2)

	p.logLn("Join conn %s and %s", from.RemoteAddr().String(), to.RemoteAddr().String())
	go pipe(from ,to)
	go pipe(to, from)

	wait.Wait()

}


func (p *Proxy) start(){
	p.logLn("start %s server", p.name)
	go p.accept()

	for conn := range p.conns{
		p.handle(conn)
	}//for
}


func (p *Proxy)Start(){
	p.start()
}


func (p *Proxy) AsyncStart(){
	go p.start()
}

func getListener(listen string) net.Listener{
	listener, err := net.Listen("tcp", listen)
	if err != nil{
		panic(err)
	}
	return listener
}

func NewCrackProxy(
	listen string,
	getProxy func(*Request) (net.Conn, error),
	handlerRequest func(*Request) *Request) Server{

	listener := getListener(listen)

	converter := crackConverter{getProxy:getProxy, handlerRequest:handlerRequest}
	crack := &Proxy{
		Listener:listener,
		conns:make(chan net.Conn, 15),
		name:"crack-proxy",
		converter:&converter,
	}

	return crack
}


func NewCommonProxy(listen string, getProxy func(*Request) (net.Conn, error)) Server{
	listener := getListener(listen)
	converter := httpConverter{getProxy:getProxy}

	proxy := &Proxy{
		Listener:listener,
		conns:make(chan net.Conn, 15),
		name:"http-proxy",
		converter:&converter,
	}

	return proxy
}

type Server interface {
	Start()
	AsyncStart()
}