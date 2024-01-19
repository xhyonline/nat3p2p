package server

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net"
)

// Message 是一条标准消息的实现
type Message struct {
	Body string `json:"body"`
}

// Encode 消息编码
func (m *Message) Encode() []byte {
	// 序列化为 json
	message, _ := json.Marshal(m)
	// 读取该 json 的长度
	var length = int32(len(message))
	var pkg = new(bytes.Buffer)
	// 写入消息头
	err := binary.Write(pkg, binary.BigEndian, length)
	if err != nil {
		return nil
	}
	// 写入消息实体
	err = binary.Write(pkg, binary.BigEndian, message)
	if err != nil {
		return nil
	}
	return pkg.Bytes()
}

// NewMsg 实例化消息实体
func NewMsg(msg interface{}) *Message {
	body, _ := json.Marshal(msg)
	return &Message{Body: string(body)}
}

// Ctx 消息上下文
type Ctx struct {
	RemoteAddr net.Addr
	Body       []byte
	BodyString string
	FromConn   net.Conn
}

// BizStandardMsg 业务标准消息
type BizStandardMsg struct {
	MsgType int         `json:"msg_type"`
	MsgBody interface{} `json:"msg_body"`
}

// OnMsg 当消息事件发生时
func OnMsg(conn net.Conn, handelFunc func(Ctx), handelError func(conn net.Conn, err error)) {
	reader := bufio.NewReader(conn)
	for {
		// 前4个字节表示数据长度
		// 此外 Peek 方法并不会减少 reader 中的实际数据量
		peek, err := reader.Peek(4)
		if err != nil {
			handelError(conn, err)
			return
		}
		buffer := bytes.NewBuffer(peek)
		var length int32
		// 读取缓冲区前4位,代表消息实体的数据长度,赋予 length 变量
		err = binary.Read(buffer, binary.BigEndian, &length)
		if err != nil {
			handelError(conn, err)
			return
		}
		// reader.Buffered() 返回缓存中未读取的数据的长度,
		// 如果缓存区的数据小于总长度，则意味着数据不完整,很可能是内核态没有完全拷贝数据到用户态中
		// 因此下一轮就齐活了
		if int32(reader.Buffered()) < length+4 {
			continue
		}
		//从缓存区读取大小为数据长度的数据
		data := make([]byte, length+4)
		_, err = reader.Read(data)
		if err != nil {
			handelError(conn, err)
			return
		}
		m := new(Message)
		_ = json.Unmarshal(data[4:], m)
		handelFunc(Ctx{
			RemoteAddr: conn.RemoteAddr(),
			Body:       data[4:],
			BodyString: m.Body,
			FromConn:   conn,
		})
	}
}
