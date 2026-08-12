package builtin

import (
	"context"

	"Qavor/internal/tool"
)

// ReportNeedInputTool 子智能体上报需要用户输入的工具。
// 子智能体在遇到信息不足、需要用户决策等情况时调用此工具。
// 实际的中断/恢复逻辑在 ApprovalMiddleware 中处理（与 ask_user 共享同一套机制），
// 中断通过 eino 中断树自动传播到主智能体。
type ReportNeedInputTool struct{}

// Meta 返回工具元数据
func (t *ReportNeedInputTool) Meta() tool.ToolMeta {
	return tool.ToolMeta{
		Name:        tool.ReportNeedInputToolName,
		Label:       "上报需要用户输入",
		Description: "当你执行任务时遇到信息不足、需要用户决策或确认才能继续的情况，使用此工具向上级报告问题。说明为什么需要用户介入、有哪些可选方案以及需要的决策类型。",
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
	    "question": "数据库类型是什么？",
	    "options": ["MySQL", "PostgreSQL", "SQLite"],
	    "multi_select": false,
	    "allow_other": true
	  }
	]`,
	}
}

// Execute 执行 report_need_input 工具。
// 实际逻辑通过 ApprovalMiddleware 的 StatefulInterrupt 机制处理，
// 此方法仅在同步测试路径中直接调用，生产中由中间件拦截。
func (t *ReportNeedInputTool) Execute(_ context.Context, _ map[string]any) (any, error) {
	return "", nil
}
