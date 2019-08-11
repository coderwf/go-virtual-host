package go_virtual_host

import "net"

//tls记录层协议
//客户端发送clientHello消息到server
//从中解析出serverName


type TlsConn struct {
	*sharedConn
}

func TLS(conn net.Conn) (*TlsConn, error){
	return nil, nil
}

type ClientHello struct {

}