package entity

// Rsp 是 HTTP 统一响应外层，业务数据由各接口的 Proto 消息转换而来。
// Code 为 ok 表示成功；失败时 Data 仍可承载已经完成的部分结果。
type Rsp[T any] struct {
	Code    string `json:"code"`
	Data    T      `json:"data"`
	TraceID string `json:"traceID"`
	Msg     string `json:"msg"`
}
