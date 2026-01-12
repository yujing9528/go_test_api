package todo

import (
	"encoding/json"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func newPracticeTestRouter() http.Handler {
	logger := log.New(io.Discard, "", 0)
	handler := NewHandler(nil, logger)
	return handler.Routes()
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func assertFloatApprox(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("unexpected float value: got %.2f want %.2f", got, want)
	}
}

func TestPracticeConcurrency(t *testing.T) {
	router := newPracticeTestRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/practice/concurrency", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var res struct {
		Quotes  []orderQuote `json:"quotes"`
		Workers int          `json:"workers"`
	}
	decodeResponse(t, rec, &res)

	if res.Workers != 3 {
		t.Fatalf("unexpected workers: %d", res.Workers)
	}

	expected := []orderQuote{
		{OrderID: "ORD-1001", Member: "silver", Subtotal: 120, Discount: 6, Shipping: 0, Total: 114},
		{OrderID: "ORD-1002", Member: "guest", Subtotal: 45, Discount: 0, Shipping: 12, Total: 57},
		{OrderID: "ORD-1003", Member: "gold", Subtotal: 260, Discount: 26, Shipping: 0, Total: 234},
	}

	if len(res.Quotes) != len(expected) {
		t.Fatalf("unexpected quotes length: %d", len(res.Quotes))
	}

	for i, want := range expected {
		got := res.Quotes[i]
		if got.OrderID != want.OrderID || got.Member != want.Member {
			t.Fatalf("unexpected quote identity: %#v", got)
		}
		assertFloatApprox(t, got.Subtotal, want.Subtotal)
		assertFloatApprox(t, got.Discount, want.Discount)
		assertFloatApprox(t, got.Shipping, want.Shipping)
		assertFloatApprox(t, got.Total, want.Total)
	}
}

func TestPracticeInterface(t *testing.T) {
	router := newPracticeTestRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/practice/interface", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var res struct {
		Amount float64        `json:"amount"`
		Quotes []paymentQuote `json:"quotes"`
	}
	decodeResponse(t, rec, &res)

	assertFloatApprox(t, res.Amount, 200)

	expected := []paymentQuote{
		{Method: "card", Fee: 5, FinalAmount: 205},
		{Method: "wallet", Fee: 2, FinalAmount: 202},
		{Method: "bank_transfer", Fee: 3, FinalAmount: 203},
	}

	if len(res.Quotes) != len(expected) {
		t.Fatalf("unexpected quotes length: %d", len(res.Quotes))
	}

	for i, want := range expected {
		got := res.Quotes[i]
		if got.Method != want.Method {
			t.Fatalf("unexpected method: %s", got.Method)
		}
		assertFloatApprox(t, got.Fee, want.Fee)
		assertFloatApprox(t, got.FinalAmount, want.FinalAmount)
	}
}

func TestPracticeRange(t *testing.T) {
	router := newPracticeTestRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/practice/range", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var res struct {
		Items         []lineItem `json:"items"`
		TotalQty      int        `json:"total_qty"`
		TotalAmount   float64    `json:"total_amount"`
		CouponLetters []string   `json:"coupon_letters"`
	}
	decodeResponse(t, rec, &res)

	expectedItems := []lineItem{
		{SKU: "SKU-1001", Qty: 2, UnitPrice: 19.9, LineTotal: 39.8},
		{SKU: "SKU-2002", Qty: 1, UnitPrice: 49.5, LineTotal: 49.5},
		{SKU: "SKU-3003", Qty: 3, UnitPrice: 9.9, LineTotal: 29.7},
	}

	if len(res.Items) != len(expectedItems) {
		t.Fatalf("unexpected items length: %d", len(res.Items))
	}

	for i, want := range expectedItems {
		got := res.Items[i]
		if got.SKU != want.SKU || got.Qty != want.Qty {
			t.Fatalf("unexpected item identity: %#v", got)
		}
		assertFloatApprox(t, got.UnitPrice, want.UnitPrice)
		assertFloatApprox(t, got.LineTotal, want.LineTotal)
	}

	if res.TotalQty != 6 {
		t.Fatalf("unexpected total qty: %d", res.TotalQty)
	}
	assertFloatApprox(t, res.TotalAmount, 119.0)

	if strings.Join(res.CouponLetters, "") != "SAVE10" {
		t.Fatalf("unexpected coupon letters: %#v", res.CouponLetters)
	}
}

