package enums

// BidStatus 出价（bid_placed）状态，与库中 int 一一对应（0–7）。
type BidStatus uint

const (
	BidPending   BidStatus = iota // 进行中（已出价）
	BidCompleted                  // 已成交（中标）
	BidExpired                    // 已过期
	BidCancelled                  // 取消出价（含撤回、上链失败等）
	BidDelisted                   // 已下架（挂单下架后，该出价失效）
	BidOutbid                     // 未中标（他人成交后可 refundLosingBid）
	BidRefunded                   // 已退款（链上退款完成）
)

// Desc 返回出价状态中文描述。
func (s BidStatus) Desc() string {
	switch s {
	case BidPending:
		return "进行中"
	case BidCompleted:
		return "已成交"
	case BidExpired:
		return "已过期"
	case BidCancelled:
		return "取消出价"
	case BidDelisted:
		return "已下架"
	case BidOutbid:
		return "未中标"
	default:
		return "未知"
	}
}
