package agent

import (
	"fmt"
	"strings"
	"time"

	"Qavor/internal/tool"
)

const defaultAgentInstruction = "你是一个智能助手，可以根据用户的问题调用可用的工具来提供帮助。请用中文回答用户的问题。"

const knowledgeToolGuidanceMarker = "## 知识库工具使用规则"

const mainAgentCollaborationGuidance = "子智能体具备向用户提问的能力（通过 report_need_input 工具，而非 ask_user）。当任务适合委托且需要用户交互时，可以直接把完整任务交给子智能体。"

const subagentInputGuidance = "你可以使用 report_need_input 工具向用户提出问题或请求决策。收到回答后，应在最终输出中说明已向用户提问并基于回答继续处理。"

const knowledgeToolGuidance = knowledgeToolGuidanceMarker + `
你可以使用 query_kb 查询当前 Agent 绑定的知识库。
当回答依赖用户上传的文档、内部资料、当前项目或组织的专有事实，或者用户要求原文依据和引用时，使用 query_kb。
问候、闲聊、翻译、改写、用户已提供内容的总结、数学计算、通用编程知识，以及当前上下文足以回答的问题，通常不需要调用 query_kb。
检索结果为空或相关性不足时，应明确说明资料不足。查询没有实质变化时，不要重复调用 query_kb；只有查询被明显澄清、拆分或补充条件后才再次检索。`

func appendInstructionSection(base, section string) string {
	base = strings.TrimSpace(base)
	section = strings.TrimSpace(section)
	if base == "" {
		return section
	}
	if section == "" || strings.Contains(base, section) {
		return base
	}
	return base + "\n\n" + section
}

func baseInstruction(configured string) string {
	if strings.TrimSpace(configured) == "" {
		return defaultAgentInstruction
	}
	return strings.TrimSpace(configured)
}

func appendKnowledgeToolGuidance(instruction string, builtinToolNames []string) string {
	if !containsTool(builtinToolNames, tool.QueryKBToolName) || strings.Contains(instruction, knowledgeToolGuidanceMarker) {
		return instruction
	}
	return appendInstructionSection(instruction, knowledgeToolGuidance)
}

func buildMainAgentInstruction(configured string, builtinToolNames []string) string {
	instruction := appendInstructionSection(baseInstruction(configured), mainAgentCollaborationGuidance)
	return appendKnowledgeToolGuidance(instruction, builtinToolNames)
}

func buildSubagentInstruction(configured string, builtinToolNames []string, now time.Time) string {
	instruction := baseInstruction(configured)
	instruction = appendInstructionSection(instruction, fmt.Sprintf("当前日期时间：%s（时区：%s）", now.Format("2006-01-02 15:04:05"), now.Location().String()))
	instruction = appendInstructionSection(instruction, subagentInputGuidance)
	return appendKnowledgeToolGuidance(instruction, builtinToolNames)
}
