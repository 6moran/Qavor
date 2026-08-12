package builtin

import (
	"context"

	"Qavor/internal/tool"
)

// AskUserTool 向用户提问工具。
// 实际的中断/恢复逻辑在 ApprovalMiddleware 中处理，
// 这个工具仅提供元数据描述，Execute 恒返回空。
type AskUserTool struct{}

// Meta 返回工具元数据
func (t *AskUserTool) Meta() tool.ToolMeta {
	return tool.ToolMeta{
		Name:        tool.AskUserToolName,
		Label:       "询问用户",
		Description: "当你需要向用户澄清问题或获取更多信息时，使用此工具向用户提问。支持选择题、多选题和自由输入。",
		Category:    tool.CategorySystem,
		Args: []tool.ArgDef{
			{
				Name:        "questions",
				Type:        "array",
				Description: "需要用户回答的问题列表",
				Required:    true,
				ElemInfo: &tool.ArgDef{
					Name:        "question_item",
					Type:        "object",
					Description: "单个问题，包含 question_id（唯一标识）、question（问题内容）、options（可选项）、multi_select（是否多选）、allow_other（是否允许用户输入其他选项）",
					SubParams: map[string]*tool.ArgDef{
						"question_id": {
							Name:        "question_id",
							Type:        "string",
							Description: "问题唯一标识（如 q1、q2）",
							Required:    true,
						},
						"question": {
							Name:        "question",
							Type:        "string",
							Description: "问题内容",
							Required:    true,
						},
						"options": {
							Name:        "options",
							Type:        "array",
							Description: "可选项列表（填空类型的题目不提供此字段）",
							ElemInfo:    &tool.ArgDef{Name: "option", Type: "string", Description: "选项文本"},
						},
						"multi_select": {
							Name:        "multi_select",
							Type:        "boolean",
							Description: "是否允许多选（默认 false 表示单选）",
						},
						"allow_other": {
							Name:        "allow_other",
							Type:        "boolean",
							Description: "是否允许用户自由输入其他选项（默认 false）",
						},
					},
				},
			},
		},
		ConfigGuide: `questions 参数格式：
	[
	  {
	    "question_id": "q1",
	    "question": "你想要什么颜色？",
	    "options": ["红色", "蓝色", "绿色"],
	    "multi_select": false,
	    "allow_other": true
	  }
	]`,
	}
}

// Execute 执行 ask_user 工具。
// 实际逻辑通过 ApprovalMiddleware 的 StatefulInterrupt 机制处理，
// 此方法仅在同步测试路径中直接调用，生产中由中间件拦截。
func (t *AskUserTool) Execute(_ context.Context, _ map[string]any) (any, error) {
	return "", nil
}
