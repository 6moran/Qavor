package security

import "errors"

// ErrDenied 标记所有安全策略拒绝。调用处应使用 fmt.Errorf("%w: <中文文案>", ErrDenied) 包装，中间件通过 errors.Is 识别。
var ErrDenied = errors.New("access denied by security policy")

// SecurityBlockMarker 命令被安全策略黑名单拦截时，在工具结果文本中携带此标记。
// executor 检测到该标记即终止本轮 run，从结构上杜绝模型尝试绕过黑名单（不依赖模型自觉遵守）。
const SecurityBlockMarker = "[SECURITY_POLICY_BLOCKED]"
