package validator

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

// ErrorHandler 自定义验证错误处理
func ErrorHandler(err error) (int, string) {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range validationErrors {
			field := fieldError.Field()
			tag := fieldError.Tag()

			// 将字段名转换为友好的中文名称
			fieldName := getFieldName(field)
			message := getErrorMessage(fieldName, tag)
			return http.StatusBadRequest, message
		}
	}

	// JSON 解析错误
	if strings.Contains(err.Error(), "cannot unmarshal") {
		return http.StatusBadRequest, "请求格式错误"
	}

	return http.StatusBadRequest, "请求参数无效"
}

// getFieldName 将字段名转换为友好的中文名称
func getFieldName(field string) string {
	fieldMap := map[string]string{
		"Password":        "密码",
		"Username":        "用户名",
		"ConfirmPassword": "确认密码",
		"Code":            "验证码",
		"NewPassword":     "新密码",
		"RefreshToken":    "刷新令牌",
		"AccessToken":     "访问令牌",
		"Phone":           "手机号",
		"Name":            "名称",
		"Title":           "标题",
		"Content":         "内容",
		"Description":     "描述",
	}

	if name, ok := fieldMap[field]; ok {
		return name
	}
	return field
}

// getErrorMessage 根据字段名和验证规则生成友好的错误信息
func getErrorMessage(field, tag string) string {
	switch tag {
	case "required":
		return fmt.Sprintf("%s不能为空", field)
	case "min":
		return fmt.Sprintf("%s长度不足", field)
	case "max":
		return fmt.Sprintf("%s长度超出限制", field)
	case "len":
		return fmt.Sprintf("%s长度必须为指定值", field)
	case "eqfield":
		return fmt.Sprintf("两次输入的%s不一致", field)
	case "gte":
		return fmt.Sprintf("%s必须大于等于指定值", field)
	case "lte":
		return fmt.Sprintf("%s必须小于等于指定值", field)
	case "gt":
		return fmt.Sprintf("%s必须大于指定值", field)
	case "lt":
		return fmt.Sprintf("%s必须小于指定值", field)
	case "oneof":
		return fmt.Sprintf("%s的值无效", field)
	case "alpha":
		return fmt.Sprintf("%s只能包含字母", field)
	case "alphanum":
		return fmt.Sprintf("%s只能包含字母和数字", field)
	case "numeric":
		return fmt.Sprintf("%s必须是数字", field)
	case "boolean":
		return fmt.Sprintf("%s必须是布尔值", field)
	case "url":
		return fmt.Sprintf("%s必须是有效的URL", field)
	case "uuid":
		return fmt.Sprintf("%s必须是有效的UUID", field)
	case "json":
		return fmt.Sprintf("%s必须是有效的JSON", field)
	default:
		return fmt.Sprintf("%s格式不正确", field)
	}
}
