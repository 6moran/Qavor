package email

import (
	"strings"
	"testing"
)

func TestResetCodeEmail(t *testing.T) {
	code := "123456"
	expireMinutes := 10

	email := ResetCodeEmail(code, expireMinutes)

	if !strings.Contains(email, code) {
		t.Errorf("ResetCodeEmail should contain code %s", code)
	}

	if !strings.Contains(email, "10") {
		t.Error("ResetCodeEmail should contain expire minutes")
	}

	if !strings.Contains(email, "html") {
		t.Error("ResetCodeEmail should contain HTML content")
	}
}

func TestResetSuccessEmail(t *testing.T) {
	email := ResetSuccessEmail()

	if !strings.Contains(email, "密码重置成功") {
		t.Error("ResetSuccessEmail should contain success message")
	}

	if !strings.Contains(email, "html") {
		t.Error("ResetSuccessEmail should contain HTML content")
	}
}

func TestBuildResetCodeEmail(t *testing.T) {
	code := "654321"
	subject, body := BuildResetCodeEmail(code)

	if !strings.Contains(subject, "密码重置验证码") {
		t.Error("Subject should contain password reset code")
	}

	if !strings.Contains(body, code) {
		t.Error("Body should contain code")
	}
}

func TestBuildResetSuccessEmail(t *testing.T) {
	subject, body := BuildResetSuccessEmail()

	if !strings.Contains(subject, "密码重置成功") {
		t.Error("Subject should contain password reset success")
	}

	if !strings.Contains(body, "密码重置成功") {
		t.Error("Body should contain success message")
	}
}

func TestParseEmailList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single email",
			input:    "test@example.com",
			expected: []string{"test@example.com"},
		},
		{
			name:     "multiple emails",
			input:    "test1@example.com,test2@example.com",
			expected: []string{"test1@example.com", "test2@example.com"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseEmailList(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("ParseEmailList(%s) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("ParseEmailList(%s)[%d] = %s, want %s", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}
