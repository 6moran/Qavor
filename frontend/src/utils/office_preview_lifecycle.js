export const createPreviewRenderGate = () => {
  let activeRunId = 0

  return {
    begin() {
      activeRunId += 1
      return activeRunId
    },
    invalidate() {
      activeRunId += 1
    },
    isCurrent(runId) {
      return runId === activeRunId
    }
  }
}
