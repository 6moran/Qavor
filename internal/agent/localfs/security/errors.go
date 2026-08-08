package security

import "errors"

// ErrDenied 标记所有安全策略拒绝。调用处应使用 fmt.Errorf("%w: <中文文案>", ErrDenied) 包装，中间件通过 errors.Is 识别。
var ErrDenied = errors.New("access denied by security policy")
