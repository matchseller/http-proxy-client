package main

import (
	"fmt"
	"github.com/matchseller/http-proxy-client/log"
	"github.com/matchseller/http-proxy-client/parser"
	"github.com/matchseller/http-proxy-client/response"
	"github.com/matchseller/http-proxy-client/util"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

type manager struct {
	dConn *net.Conn	//与代理服务器的连接
	localConn *net.Conn	//与本地被代理服务的连接
	requestData chan parser.PData
	maxRequestLen int
	maxResponseLen int

	args map[string]string
}

var wg sync.WaitGroup

func main() {
	m := manager{
		dConn:     nil,
		localConn: nil,
		maxRequestLen: 10*1024*1024,
		maxResponseLen: 10*1024*1024,
		requestData: make(chan parser.PData),
		args: make(map[string]string),
	}

	err := m.parseArgs()
	if err != nil {
		panic(err)
	}

	//连接代理服务器
	err = m.connToServer()
	if err != nil {
		if m.dConn != nil {
			(*m.dConn).Close()
		}
		panic(err)
	}

	fmt.Println("Proxy is running!")

	//转发消息
	wg.Add(1)
	go m.readFromServer()
	wg.Wait()

	fmt.Println("Proxy stopped unexpectedly!")
}

func (m *manager)parseArgs() error {
	argKeys := []string{"sAddr", "host", "lAddr"}

	if len(os.Args) < 4 {
		return fmt.Errorf("not enough parameters:%v",os.Args[1:])
	}
	for i := 1; i < len(os.Args); i++ {
		argArr := strings.Split(os.Args[i], "=")
		if len(argArr) != 2 {
			return fmt.Errorf("incorrect parameter format:%v", os.Args[i])
		}
		if !util.StringInArray(argKeys, argArr[0]) {
			return fmt.Errorf("incorrect parameter key:%v", argArr[0])
		}
		if strings.TrimSpace(argArr[1]) == "" {
			return fmt.Errorf("the key '%v''s value cannot be empty", argArr[0])
		}

		m.args[argArr[0]] = argArr[1]
	}
	return nil
}

//连接代理服务器
func (m *manager)connToServer() (err error) {
	conn, err := net.Dial("tcp4", m.args["sAddr"])
	if err != nil {
		return fmt.Errorf("connect to proxy server error:%v", err)
	}
	m.dConn = &conn

	//验证身份
	passport := m.args["host"] + "\r\ne10adc3949ba59abbe56e057f20f883e\r\n"
	_, err = conn.Write([]byte(passport))
	if err != nil {
		return fmt.Errorf("send message to proxy server error:%v", err)
	}

	//判断验证是否通过
	buff := make([]byte, 256)
	bLen, err := conn.Read(buff)
	if err != nil {
		return fmt.Errorf("read from proxy server error:%v", err)
	}

	str := string(buff[:bLen])
	strArr := strings.Split(str, " ")
	if len(strArr) < 2 {
		return fmt.Errorf("data format error:%v", str)
	}

	if strArr[0] != "200" {
		return fmt.Errorf("authentication failed:%v", strings.Join(strArr[1:], ""))
	}

	return
}

//从代理服务器读取请求并转发给本地服务
func (m *manager)readFromServer() {
	defer func() {
		(*m.dConn).Close()
		close(m.requestData)
		wg.Done()
	}()

	go m.acceptRequest()

	err := parser.NewRequest(m.dConn, m.maxRequestLen).Read(m.requestData)
	if err != nil {
		log.MyLogger.Println("connection closed unexpectedly:", err)
	}
}

func (m *manager)acceptRequest() {
	for {
		data, isOk := <- m.requestData
		if !isOk {
			break
		}

		//连接本地服务
		err := m.connToLocal()
		if err != nil {
			log.MyLogger.Println("connect to local server error:", err)
			response.NewResponse(http.StatusServiceUnavailable, m.dConn).Error()
			continue
		}

		//向本地服务转发请求
		_, err = (*m.localConn).Write([]byte(data.BinaryData))
		if err != nil {
			response.NewResponse(http.StatusServiceUnavailable, m.dConn).Error()
			continue
		}
		m.sendResponse()
	}
}

//连接本地被代理的服务
func (m *manager)connToLocal() (err error) {
	//连接被代理的本地服务
	conn, err := net.Dial("tcp4", m.args["lAddr"])
	if err != nil {
		return fmt.Errorf("connect to local server error:%v", err)
	}
	m.localConn = &conn
	return
}

//从本地服务读取响应并转发给代理服务
func (m *manager)sendResponse() {
	pData, err := parser.NewResponse(m.localConn, m.maxResponseLen).Read()
	if err != nil {
		response.NewResponse(http.StatusInternalServerError, m.dConn).Error()
		log.MyLogger.Println("connection closed unexpectedly:", err)
		return
	}
	(*m.dConn).Write([]byte(pData.BinaryData))
}
