package server

const (
	No = iota
	Yes
)

const (
	Success = iota
	Error
)

const (
	// 云端消息发送给客户端的类型
	CloudBroadcastClientOnline    = iota + 1 // 客户端上线广播
	CloudBroadcastOffline                    // 客户端下线广播
	CloudNotifyClientRegisterRes             // 云端通知客户端注册结果
	CloudNotifyClientHolePunching            // 云端通知客户端,向来源打洞
	ClientNotifyCloudHolePunching            // 客户端通知云端,让对方打洞
	ClientGetFriendList                      // 云端提供在线好友列表
	// 客户端告知云端的消息类型
	ClientRegister // 客户端向云端进行注册
	PeerMsgSay     // 消息
)

type CommonResult struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// ClientRegisterMsg 客户端注册事件
type ClientRegisterMsg struct {
	NickName string `json:"nickname"`
}

type ClientMsg struct {
	MsgType int         `json:"msg_type"`
	Body    interface{} `json:"body"`
}

// ClientMsgResult 客户端注册结果
type ClientMsgResult struct {
	MsgType int         `json:"msg_type"`
	Code    int         `json:"code"`
	Msg     string      `json:"msg"`
	Body    interface{} `json:"body"`
}

// CloudBroadcastRegisterMsg 云端广播注册事件
type CloudBroadcastRegisterMsg struct {
	NickName   string `json:"nickname"`
	RemoteAddr string `json:"remote_addr"`
}

// CloudBroadcastOfflineMsg 广播通知用户下线
type CloudBroadcastOfflineMsg struct {
}

type Friend struct {
	NickName   string `json:"nickname"`
	RemoteAddr string `json:"remote_addr"`
}