func TestPracticeSlice(t *testing.T) {
	router := newPracticeTestRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/practice/slice", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var res struct {
		PendingOrders []string       `json:"pending_orders"`
		QueuedOrders  []string       `json:"queued_orders"`
		DashboardPage []string       `json:"dashboard_page"`
		PickList      []string       `json:"pick_list"`
		CopyCount     int            `json:"copy_count"`
		Lengths       map[string]int `json:"lengths"`
		Capacities    map[string]int `json:"capacities"`
	}
	decodeResponse(t, rec, &res)

	if !reflect.DeepEqual(res.PendingOrders, []string{"ORD-2001", "ORD-2002", "ORD-2003"}) {
		t.Fatalf("unexpected pending orders: %#v", res.PendingOrders)
	}
	if !reflect.DeepEqual(res.QueuedOrders, []string{"ORD-2001", "ORD-2002", "ORD-2003", "ORD-2004", "ORD-2005"}) {
		t.Fatalf("unexpected queued orders: %#v", res.QueuedOrders)
	}
	if !reflect.DeepEqual(res.DashboardPage, []string{"ORD-2001", "ORD-2002"}) {
		t.Fatalf("unexpected dashboard page: %#v", res.DashboardPage)
	}
	if !reflect.DeepEqual(res.PickList, []string{"ORD-2001", "ORD-2002"}) {
		t.Fatalf("unexpected pick list: %#v", res.PickList)
	}
	if res.CopyCount != 2 {
		t.Fatalf("unexpected copy count: %d", res.CopyCount)
	}
	if res.Lengths["pending"] != 3 || res.Lengths["queued"] != 5 || res.Lengths["page"] != 2 {
		t.Fatalf("unexpected lengths: %#v", res.Lengths)
	}

	capPending, ok := res.Capacities["pending"]
	if !ok || capPending != 3 {
		t.Fatalf("unexpected base capacity: %#v", res.Capacities)
	}
	capQueued, ok := res.Capacities["queued"]
	if !ok || capQueued < res.Lengths["queued"] {
		t.Fatalf("unexpected extended capacity: %#v", res.Capacities)
	}
	capPage, ok := res.Capacities["page"]
	if !ok || capPage < res.Lengths["page"] {
		t.Fatalf("unexpected subslice capacity: %#v", res.Capacities)
	}
}

func TestPracticeMap(t *testing.T) {
	router := newPracticeTestRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/practice/map", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var res struct {
		Leaderboard []leaderboardItem `json:"leaderboard"`
		Lookup      struct {
			Member string `json:"member"`
			Points int    `json:"points"`
			Ok     bool   `json:"ok"`
		} `json:"lookup"`
		Missing struct {
			Member string `json:"member"`
			Points int    `json:"points"`
			Ok     bool   `json:"ok"`
		} `json:"missing"`
		Count int `json:"count"`
	}
	decodeResponse(t, rec, &res)

	expectedItems := []leaderboardItem{
		{Member: "alice", Points: 1200},
		{Member: "carol", Points: 1500},
		{Member: "dave", Points: 950},
	}
	if !reflect.DeepEqual(res.Leaderboard, expectedItems) {
		t.Fatalf("unexpected leaderboard: %#v", res.Leaderboard)
	}
	if res.Lookup.Member != "carol" || res.Lookup.Points != 1500 || !res.Lookup.Ok {
		t.Fatalf("unexpected lookup: %#v", res.Lookup)
	}
	if res.Missing.Member != "zoe" || res.Missing.Points != 0 || res.Missing.Ok {
		t.Fatalf("unexpected missing: %#v", res.Missing)
	}
	if res.Count != 3 {
		t.Fatalf("unexpected count: %d", res.Count)
	}
}
