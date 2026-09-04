import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'

/**
 * 让自定义 a-dropdown 浮层（overlay）的最小宽度与触发框保持一致，
 * 避免展开面板比触发框窄一截而显得不协调。
 *
 * 自定义下拉浮层是 portal 到 body 的，CSS 无法直接 100% 跟随触发框，
 * 因此这里在打开时测量触发框宽度，并用 ResizeObserver 跟随其变化。
 * 关闭过程中冻结宽度，防止 overlay 在消失动画期间因宽度重算而"先变小再消失"。
 *
 * @param {import('vue').Ref<boolean>} open 下拉框打开状态
 * @param {number} minWidth 面板最小宽度保底值（px）
 * @returns {{ triggerRef: import('vue').Ref, overlayStyle: import('vue').ComputedRef<{ minWidth: string }> }}
 */
export function useDropdownWidth(open, minWidth = 280) {
  const triggerRef = ref(null)
  const triggerWidth = ref(0)
  let resizeObserver = null

  const measure = () => {
    // 关闭过程中不再更新宽度，避免消失动画闪烁
    if (!open.value) return
    const el = triggerRef.value
    if (el) {
      triggerWidth.value = el.getBoundingClientRect().width
    }
  }

  onMounted(() => {
    if (open.value) measure()
    if (typeof ResizeObserver !== 'undefined' && triggerRef.value) {
      resizeObserver = new ResizeObserver(measure)
      resizeObserver.observe(triggerRef.value)
    }
  })

  onBeforeUnmount(() => {
    if (resizeObserver) {
      resizeObserver.disconnect()
      resizeObserver = null
    }
  })

  // 每次打开时立即重新测量；关闭时保留最后一次宽度，直到 overlay 卸载
  watch(open, (isOpen) => {
    if (isOpen) measure()
  })

  const overlayStyle = computed(() => ({
    minWidth: `${Math.max(triggerWidth.value || minWidth, minWidth)}px`
  }))

  return { triggerRef, overlayStyle }
}
