package server

import (
	"bufio"
	"fmt"
	"github.com/gogf/gf/container/gmap"
	"github.com/gogf/gf/encoding/gjson"
	"github.com/gogf/gf/frame/g"
	"github.com/gogf/gf/os/glog"
	"github.com/gogf/gf/os/gtime"
	"github.com/gogf/gf/util/gconv"
	"github.com/gookit/color"
	reuse "github.com/libp2p/go-reuseport"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// TmpPeerConn 临时对等节点
type TmpPeerConn struct {
	nickname  string
	Timestamp int64
}

// WaitHandShake
type WaitHandShake struct {
	PeerConn  net.Conn // 对等节点
	UUID      string
	Timestamp int64
}

type Client struct {
	nickname string
	// 本地监听地址
	Local string
	// 云端地址
	Cloud string
	// 聊天处理函数
	HandleMsg func(ctx Ctx)
	// 云端连接
	CloudConn net.Conn
	// 地址转昵称
	RemoteAddrToNickname *gmap.Map
	// 好友列表 key:nickname value:remoteAddr
	FriendList *gmap.Map
	// 等待连接对等节点 key 对等节点 nickname
	WaitConnectPeer *gmap.Map
	// 自己是否注册成功
	registerInit *int32
	// 对等节点连接集合,当云端通知用户下线时,此处会自动剔除对等连接
	// key:nickname,value:net.Conn
	PeersConn *gmap.Map
	// 监听句柄
	listener net.Listener
	// key:nickname value:int32	原子性操作
	nicknameConnectLock *gmap.Map
	// 是否首次刷新用户列表
	isFirstRefreshUserList *int32
	// 头行管道
	headline chan struct{}
}

// NewClient 实例化客户端
func NewClient(cloud, local, nickname string) *Client {
	ins := &Client{
		nickname:               nickname,
		Local:                  local,
		Cloud:                  cloud,
		FriendList:             gmap.New(true),
		RemoteAddrToNickname:   gmap.New(true),
		PeersConn:              gmap.New(true),
		nicknameConnectLock:    gmap.New(true),
		WaitConnectPeer:        gmap.New(true),
		registerInit:           new(int32),
		isFirstRefreshUserList: new(int32),
		headline:               make(chan struct{}, 3),
	}
	return ins
}

// OnMessage 注册消息发送时触发的事件函数
func (s *Client) On1Message(handel func(ctx Ctx)) {
	s.HandleMsg = handel
}

// cloudDail 云端链接
func (s *Client) cloudDail(nickname string) {
	cloudConn, err := reuse.Dial("tcp", s.Local, s.Cloud)
	if err != nil {
		glog.Fatal(err)
	}
	s.CloudConn = cloudConn
	s.RegisterUser(nickname)
	// 持续监听云端事件,包括客户端注册事件
	go OnMsg(cloudConn, s.onCloudMsg, s.onCloudMsgError)
}

// onCloudMsgError 当读取消息失败时
func (s *Client) onCloudMsgError(conn net.Conn, err error) {
	color.LightBlue.Printf("\n云端链接断开\n")
	_ = conn.Close()
}

// RegisterUser 发送注册消息,告诉云端我上线了
func (s *Client) RegisterUser(nickname string) {
	registerMsg := &ClientMsg{
		MsgType: ClientRegister,
		Body: g.Map{
			"nickname": nickname,
		},
	}
	msg := NewMsg(registerMsg).Encode()
	if _, err := s.CloudConn.Write(msg); err != nil {
		glog.Fatalf("向云端发送注册消息失败,错误:%s", err)
	}
}

// onCloudMsg 获取到云端的消息时需要执行的方法
func (s *Client) onCloudMsg(ctx Ctx) {
	parseJSON, err := gjson.LoadContent(ctx.BodyString)
	if err != nil {
		glog.Fatalf("无法解析云端数据:%s", ctx.BodyString)
	}
	msgType := parseJSON.GetInt("msg_type")
	if atomic.LoadInt32(s.registerInit) == No && msgType != CloudNotifyClientRegisterRes {
		return
	}
	// 获取消息类型
	switch msgType {
	case CloudNotifyClientRegisterRes: // 获取注册结果
		s.HandleRegisterRes(parseJSON)
	case CloudBroadcastClientOnline: // 客户端上线事件
		s.HandleClientOnline(parseJSON)
	case CloudBroadcastOffline: // 客户端下线事件
		s.HandleClientOffline(parseJSON)
	case ClientGetFriendList: // 云端提供在线好友列表事件
		s.HandleFriendList(parseJSON)
	case CloudNotifyClientHolePunching: // 云端通知客户端向对等节点进行打洞
		s.ClientHolePunching(parseJSON)
	}
}

// CloudBroadcastClientOnline 客户端注册事件
func (s *Client) HandleClientOnline(msgJSON *gjson.Json) {
	nickname := msgJSON.GetString("body.nickname")
	remoteAddr := msgJSON.GetString("body.remote_addr")
	s.nicknameConnectLock.Set(nickname, new(int32))
	s.FriendList.Set(nickname, remoteAddr)
	s.RemoteAddrToNickname.Set(remoteAddr, nickname)
	color.Cyan.Printf("\n云端全体广播事件,用户:%s 地址:%s 在聊天室中已上线\n", nickname, remoteAddr)
}

// CloudBroadcastOffline 客户端离线事件
func (s *Client) HandleClientOffline(msgJSON *gjson.Json) {
	nickname := msgJSON.GetString("body.nickname")
	color.Cyan.Printf("\n云端全体广播事件,用户:%s 在聊天室中下线\n", nickname)
	s.removeUser(nickname)
}

// HandleRegisterRes 处理注册结果
func (s *Client) HandleRegisterRes(msgJSON *gjson.Json) {
	if msgJSON.GetInt("code") != Success {
		glog.Fatalf("加入聊天室注册失败,原因:%s", msgJSON.GetString("msg"))
	}
	atomic.AddInt32(s.registerInit, 1) // 从此刻起,用户可以接收所有数据
	// 向云端获取全部在线的好友列表
	s.RefreshOnlineFriendList()
}

// RefreshOnlineFriendList 向云端获取好友列表
func (s *Client) RefreshOnlineFriendList() {
	clientMsg := &ClientMsg{
		MsgType: ClientGetFriendList,
	}
	msg := NewMsg(clientMsg).Encode()
	if _, err := s.CloudConn.Write(msg); err != nil {
		glog.Fatalf("向云端发送注册消息失败,错误:%s", err)
	}
}

// HandleFriendList 获取云端提供的好友列表
func (s *Client) HandleFriendList(msgJSON *gjson.Json) {
	if msgJSON.GetInt("code") != Success {
		glog.Error("获取云端好友列表失败,错误:%s", msgJSON.GetString("msg"))
		return
	}
	var printResult = new(strings.Builder)
	friends := msgJSON.GetJsons("body.list")
	printResult.WriteString("\n当前在线的好友列表如下\n")
	var initNicknameConnLock bool
	if atomic.AddInt32(s.isFirstRefreshUserList, 1) == 1 {
		initNicknameConnLock = true
	}
	for _, v := range friends {
		if initNicknameConnLock {
			s.nicknameConnectLock.Set(v.GetString("nickname"), new(int32))
		}
		res := fmt.Sprintf("昵称:%s 通讯地址:%s\n", v.GetString("nickname"),
			v.GetString("remote_addr"))
		printResult.WriteString(res)
		s.RemoteAddrToNickname.Set(v.GetString("remote_addr"), v.GetString("nickname"))
		s.FriendList.Set(v.GetString("nickname"), v.GetString("remote_addr"))
	}
	fmt.Println(printResult.String())
	s.headline <- struct{}{}
}

// tryConnPeer 本地客户端不停尝试连接对等节点
func (s *Client) tryConnPeer() {
	for {
		wg := new(sync.WaitGroup)
		s.FriendList.Iterator(func(nickname interface{}, remoteAddrString interface{}) bool {
			// 自己不能向自己链接
			if gconv.String(nickname) == s.nickname {
				return true
			}
			peerConnAddr := s.PeersConn.GetVar(nickname)
			// 已经连接上的用户跳过
			if !peerConnAddr.IsEmpty() {
				return true
			}
			// 等待连接的跳过
			if !s.WaitConnectPeer.GetVar(nickname).IsEmpty() {
				return true
			}
			wg.Add(1)
			go s.ConnPeerAndNotifyCloud(wg, gconv.String(nickname))
			return true
		})
		wg.Wait()
		time.Sleep(time.Millisecond * 200)
	}
}

// removeUser 移除用户
func (s *Client) removeUser(nickname string) {
	friendAddr := s.FriendList.GetVar(nickname)
	if !friendAddr.IsEmpty() {
		s.RemoteAddrToNickname.Remove(friendAddr.String())
	}
	s.FriendList.Remove(nickname)
	s.PeersConn.Remove(nickname)
	s.nicknameConnectLock.Remove(nickname)
	s.WaitConnectPeer.Remove(nickname)
}

// ConnPeerAndNotifyCloud 通知云端
func (s *Client) ConnPeerAndNotifyCloud(wg *sync.WaitGroup, nickname string) {
	defer wg.Done()
	s.WaitConnectPeer.Set(nickname, "")
	_, _ = s.CloudConn.Write(NewMsg(&ClientMsg{
		MsgType: ClientNotifyCloudHolePunching,
		Body: g.Map{
			"my_nickname":  s.nickname,
			"dst_nickname": nickname,
		},
	}).Encode())
}

// peerJoin 加入对等节点
func (s *Client) peerJoin(nickname string, conn net.Conn) {
	s.PeersConn.Set(nickname, conn)
	color.Green.Printf("\n昵称:%s,加入聊天,对端远程地址:%s\n", nickname, conn.RemoteAddr().String())
}

// accept 监听对等节点连接事件
func (s *Client) accept() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			glog.Fatal(err)
		}
		nickname := s.RemoteAddrToNickname.GetVar(conn.RemoteAddr().String()).String()
		color.LightBlue.Printf("\n昵称:%s,加入聊天,对端远程地址:%s\n", nickname, conn.RemoteAddr().String())
		go OnMsg(conn, s.OnPeerMsg, s.OnPeerMsgError)
	}
}

