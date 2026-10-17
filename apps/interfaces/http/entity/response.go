package entity

// Rsp 是 HTTP 统一响应外层，业务数据由各接口的 Proto 消息转换而来。
// Code 为 ok 表示成功；失败时 Data 仍可承载已经完成的部分结果。
type Rsp[T any] struct {
	Code    string `json:"code"`
	Data    T      `json:"data"`
	TraceID string `json:"traceID"` // 来自实际 HTTP 请求链路，未建立链路时为空。
	Msg     string `json:"msg"`
}
