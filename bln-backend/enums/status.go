package enums

type Status uint

const (
	Pending   Status = iota // 待处理
	Completed               // 已完成
	Expired                 // 已过期
	Cancelled               // 已取消
	Invalid                 // 已失效
)
