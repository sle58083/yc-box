package model

// Response 是统一 API 响应信封（system_design.md §9.1）。
// 所有 handler 一律 c.JSON 该结构，禁止在 handler 内散落不同格式。
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// OK 构造成功响应。
func OK(data any) Response {
	return Response{Code: CodeOK, Message: "ok", Data: data}
}

// Err 构造错误响应。
func Err(code int, message string) Response {
	return Response{Code: code, Message: message}
}
