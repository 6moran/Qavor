package email

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"Qavor/pkg/config"
	"Qavor/pkg/logger"
	"go.uber.org/zap"
)

// SMTPClient SMTP 客户端
type SMTPClient struct {
	config *config.EmailConfig
}

// NewSMTPClient 创建 SMTP 客户端
// 参数:
//   - cfg: 邮件配置，包含 SMTP 服务器信息和发件人信息
//
// 返回:
//   - *SMTPClient: SMTP 客户端实例
func NewSMTPClient(cfg *config.EmailConfig) *SMTPClient {
	return &SMTPClient{config: cfg}
}

// Send 发送邮件
// 参数:
//   - to: 收件人邮箱列表
//   - subject: 邮件主题
//   - body: 邮件正文（HTML 格式）
//
// 返回:
//   - error: 发送失败时返回错误
func (c *SMTPClient) Send(to []string, subject, body string) error {
	if c.config.SMTP.Host == "" || c.config.SMTP.Port == 0 {
		logger.Warn("SMTP 未配置，跳过邮件发送")
		return nil
	}

	// 构建邮件内容
	msg := c.buildMessage(to, subject, body)

	// SMTP 认证
	auth := smtp.PlainAuth("", c.config.SMTP.Account, c.config.SMTP.AuthCode, c.config.SMTP.Host)

	// SMTP 服务器地址
	addr := fmt.Sprintf("%s:%d", c.config.SMTP.Host, c.config.SMTP.Port)

	// 发送邮件
	err := c.sendWithTLS(addr, auth, c.config.SMTP.Account, to, msg)
	if err != nil {
		logger.Error("发送邮件失败", zap.Error(err))
		return fmt.Errorf("发送邮件失败: %w", err)
	}

	logger.Info("邮件发送成功", zap.Strings("to", to), zap.String("subject", subject))
	return nil
}

// sendWithTLS 使用 TLS 加密方式发送邮件
// 参数:
//   - addr: SMTP 服务器地址（host:port）
//   - auth: SMTP 认证信息
//   - from: 发件人邮箱
//   - to: 收件人邮箱列表
//   - msg: 邮件内容
//
// 返回:
//   - error: 发送失败时返回错误
func (c *SMTPClient) sendWithTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	// 连接 SMTP 服务器
	conn, connErr := tls.Dial("tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
	})
	if connErr != nil {
		return fmt.Errorf("连接 SMTP 服务器失败: %w", connErr)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Warn("关闭 SMTP 连接失败", zap.Error(err))
		}
	}()

	// 创建 SMTP 客户端
	client, clientErr := smtp.NewClient(conn, c.config.SMTP.Host)
	if clientErr != nil {
		return fmt.Errorf("创建 SMTP 客户端失败: %w", clientErr)
	}
	defer func() {
		if err := client.Close(); err != nil {
			logger.Warn("关闭 SMTP 客户端失败", zap.Error(err))
		}
	}()

	// 认证
	if authErr := client.Auth(auth); authErr != nil {
		return fmt.Errorf("SMTP 认证失败: %w", authErr)
	}

	// 设置发件人
	if mailErr := client.Mail(from); mailErr != nil {
		return fmt.Errorf("设置发件人失败: %w", mailErr)
	}

	// 设置收件人
	for _, addr := range to {
		if rcptErr := client.Rcpt(addr); rcptErr != nil {
			return fmt.Errorf("设置收件人失败: %w", rcptErr)
		}
	}

	// 发送邮件内容
	writer, dataErr := client.Data()
	if dataErr != nil {
		return fmt.Errorf("获取邮件写入器失败: %w", dataErr)
	}

	if _, writeErr := writer.Write(msg); writeErr != nil {
		return fmt.Errorf("写入邮件内容失败: %w", writeErr)
	}

	if closeErr := writer.Close(); closeErr != nil {
		return fmt.Errorf("关闭邮件写入器失败: %w", closeErr)
	}

	// 退出
	return client.Quit()
}

// buildMessage 构建邮件内容
// 参数:
//   - to: 收件人邮箱列表
//   - subject: 邮件主题
//   - body: 邮件正文（HTML 格式）
//
// 返回:
//   - []byte: 格式化后的邮件内容
func (c *SMTPClient) buildMessage(to []string, subject, body string) []byte {
	header := make(map[string]string)
	header["From"] = fmt.Sprintf("%s <%s>", c.config.FromName, c.config.From)
	header["To"] = strings.Join(to, ",")
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=UTF-8"

	var msg strings.Builder
	for k, v := range header {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return []byte(msg.String())
}
