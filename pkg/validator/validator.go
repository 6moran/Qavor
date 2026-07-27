package validator

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"Qavor/pkg/errors"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Validator 自定义验证器
type Validator struct {
	validate *validator.Validate
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message"`
}

// 全局验证器实例
var (
	globalValidator *Validator
)

func init() {
	globalValidator = NewWithBindingTag()
}

// New 创建新的验证器（使用 validate 标签）
func New() *Validator {
	v := &Validator{
		validate: validator.New(),
	}

	// 注册JSON标签作为字段名
	v.validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name == "" {
			name = strings.SplitN(fld.Tag.Get("form"), ",", 2)[0]
		}
		return name
	})

	// 注册自定义验证器
	v.registerCustomValidators()

	// 注册自定义标签名称
	v.registerCustomTagNames()

	return v
}

// NewWithBindingTag 创建支持binding标签的验证器（与Gin兼容）
func NewWithBindingTag() *Validator {
	v := &Validator{
		validate: validator.New(),
	}

	// 设置标签名为 binding，与Gin保持一致
	v.validate.SetTagName("binding")

	// 注册JSON标签作为字段名
	v.validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name == "" {
			name = strings.SplitN(fld.Tag.Get("form"), ",", 2)[0]
		}
		return name
	})

	// 注册自定义验证器
	v.registerCustomValidators()

	// 注册自定义标签名称
	v.registerCustomTagNames()

	return v
}

// GetValidator 获取全局验证器
func GetValidator() *Validator {
	return globalValidator
}

// registerCustomValidators 注册自定义验证规则
func (v *Validator) registerCustomValidators() {
	// 手机号验证
	_ = v.validate.RegisterValidation("mobile", func(fl validator.FieldLevel) bool {
		phone := fl.Field().String()
		if phone == "" {
			return true // 空值由required标签处理
		}
		// 简单的手机号格式验证（支持中国手机号）
		return len(phone) == 11 && strings.HasPrefix(phone, "1")
	})

	// 身份证号验证（简单格式验证）
	_ = v.validate.RegisterValidation("idcard", func(fl validator.FieldLevel) bool {
		idCard := fl.Field().String()
		if idCard == "" {
			return true
		}
		// 简单的身份证号格式验证
		length := len(idCard)
		return length == 18 || length == 15
	})

	// 用户名验证（字母、数字、下划线，4-20位）
	_ = v.validate.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		username := fl.Field().String()
		if username == "" {
			return true
		}
		if len(username) < 4 || len(username) > 20 {
			return false
		}
		for _, c := range username {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
				return false
			}
		}
		return true
	})

	// 密码强度验证（至少包含大小写字母和数字）
	_ = v.validate.RegisterValidation("password", func(fl validator.FieldLevel) bool {
		password := fl.Field().String()
		if password == "" {
			return true
		}
		if len(password) < 6 || len(password) > 50 {
			return false
		}
		hasUpper := false
		hasLower := false
		hasDigit := false
		for _, c := range password {
			switch {
			case c >= 'A' && c <= 'Z':
				hasUpper = true
			case c >= 'a' && c <= 'z':
				hasLower = true
			case c >= '0' && c <= '9':
				hasDigit = true
			}
		}
		return hasUpper && hasLower && hasDigit
	})

	// IP地址验证
	_ = v.validate.RegisterValidation("ip", func(fl validator.FieldLevel) bool {
		ip := fl.Field().String()
		if ip == "" {
			return true
		}
		parts := strings.Split(ip, ".")
		if len(parts) != 4 {
			return false
		}
		for _, part := range parts {
			if len(part) == 0 || len(part) > 3 {
				return false
			}
			for _, c := range part {
				if c < '0' || c > '9' {
					return false
				}
			}
			// 简单的范围检查
			if len(part) > 1 && part[0] == '0' {
				return false
			}
		}
		return true
	})

	// URL验证
	_ = v.validate.RegisterValidation("url", func(fl validator.FieldLevel) bool {
		url := fl.Field().String()
		if url == "" {
			return true
		}
		url = strings.ToLower(url)
		return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
	})

	// JSON数组格式验证
	_ = v.validate.RegisterValidation("json_array", func(fl validator.FieldLevel) bool {
		str := fl.Field().String()
		if str == "" {
			return true
		}
		str = strings.TrimSpace(str)
		return strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]")
	})
}

// registerCustomTagNames 注册自定义标签的错误消息
func (v *Validator) registerCustomTagNames() {
	// 可以在这里注册自定义的错误消息映射
}

// ValidateStruct 验证结构体
func (v *Validator) ValidateStruct(obj interface{}) error {
	return v.validate.Struct(obj)
}

// ValidateVar 验证单个变量
func (v *Validator) ValidateVar(field interface{}, tag string) error {
	return v.validate.Var(field, tag)
}

// translateValidationErrors 将验证错误转换为统一格式
func (v *Validator) translateValidationErrors(err error) []ValidationError {
	var validationErrors []ValidationError

	if errs, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range errs {
			validationError := ValidationError{
				Field: fieldError.Field(),
				Tag:   fieldError.Tag(),
				Value: fmt.Sprintf("%v", fieldError.Value()),
			}

			// 获取字段的JSON标签名称
			validationError.Field = v.getFieldName(fieldError)

			// 生成友好的错误消息
			validationError.Message = v.getErrorMessage(fieldError)

			validationErrors = append(validationErrors, validationError)
		}
	}

	return validationErrors
}