// OnPeerMsg 当对等节点发来连接消息的请求
func (s *Client) OnPeerMsg(ctx Ctx) {
	parseJSON, err := gjson.LoadContent(ctx.BodyString)
	if err != nil {
		glog.Fatal(err)
	}
	msgType := parseJSON.GetInt("msg_type")
	switch msgType {
	case PeerMsgSay:
		s.OnPeerMsgSay(ctx, parseJSON)
	default:
		glog.Infof("对等节点收到未知消息:%s", ctx.BodyString)
	}
}

// OnPeerMsgSay
func (s *Client) OnPeerMsgSay(ctx Ctx, parseJSON *gjson.Json) {
	nickname := parseJSON.GetString("body.nickname")
	say := parseJSON.GetString("body.say")
	date := gtime.Now().Format("Y-m-d H:i:s")
	color.LightBlue.Printf("\n%s 来自%s:%s的消息:%s", date, ctx.FromConn.RemoteAddr().String(), nickname, say)
	s.headline <- struct{}{}
}

// ClientHolePunching 客户端
func (s *Client) ClientHolePunching(parseJSON *gjson.Json) {
	nickname := parseJSON.GetString("body.src_nickname")
	remoteAddr := parseJSON.GetString("body.src_remote_addr")
	lock := s.nicknameConnectLock.GetVar(nickname)
	// 避免同时打洞,此处原子性保证
	if lock.IsEmpty() {
		var lockInt int32 = 1
		s.nicknameConnectLock.Set(nickname, &lockInt)
	} else if atomic.AddInt32(lock.Interface().(*int32), 1) != 1 {
		return
	}
	go func() {
	FOR:
		for {
			select {
			case <-time.After(time.Minute * 2):
				break FOR
			default:
			}
			// 打洞也需要检查该好友是否在线
			if s.FriendList.GetVar(nickname).IsEmpty() {
				break
			}
			conn, err := reuse.Dial("tcp", s.Local, remoteAddr)
			if err == nil {
				s.peerJoin(nickname, conn)
				go OnMsg(conn, s.OnPeerMsg, s.OnPeerMsgError)
				break
			}
			time.Sleep(time.Millisecond * 100)
		}
		s.WaitConnectPeer.Remove(nickname)
	}()
}

