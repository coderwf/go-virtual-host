package main

import (
	"fmt"
	"go-virtual-host"
	"io"
	"net"
	"sync"
)


func handlerRequest(request *go_virtual_host.Request) *go_virtual_host.Request{
	fmt.Println(request.Method, request.Version, request.ContentLength, request.URI)
	request.URI = "/"
	return request
}


func handlerConn(conn net.Conn, target string){
	var wait sync.WaitGroup

	hCrack := go_virtual_host.HTTPCRACK(conn)
	hCrack.SetRequestHandler(handlerRequest)
	proxy := getProxy(target)

	pipe := func(c1 net.Conn, c2 net.Conn) {defer c1.Close(); defer wait.Done() ;io.Copy(c1, c2)}
	wait.Add(2)
	go pipe(hCrack, proxy)
	go pipe(proxy, hCrack)
	wait.Wait()
}

func getProxy(target string) net.Conn{
    conn, err := net.Dial("tcp", target)

    if err != nil{panic(err)}

    return conn
}

func main(){
	var err error
	address := "127.0.0.1:9999"
    target := "39.106.71.25:10000"
	server, err := net.Listen("tcp", address)
	if err != nil{panic(err)}

	conn, err := server.Accept()
	if err != nil{panic(err)}

	handlerConn(conn, target)

}
