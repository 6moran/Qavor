<script setup>
import zhCN from 'ant-design-vue/es/locale/zh_CN'
import { useThemeStore } from '@/stores/theme'

const themeStore = useThemeStore()
</script>
<template>
  <a-config-provider :theme="themeStore.currentTheme" :locale="zhCN">
    <router-view />
  </a-config-provider>
</template>
