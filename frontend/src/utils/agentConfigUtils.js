export const DEFAULT_ALL_AGENT_RESOURCE_KINDS = Object.freeze([
  'tools',
  'knowledges',
  'mcps',
  'skills',
  'subagents'
])

export const MENTION_AGENT_RESOURCE_KINDS = Object.freeze([
  'knowledges',
  'mcps',
  'skills',
  'subagents'
])

export const isDefaultAllAgentResourceKind = (kind) =>
  DEFAULT_ALL_AGENT_RESOURCE_KINDS.includes(kind)

export const isMentionAgentResourceKind = (kind) => MENTION_AGENT_RESOURCE_KINDS.includes(kind)

export const getAgentConfigOptions = (item) => (Array.isArray(item?.options) ? item.options : [])

export const isSingleSelectAgentConfig = (item) =>
  getAgentConfigOptions(item).length > 0 && ['str', 'string', 'select'].includes(item?.type)

export const getAgentConfigOptionValue = (option) => {
  if (typeof option !== 'object' || option === null) return option
  return (
    option.key ||
    option.id ||
    option.value ||
    option.name ||
    option.db_id ||
    option.slug ||
    option.label
  )
}

export const getAgentConfigOptionLabel = (option) => {
  if (typeof option !== 'object' || option === null) return option
  return option.name || option.label || getAgentConfigOptionValue(option)
}

export const getAgentConfigOptionDescription = (option) =>
  typeof option === 'object' && option !== null ? option.description || '' : ''
