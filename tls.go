package go_virtual_host

import (
	"bufio"
	"fmt"
	"io"
	"net"
)

//tls记录层协议
//客户端发送clientHello消息到server
//从中解析出serverName

const (
	maxPlaintext    = 16384        // maximum plaintext payload length
	maxCiphertext   = 16384 + 2048 // maximum ciphertext payload length
	maxHandshake    = 65536        // maximum handshake we support (protocol max is 16 MB)
)

type alert int

const (
	alertRecordOverflow alert = 0
	alertInternalError alert = 1
	alertUnexpectedMsg alert = 2
	alertUnsupportTls alert = 3
)

var textMap = map[alert] string{
	alertRecordOverflow: "alert overflow",
	alertInternalError: "alert internal error",
	alertUnexpectedMsg: "alert unexpected message",
	alertUnsupportTls: "tls: unsupported tlsv2 message received",
}

func (a alert) Error() string{
	return a.String()
}

func (a alert) String() string{
	msg, ok := textMap[a]
	if !ok{
		return fmt.Sprintf("alert (%d)", a)
	}
	return msg
}


type TlsConn struct {
	*sharedConn
	clientHello *ClientHello
}


func (tc *TlsConn) Free(){
	tc.clientHello = nil
}

func (tc *TlsConn) Host() string{
    if tc.clientHello == nil{
    	return ""
	}
    return tc.clientHello.ServerName
}

func TLS(conn net.Conn) (tc *TlsConn, err error){
    sc, tee := newSharedConn(conn)

    clientHello, err := readClientHello(tee)
    if err != nil{
    	return nil, err
	}//if

	return &TlsConn{sharedConn: sc, clientHello:clientHello}, nil
}


//byte 转化为int
func uToInt(b []byte) int{
	num := 0

    for i:= 0; i < len(b); i++{
		num <<= 8
    	num |= int(b[i])
	}

    return num
}


type ClientHello struct {
	//handshake 22 1 bytes
    ContentType uint8

    //tls version 2 bytes
    Version int

    //content length 2 bytes (不包括这5个字节)
    Length int

    //以上共5个字节


    //以下为handshake内容

    //handshake typ 1 bytes
    //client hello 0x01
    HandshakeTyp uint8

    //handshake 长度 3 bytes
    //不包括这4个字节
    HandshakeLength int


    //handshake 版本 2 bytes
    HandshakeVersion int

    //随机数 32 bytes
    //
    Random []byte

    //sessionId的长度 1 bytes
    SessionIdLen uint8

    //sessionId 长度为SessionIdLen bytes
    SessionId []byte

    //加密套件长度 2 bytes
    //每个加密套件类型长度为2bytes
    //所以此长度必定为偶数
    CipherSuitsLen int

    //加密套件 每个长度为2 bytes总长度为 CipherSuitsLen bytes
    CipherSuits []byte

    //压缩方法长度 1bytes
    CompressionMethodLen uint8

    //压缩方法 CompressionMethodLen bytes
    CompressionMethods []byte

    //扩展字段长度 2 bytes
    ExtensionsLen int

    //以下为扩展字段
    //每个扩展字段的格式为 Type 2bytes, Length 2bytes, Data Length bytes

    //serverName在扩展字段中
    //格式为 Type 2 bytes 0x0000
    //Length 2 bytes
    //ListLength 2bytes
    //type 1bytes hostname 为0x0
    //length 2 bytes
    //data length bytes
    ServerName string
}



