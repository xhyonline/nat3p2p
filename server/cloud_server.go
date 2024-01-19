package server

import (
	"github.com/gogf/gf/container/gmap"
	"github.com/gogf/gf/encoding/gjson"
	"github.com/gogf/gf/frame/g"
	"github.com/gogf/gf/os/glog"
	"github.com/gogf/gf/util/gconv"
	reuse "github.com/libp2p/go-reuseport"
	"net"
)

type Cloud struct {
	// 好友列表,key 为用户昵称 v 为 conn 连接句柄
	FriendList *gmap.Map
	// 链接映射用户
	ConnRemapUserNickname *gmap.Map
	// 云端监听的本地地址
	Local string
	// 云端监听地址
	listener net.Listener
}

// NewCloud 创建一个云端地址
func NewCloud(local string) *Cloud {
	return &Cloud{
		FriendList:            gmap.New(true),
		ConnRemapUserNickname: gmap.New(true),
		Local:                 local,
	}
}

// Run 运行云端
func (s *Cloud) Run() {
	listener, err := reuse.Listen("tcp", s.Local)
	if err != nil {
		glog.Fatalf("云端监听地址失败,%s", err)
	}
	s.listener = listener
	go s.accept()
	select {}
}

// accept 接受客户端
func (s *Cloud) accept() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			glog.Fatalf("云端监听发生错误:%s", err)
		}
		go OnMsg(conn, s.OnMsg, s.OnMsgError)
	}
}

// OnMsg 读取到消息时
func (s *Cloud) OnMsg(ctx Ctx) {
	parseJSON, err := gjson.LoadContent(ctx.BodyString)
	if err != nil {
		glog.Fatal(err)
	}
	msgType := parseJSON.GetInt("msg_type")
	switch msgType {
	case ClientRegister:
		s.ClientOnline(ctx.FromConn, parseJSON)
	case ClientGetFriendList: // 获取在线的用户列表
		s.ClientGetFriendList(ctx.FromConn, parseJSON)
	case ClientNotifyCloudHolePunching: // 客户端通知云端向另一端发送打洞消息
		s.CloudNotifyHolePunching(ctx.FromConn, parseJSON)
	}
}

// ClientNotifyCloudHolePunching 云端通知对端开始进行打洞
func (s *Cloud) CloudNotifyHolePunching(src net.Conn, parseJSON *gjson.Json) {
	srcNickname := parseJSON.GetString("body.my_nickname")
	distNickname := parseJSON.GetString("body.dst_nickname")
	srcUser := s.FriendList.GetVar(srcNickname)
	dstUser := s.FriendList.GetVar(distNickname)
	if dstUser.IsEmpty() {
		glog.Error("云端通知打洞失败 dst empty")
		return
	}
	if srcUser.IsEmpty() {
		glog.Error("云端通知打洞失败 src empty")
		return
	}
	// 通知另一端进行打洞
	dstConn := dstUser.Interface().(net.Conn)
	srcConn := srcUser.Interface().(net.Conn)
	go func() {
		_, _ = dstConn.Write(NewMsg(&ClientMsg{
			MsgType: CloudNotifyClientHolePunching,
			Body: g.Map{
				"src_nickname":    srcNickname,
				"src_remote_addr": src.RemoteAddr().String(),
			},
		}).Encode())
	}()
	go func() {
		_, _ = srcConn.Write(NewMsg(&ClientMsg{
			MsgType: CloudNotifyClientHolePunching,
			Body: g.Map{
				"src_nickname":    distNickname,
				"src_remote_addr": dstConn.RemoteAddr().String(),
			},
		}).Encode())
	}()
}

// ClientOffline 客户端下线事件通知
func (s *Cloud) ClientOffline(offlineNickname string) {
	conn := s.FriendList.Get(offlineNickname).(net.Conn)
	s.FriendList.Remove(offlineNickname)                       // 删除该用户
	s.ConnRemapUserNickname.Remove(conn.RemoteAddr().String()) // 删除该用户
	tmp := &ClientMsg{
		MsgType: CloudBroadcastOffline,
		Body: g.Map{
			"nickname": offlineNickname,
		},
	}
	msg := NewMsg(tmp).Encode()
	// 全体用户发通知,告知该用户下线了
	s.FriendList.Iterator(func(nicknameKey interface{}, connIface interface{}) bool {
		conn := connIface.(net.Conn)
		_, _ = conn.Write(msg)
		return true
	})
	glog.Infof("用户:%s已下线", offlineNickname)
}

// ClientGetFriendList 获取云端的好友列表
func (s *Cloud) ClientGetFriendList(conn net.Conn, _ *gjson.Json) {
	friends := make([]*Friend, 0)
	s.FriendList.Iterator(func(nickname interface{}, connIface interface{}) bool {
		remoteAddr := connIface.(net.Conn).RemoteAddr().String()
		friends = append(friends, &Friend{
			NickName:   gconv.String(nickname),
			RemoteAddr: remoteAddr,
		})
		return true
	})
	_, _ = conn.Write(NewMsg(&ClientMsgResult{
		MsgType: ClientGetFriendList,
		Body: g.Map{
			"list": friends,
		},
	}).Encode())
}

// ClientOnline 客户端上线通知
func (s *Cloud) ClientOnline(conn net.Conn, parseJSON *gjson.Json) {
	nickname := parseJSON.GetString("body.nickname")
	gvar := s.FriendList.GetVar(nickname)
	if !gvar.IsEmpty() {
		clientMsg := &ClientMsgResult{
			MsgType: CloudNotifyClientRegisterRes,
			Code:    Error,
			Msg:     "聊天室已有和您相同的昵称,请重新注册一个昵称",
		}
		if _, err := conn.Write(NewMsg(clientMsg).Encode()); err != nil {
			glog.Error(err)
		}
		_ = conn.Close()
		return
	}
	clientMsg := &ClientMsgResult{
		MsgType: CloudNotifyClientRegisterRes,
		Code:    Success,
		Msg:     "success",
	}
	if _, err := conn.Write(NewMsg(clientMsg).Encode()); err != nil {
		glog.Error(err)
	}
	// 写入用户列表
	s.FriendList.Set(nickname, conn)
	s.ConnRemapUserNickname.Set(conn.RemoteAddr().String(), nickname)
	// 广播事件,有用户上线了
	s.CloudBroadcast(&ClientMsgResult{
		MsgType: CloudBroadcastClientOnline,
		Body: g.Map{
			"nickname":    nickname,
			"remote_addr": conn.RemoteAddr().String(),
		},
	})
}

// CloudBroadcast 云端广播消息
func (s *Cloud) CloudBroadcast(msg interface{}) {
	waitSendMsg := NewMsg(msg).Encode()
	s.FriendList.Iterator(func(nickname interface{}, connIface interface{}) bool {
		go func() {
			conn := connIface.(net.Conn)
			_, _ = conn.Write(waitSendMsg)
		}()
		return true
	})
}

// OnMsgError 当发生错误时
func (s *Cloud) OnMsgError(conn net.Conn, _ error) {
	var nickname string
	s.FriendList.Iterator(func(nicknameKey interface{}, connIface interface{}) bool {
		listConn := connIface.(net.Conn)
		if listConn.RemoteAddr().String() == conn.RemoteAddr().String() {
			nickname = gconv.String(nicknameKey)
			return false // 退出循环
		}
		return true
	})
	if nickname != "" {
		s.ClientOffline(nickname)
	}
}
