package enums

// ListingStatus 挂单（entry_orders）状态，与库中 int 一一对应（0–4）。
type ListingStatus uint

const (
	ListingReady     ListingStatus = iota // 准备中（未上链）
	ListingPending                        // 进行中（已上架）
	ListingCompleted                      // 已成交
	ListingExpired                        // 已过期
	ListingCancelled                      // 已取消（含卖家取消、上链失败等）
)

// Desc 返回挂单状态中文描述。
func (s ListingStatus) Desc() string {
	switch s {
	case ListingReady:
		return "准备中"
	case ListingPending:
		return "进行中"
	case ListingCompleted:
		return "已成交"
	case ListingExpired:
		return "已过期"
	case ListingCancelled:
		return "已取消"
	default:
		return "未知"
	}
}
