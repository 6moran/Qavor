package email

import (
	"fmt"
	"strings"
)

// ResetCodeEmail 生成验证码邮件 HTML 模板
// 参数:
//   - code: 验证码
//   - expireMinutes: 验证码有效期（分钟）
//
// 返回:
//   - string: HTML 格式的邮件内容
func ResetCodeEmail(code string, expireMinutes int) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>密码重置验证码</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .container {
            background-color: #f9f9f9;
            border-radius: 8px;
            padding: 30px;
            margin-top: 20px;
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .header h1 {
            color: #333;
            font-size: 24px;
            margin-bottom: 10px;
        }
        .code-container {
            background-color: #fff;
            border: 2px dashed #e0e0e0;
            border-radius: 8px;
            padding: 20px;
            text-align: center;
            margin: 30px 0;
        }
        .code {
            font-size: 36px;
            font-weight: bold;
            color: #007bff;
            letter-spacing: 8px;
            font-family: 'Courier New', monospace;
        }
        .info {
            color: #666;
            font-size: 14px;
            margin-top: 20px;
        }
        .warning {
            background-color: #fff3cd;
            border: 1px solid #ffeaa7;
            border-radius: 4px;
            padding: 15px;
            margin-top: 20px;
            color: #856404;
            font-size: 14px;
        }
        .footer {
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #eee;
            color: #999;
            font-size: 12px;
            text-align: center;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>密码重置验证码</h1>
        </div>

        <p>您好，</p>

        <p>您请求了密码重置。请使用以下验证码完成密码重置：</p>

        <div class="code-container">
            <div class="code">%s</div>
        </div>

        <div class="info">
            <p><strong>验证码有效期：</strong>%d 分钟</p>
            <p>如果您没有请求密码重置，请忽略此邮件。</p>
        </div>

        <div class="warning">
            <strong>安全提示：</strong>请勿将验证码分享给他人。如非本人操作，请立即修改密码。
        </div>

        <div class="footer">
            <p>此邮件由系统自动发送，请勿直接回复。</p>
            <p>© 2024 Qavor. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, code, expireMinutes)
}

// ResetSuccessEmail 生成密码重置成功邮件 HTML 模板
//
// 返回:
//   - string: HTML 格式的邮件内容
func ResetSuccessEmail() string {
	return `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>密码重置成功</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .container {
            background-color: #f9f9f9;
            border-radius: 8px;
            padding: 30px;
            margin-top: 20px;
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .header h1 {
            color: #28a745;
            font-size: 24px;
            margin-bottom: 10px;
        }
        .success-icon {
            font-size: 48px;
            margin-bottom: 20px;
        }
        .info {
            background-color: #d4edda;
            border: 1px solid #c3e6cb;
            border-radius: 4px;
            padding: 15px;
            margin: 20px 0;
            color: #155724;
            font-size: 14px;
        }
        .footer {
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #eee;
            color: #999;
            font-size: 12px;
            text-align: center;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="success-icon">✅</div>
            <h1>密码重置成功</h1>
        </div>

        <p>您好，</p>

        <p>您的密码已成功重置。</p>

        <div class="info">
            <strong>温馨提示：</strong>
            <ul>
                <li>请使用新密码登录系统</li>
                <li>建议定期修改密码以保障账户安全</li>
                <li>如非本人操作，请立即联系管理员</li>
            </ul>
        </div>

        <div class="footer">
            <p>此邮件由系统自动发送，请勿直接回复。</p>
            <p>© 2024 Qavor. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`
}

// BuildResetCodeEmail 构建验证码邮件
// 参数:
//   - code: 验证码
//
// 返回:
//   - string: 邮件主题
//   - string: 邮件正文（HTML 格式）
func BuildResetCodeEmail(code string) (string, string) {
	subject := "【Qavor】密码重置验证码"
	body := ResetCodeEmail(code, 10)
	return subject, body
}

// BuildResetSuccessEmail 构建密码重置成功邮件
//
// 返回:
//   - string: 邮件主题
//   - string: 邮件正文（HTML 格式）
func BuildResetSuccessEmail() (string, string) {
	subject := "【Qavor】密码重置成功通知"
	body := ResetSuccessEmail()
	return subject, body
}

// ParseEmailList 解析邮件列表
// 参数:
//   - emails: 逗号分隔的邮箱字符串
//
// 返回:
//   - []string: 邮箱列表
func ParseEmailList(emails string) []string {
	if emails == "" {
		return nil
	}
	return strings.Split(emails, ",")
}
