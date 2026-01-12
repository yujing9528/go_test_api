package todo

import (
	"math"
	"net/http"
	"sort"
	"sync"
)

type paymentMethod interface {
	// Name 返回支付方式标识，用于输出与分流逻辑。
	Name() string
	// Fee 根据订单金额计算手续费。
	Fee(amount float64) float64
}

type cardPayment struct {
	Rate  float64
	Fixed float64
}

func (c cardPayment) Name() string {
	return "card"
}

func (c cardPayment) Fee(amount float64) float64 {
	return amount*c.Rate + c.Fixed
}

type walletPayment struct {
	Rate float64
}

func (w walletPayment) Name() string {
	return "wallet"
}

func (w walletPayment) Fee(amount float64) float64 {
	return amount * w.Rate
}

type bankTransfer struct {
	FeeAmount float64
}

func (b bankTransfer) Name() string {
	return "bank_transfer"
}

func (b bankTransfer) Fee(amount float64) float64 {
	return b.FeeAmount
}

type paymentQuote struct {
	Method      string  `json:"method"`
	Fee         float64 `json:"fee"`
	FinalAmount float64 `json:"final_amount"`
}

type orderInput struct {
	OrderID  string
	Subtotal float64
	Member   string
}

type orderQuote struct {
	OrderID  string  `json:"order_id"`
	Member   string  `json:"member"`
	Subtotal float64 `json:"subtotal"`
	Discount float64 `json:"discount"`
	Shipping float64 `json:"shipping"`
	Total    float64 `json:"total"`
}

type lineItem struct {
	SKU       string  `json:"sku"`
	Qty       int     `json:"qty"`
	UnitPrice float64 `json:"unit_price"`
	LineTotal float64 `json:"line_total"`
}

type leaderboardItem struct {
	Member string `json:"member"`
	Points int    `json:"points"`
}

func (h *Handler) handlePracticeConcurrency(w http.ResponseWriter, r *http.Request) {
	// 并发示例：模拟批量订单报价计算，通过 goroutine 并行计算折扣和运费。
	// 响应字段：
	// - quotes: 订单报价明细（包含折扣、运费、总价）
	// - workers: 启动的 goroutine 数量
	orders := []orderInput{
		{OrderID: "ORD-1001", Subtotal: 120, Member: "silver"},
		{OrderID: "ORD-1002", Subtotal: 45, Member: "guest"},
		{OrderID: "ORD-1003", Subtotal: 260, Member: "gold"},
	}

	type result struct {
		Index int
		Quote orderQuote
	}

	results := make(chan result, len(orders))
	var wg sync.WaitGroup
	for i, order := range orders {
		wg.Add(1)
		go func(idx int, input orderInput) {
			defer wg.Done()
			discountRate := memberDiscountRate(input.Member)
			discount := roundTwo(input.Subtotal * discountRate)
			shipping := shippingFee(input.Subtotal)
			total := roundTwo(input.Subtotal - discount + shipping)
			results <- result{
				Index: idx,
				Quote: orderQuote{
					OrderID:  input.OrderID,
					Member:   input.Member,
					Subtotal: input.Subtotal,
					Discount: discount,
					Shipping: shipping,
					Total:    total,
				},
			}
		}(i, order)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	quotes := make([]orderQuote, len(orders))
	for res := range results {
		quotes[res.Index] = res.Quote
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"quotes":  quotes,
		"workers": len(orders),
	})
}

func (h *Handler) handlePracticeInterface(w http.ResponseWriter, r *http.Request) {
	// 接口示例：支付方式通过接口抽象，统一计算手续费。
	// 响应字段：
	// - amount: 订单金额
	// - quotes: 不同支付方式的手续费与最终金额
	amount := 200.0
	methods := []paymentMethod{
		cardPayment{Rate: 0.02, Fixed: 1},
		walletPayment{Rate: 0.01},
		bankTransfer{FeeAmount: 3},
	}

	quotes := make([]paymentQuote, 0, len(methods))
	for _, method := range methods {
		fee := roundTwo(method.Fee(amount))
		quotes = append(quotes, paymentQuote{
			Method:      method.Name(),
			Fee:         fee,
			FinalAmount: roundTwo(amount + fee),
		})
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"amount": amount,
		"quotes": quotes,
	})
}