func readClientHello(reader io.Reader) (client *ClientHello, err error){
	bufReader := bufio.NewReader(reader)

	//读取n个字节
	readBytes := func(n int) ([]byte, error){
		p := make([]byte, n)
		var read = 0
        for {
			n1, err1 := bufReader.Read(p[read: ])
			read += n1

			if read == n{
				return p, nil
			}//if

			if err1 != nil{
				return p, err1
			}//if
		}//for
	}//readBytes

    //readHeader
    client = new(ClientHello)

    var line []byte

    if line, err = readBytes(5); err != nil{
    	return
	}

    client.ContentType = uint8(line[0])

    if client.ContentType == 0x80{
		return nil, alertUnsupportTls
	}

    client.Version = uToInt(line[1: 3])
    client.Length = uToInt(line[3: 5])

    if client.Length > maxCiphertext {
    	return nil, alertRecordOverflow
	}

	if (client.ContentType != 0x16) || client.Version >= 0x1000 || client.Length >= 0x3000 {
		return nil, alertUnexpectedMsg
	}


	var handshake []byte
    if handshake, err = readBytes(client.Length); err != nil{
    	return
	}//if

	//handshake 必定有不小于46个字节
	if len(handshake) > maxPlaintext || len(handshake) < 46{
		return nil, alertRecordOverflow
	}

	//handshake type
	client.HandshakeTyp = uint8(handshake[0])

	if client.HandshakeTyp != 0x01{
		return nil, unexpectHttpMsg
	}

	//length
	client.HandshakeLength = uToInt(handshake[1: 4])

    if client.HandshakeLength > maxHandshake{
    	return nil, alertInternalError
	}

	if client.HandshakeLength > len(handshake[4: ]){
		return nil, alertUnexpectedMsg
	}//if

	client.HandshakeVersion = uToInt(handshake[4: 6])

	handshake = handshake[6: ]

	client.Random = make([]byte, 32)
	copy(client.Random, handshake)
	handshake = handshake[32: ]


	client.SessionIdLen = uint8(handshake[0])
	//
	if len(handshake) < int(client.SessionIdLen) + 1{
		return nil, alertUnexpectedMsg
	}//if

	client.SessionId= make([]byte, int(client.SessionIdLen))
	copy(client.SessionId, handshake)
	handshake = handshake[client.SessionIdLen + 1:]

	if len(handshake) < 5{
		return nil, alertUnexpectedMsg
	}//if

	//suites
	client.CipherSuitsLen = uToInt(handshake[: 2])
	handshake = handshake[2: ]
	if client.CipherSuitsLen % 2 == 1 || len(handshake) < client.CipherSuitsLen{
		err = unexpectHttpMsg
		return
	}//if


	client.CipherSuits = make([]byte, client.CipherSuitsLen)
	copy(client.CipherSuits, handshake[: client.CipherSuitsLen])

	handshake = handshake[client.CipherSuitsLen: ]

    if len(handshake) < 3{
    	return nil, alertUnexpectedMsg
	}//if


	client.CompressionMethodLen = uint8(handshake[0])
	handshake = handshake[1: ]

	if len(handshake) < int(client.CompressionMethodLen){
		return nil, alertUnexpectedMsg
	}


	client.CompressionMethods = make([]byte, int(client.CompressionMethodLen))
	copy(client.CompressionMethods, handshake)

	handshake = handshake[client.CompressionMethodLen: ]

	if len(handshake) < 2{
		return nil, alertUnexpectedMsg
	}

	client.ExtensionsLen = uToInt(handshake[:2])
    handshake = handshake[2: ]

    if len(handshake) != client.ExtensionsLen{
    	return nil, alertUnexpectedMsg
	}//if

	extensions := handshake

	//从extensions中解析出serverName

	for len(extensions) > 0{
		if len(extensions) < 4{
			return nil, alertUnexpectedMsg
		}//if

		extTyp := uToInt(extensions[: 2])
		extLen := uToInt(extensions[2: 4])
		if len(extensions[4: ]) < extLen{
			return nil, alertUnexpectedMsg
		}

        data := extensions[4: 4 + extLen]
		extensions = extensions[4 + extLen: ]

		if extTyp != 0x000{
			continue
		}//if

		//serverName
		if len(data) < 2{
			return nil, alertUnexpectedMsg
		}//if

        serverListLen := uToInt(data[: 2])

        if len(data[2: ]) < serverListLen{
        	return nil, alertUnexpectedMsg
		}//if

        data = data[2: ]
        if len(data) == 0{
        	continue
		}//if

		if len(data) < 3 {
			return nil, alertUnexpectedMsg
		}//if

        t := uint8(data[0])
        l := uToInt(data[1: 3])

        if len(data[3: ]) < l{
        	return nil, alertUnexpectedMsg
		}//

        d := data[3: 3+l]

        if t == 0x0{
        	client.ServerName = string(d)
        }//if

	}//for

	return client, nil
}