// OnPeerMsgError 当对等节点失败
func (s *Client) OnPeerMsgError(conn net.Conn, err error) {
	var nickname string
	s.PeersConn.Iterator(func(nicknameIface interface{}, connIface interface{}) bool {
		remoteAddr := connIface.(net.Conn).RemoteAddr().String()
		if remoteAddr == conn.RemoteAddr().String() {
			nickname = gconv.String(nicknameIface)
			return false
		}
		return true
	})
	_ = conn.Close()
	if nickname != "" {
		color.LightBlue.Printf("\n用户:%s 断开连接 离线\n", nickname)
		s.removeUser(nickname)
	}
}

// ReadStdinSend 读取标准输入并发送
func (s *Client) ReadStdinSend() {
	for {
		inputReader := bufio.NewReader(os.Stdin)
		input, err := inputReader.ReadString('\n')
		if err != nil {
			fmt.Println("您发送的有误:", err.Error())
			continue
		}
		if strings.TrimSpace(input) == "" {
			s.headline <- struct{}{}
			continue
		}
		s.headline <- struct{}{}
		s.PeersConn.Iterator(func(k interface{}, v interface{}) bool {
			peerConn := v.(net.Conn)
			_, _ = peerConn.Write(NewMsg(&ClientMsg{
				MsgType: PeerMsgSay,
				Body: g.Map{
					"nickname": s.nickname,
					"say":      input,
				},
			}).Encode())
			return true
		})
	}
}

// inputHeadLine 输出头行
func (s *Client) inputHeadLine() {
	for {
		select {
		case <-s.headline:
			color.Red.Printf("请说点什么:")
		}
	}
}

// Run 执行启动函数
func (s *Client) Run() {
	s.cloudDail(s.nickname)
	listener, err := reuse.Listen("tcp", s.Local)
	if err != nil {
		glog.Fatal(err)
	}
	s.listener = listener
	go s.accept()
	go s.tryConnPeer()
	go s.ReadStdinSend()
	go s.inputHeadLine()
	select {}
}
