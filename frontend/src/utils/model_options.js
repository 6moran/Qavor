const DEFAULT_MODEL_REMARK = '未填写备注'

export const buildModelSelectOptions = (models = []) =>
  models.map((model) => ({
    value: model.id,
    label: model.name || '',
    id: model.id,
    remark: model.remark?.trim() || DEFAULT_MODEL_REMARK
  }))