// getFieldName 获取字段的JSON标签名称
func (v *Validator) getFieldName(fieldError validator.FieldError) string {
	// 尝试获取JSON标签
	if fieldError.Field() != "" {
		t := reflect.TypeOf(fieldError.Value())
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() == reflect.Struct {
			if field, ok := t.FieldByName(fieldError.Field()); ok {
				jsonTag := field.Tag.Get("json")
				if jsonTag != "" && jsonTag != "-" {
					// 移除标签中的选项（如binding:"required"）
					if idx := strings.Index(jsonTag, ","); idx != -1 {
						jsonTag = jsonTag[:idx]
					}
					return jsonTag
				}
				formTag := field.Tag.Get("form")
				if formTag != "" && formTag != "-" {
					return formTag
				}
			}
		}
	}
	return fieldError.Field()
}

// getErrorMessage 获取错误消息
func (v *Validator) getErrorMessage(fieldError validator.FieldError) string {
	field := v.getFieldName(fieldError)

	switch fieldError.Tag() {
	case "required":
		return fmt.Sprintf("%s 是必填字段", field)
	case "email":
		return fmt.Sprintf("%s 必须是有效的邮箱地址", field)
	case "min":
		return fmt.Sprintf("%s 长度不能少于 %s", field, fieldError.Param())
	case "max":
		return fmt.Sprintf("%s 长度不能超过 %s", field, fieldError.Param())
	case "len":
		return fmt.Sprintf("%s 长度必须为 %s", field, fieldError.Param())
	case "eq":
		return fmt.Sprintf("%s 必须等于 %s", field, fieldError.Param())
	case "ne":
		return fmt.Sprintf("%s 不能等于 %s", field, fieldError.Param())
	case "gt":
		return fmt.Sprintf("%s 必须大于 %s", field, fieldError.Param())
	case "gte":
		return fmt.Sprintf("%s 必须大于等于 %s", field, fieldError.Param())
	case "lt":
		return fmt.Sprintf("%s 必须小于 %s", field, fieldError.Param())
	case "lte":
		return fmt.Sprintf("%s 必须小于等于 %s", field, fieldError.Param())
	case "oneof":
		return fmt.Sprintf("%s 必须是以下值之一: %s", field, fieldError.Param())
	case "mobile":
		return fmt.Sprintf("%s 必须是有效的手机号", field)
	case "idcard":
		return fmt.Sprintf("%s 必须是有效的身份证号", field)
	case "username":
		return fmt.Sprintf("%s 必须是4-20位的字母、数字或下划线", field)
	case "password":
		return fmt.Sprintf("%s 必须包含大小写字母和数字，长度在6-50位之间", field)
	case "ip":
		return fmt.Sprintf("%s 必须是有效的IP地址", field)
	case "url":
		return fmt.Sprintf("%s 必须是有效的URL", field)
	case "json_array":
		return fmt.Sprintf("%s 必须是有效的JSON数组格式", field)
	default:
		return fmt.Sprintf("%s 验证失败", field)
	}
}

// BindAndValidate 绑定并验证请求参数
func BindAndValidate(c *gin.Context, obj interface{}) error {
	// 根据请求方法选择绑定方式
	var err error
	switch c.Request.Method {
	case http.MethodGet, http.MethodDelete:
		err = c.ShouldBindQuery(obj)
	default:
		err = c.ShouldBindJSON(obj)
	}

	if err != nil {
		return err
	}

	// 执行验证
	return globalValidator.ValidateStruct(obj)
}

// BindJSONAndValidate 绑定JSON并验证
func BindJSONAndValidate(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindJSON(obj); err != nil {
		return err
	}
	return globalValidator.ValidateStruct(obj)
}

// BindQueryAndValidate 绑定Query并验证
func BindQueryAndValidate(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindQuery(obj); err != nil {
		return err
	}
	return globalValidator.ValidateStruct(obj)
}

// BindURIAndValidate 绑定URI并验证
func BindURIAndValidate(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindUri(obj); err != nil {
		return err
	}
	return globalValidator.ValidateStruct(obj)
}

// RespondWithError 统一错误响应
func RespondWithError(c *gin.Context, err error) {
	// 检查是否是业务错误
	if _, ok := err.(*errors.BizError); ok {
		response.BizError(c, err)
		return
	}

	// 检查是否是验证错误
	validationErrors := globalValidator.translateValidationErrors(err)
	if len(validationErrors) > 0 {
		response.ErrorWithData(c, errors.CodeInvalidParam, "参数验证失败", validationErrors)
		return
	}

	// 其他错误
	response.InternalError(c, "服务器内部错误")
}

// ValidateAndRespond 验证参数并响应错误
func ValidateAndRespond(c *gin.Context, obj interface{}) bool {
	if err := globalValidator.ValidateStruct(obj); err != nil {
		RespondWithError(c, err)
		return false
	}
	return true
}

// GetValidator 获取全局验证器实例（用于外部调用）
func GetInstance() *validator.Validate {
	return globalValidator.validate
}