func (h *Handler) handlePracticeRange(w http.ResponseWriter, r *http.Request) {
	// range 示例：遍历订单行项目累计数量和金额，同时拆分优惠码字符。
	// 响应字段：
	// - items: 订单行项目列表（含小计）
	// - total_qty: 商品总数量
	// - total_amount: 订单总金额
	// - coupon_letters: 优惠码拆分结果
	items := []struct {
		SKU       string
		Qty       int
		UnitPrice float64
	}{
		{SKU: "SKU-1001", Qty: 2, UnitPrice: 19.9},
		{SKU: "SKU-2002", Qty: 1, UnitPrice: 49.5},
		{SKU: "SKU-3003", Qty: 3, UnitPrice: 9.9},
	}

	summaries := make([]lineItem, 0, len(items))
	totalQty := 0
	totalAmount := 0.0
	for _, item := range items {
		lineTotal := roundTwo(float64(item.Qty) * item.UnitPrice)
		summaries = append(summaries, lineItem{
			SKU:       item.SKU,
			Qty:       item.Qty,
			UnitPrice: item.UnitPrice,
			LineTotal: lineTotal,
		})
		totalQty += item.Qty
		totalAmount += lineTotal
	}

	coupon := "SAVE10"
	letters := make([]string, 0, len(coupon))
	for _, r := range coupon {
		letters = append(letters, string(r))
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"items":          summaries,
		"total_qty":      totalQty,
		"total_amount":   roundTwo(totalAmount),
		"coupon_letters": letters,
	})
}

func (h *Handler) handlePracticeSlice(w http.ResponseWriter, r *http.Request) {
	// slice 示例：订单队列追加、分页截取与拣货清单 copy。
	// 响应字段：
	// - pending_orders/queued_orders: 原始与追加后的订单队列
	// - dashboard_page: 运营后台分页展示的订单
	// - pick_list/copy_count: copy 后的拣货清单
	// - lengths/capacities: 切片长度与容量对比
	pendingOrders := []string{"ORD-2001", "ORD-2002", "ORD-2003"}
	queuedOrders := append(pendingOrders, "ORD-2004", "ORD-2005")
	dashboardPage := queuedOrders[:2]

	pickList := make([]string, 2)
	copyCount := copy(pickList, queuedOrders)

	h.writeJSON(w, http.StatusOK, map[string]any{
		"pending_orders": pendingOrders,
		"queued_orders":  queuedOrders,
		"dashboard_page": dashboardPage,
		"pick_list":      pickList,
		"copy_count":     copyCount,
		"lengths": map[string]int{
			"pending": len(pendingOrders),
			"queued":  len(queuedOrders),
			"page":    len(dashboardPage),
		},
		"capacities": map[string]int{
			"pending": cap(pendingOrders),
			"queued":  cap(queuedOrders),
			"page":    cap(dashboardPage),
		},
	})
}

func (h *Handler) handlePracticeMap(w http.ResponseWriter, r *http.Request) {
	// map 示例：会员积分的增删查与排序输出排行榜。
	// 响应字段：
	// - leaderboard: 按会员名排序的积分列表
	// - lookup/missing: 命中与未命中的查找结果
	// - count: 当前会员数量
	loyaltyPoints := map[string]int{
		"alice": 1200,
		"bob":   800,
		"carol": 1500,
	}
	loyaltyPoints["dave"] = 950
	delete(loyaltyPoints, "bob")

	keys := make([]string, 0, len(loyaltyPoints))
	for k := range loyaltyPoints {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	leaderboard := make([]leaderboardItem, 0, len(keys))
	for _, k := range keys {
		leaderboard = append(leaderboard, leaderboardItem{Member: k, Points: loyaltyPoints[k]})
	}

	value, ok := loyaltyPoints["carol"]
	missingValue, missingOK := loyaltyPoints["zoe"]

	h.writeJSON(w, http.StatusOK, map[string]any{
		"leaderboard": leaderboard,
		"lookup": map[string]any{
			"member": "carol",
			"points": value,
			"ok":     ok,
		},
		"missing": map[string]any{
			"member": "zoe",
			"points": missingValue,
			"ok":     missingOK,
		},
		"count": len(loyaltyPoints),
	})
}

func memberDiscountRate(member string) float64 {
	switch member {
	case "gold":
		return 0.10
	case "silver":
		return 0.05
	default:
		return 0
	}
}

func shippingFee(subtotal float64) float64 {
	if subtotal >= 100 {
		return 0
	}
	return 12
}

func roundTwo(value float64) float64 {
	// 保留两位小数，避免浮点数直接输出时过长。
	return math.Round(value*100) / 100
}
