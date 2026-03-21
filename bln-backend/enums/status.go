package enums

type Status uint

const (
	Ready     Status = iota // 准备中 => 未上链
	Pending                 // 进行中 => 已上链
	Completed               // 已完成 => 已成交
	Expired                 // 已过期
	Cancelled               // 已取消
	Invalid                 // 已失效
)

// Desc 返回状态对应的中文描述。
func (s Status) Desc() string {
	switch s {
	case Ready:
		return "准备中"
	case Pending:
		return "进行中"
	case Completed:
		return "已成交"
	case Expired:
		return "已过期"
	case Cancelled:
		return "已取消"
	case Invalid:
		return "已失效"
	default:
		return "未知"
	}
}
