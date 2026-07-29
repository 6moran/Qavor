<template>
  <div class="login-page">
    <section class="login-card">
      <div class="brand">
        <h1>登录 Qavor</h1>
        <p>使用部署配置中的管理员账号访问此实例</p>
      </div>

      <a-form layout="vertical" :model="form" @finish="submit">
        <a-form-item
          label="用户名"
          name="username"
          :rules="[{ required: true, message: '请输入用户名' }]"
        >
          <a-input v-model:value="form.username" autocomplete="username" placeholder="管理员用户名" />
        </a-form-item>
        <a-form-item
          label="密码"
          name="password"
          :rules="[{ required: true, message: '请输入密码' }]"
        >
          <a-input-password
            v-model:value="form.password"
            autocomplete="current-password"
            placeholder="管理员密码"
          />
        </a-form-item>
        <a-button type="primary" html-type="submit" block size="large" :loading="submitting">
          登录
        </a-button>
      </a-form>
    </section>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useRoute, useRouter } from 'vue-router'

import { useUserStore } from '@/stores/user'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const submitting = ref(false)
const form = reactive({ username: '', password: '' })

const safeRedirect = () => {
  const target = typeof route.query.redirect === 'string' ? route.query.redirect : '/agent'
  return target.startsWith('/') && !target.startsWith('//') ? target : '/agent'
}

const submit = async () => {
  submitting.value = true
  try {
    await userStore.login(form)
    await router.replace(safeRedirect())
  } catch (error) {
    message.error(error.message || '登录失败')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    linear-gradient(rgba(248, 250, 252, 0.88), rgba(248, 250, 252, 0.94)),
    url('/login-bg.jpg') center / cover;
}

.login-card {
  width: min(420px, 100%);
  padding: 36px;
  border: 1px solid var(--gray-150);
  border-radius: 16px;
  background: var(--gray-0);
  box-shadow: 0 18px 50px rgba(15, 23, 42, 0.12);
}

.brand {
  margin-bottom: 28px;
}

.brand h1 {
  margin: 0 0 8px;
  font-size: 26px;
  color: var(--gray-900);
}

.brand p {
  margin: 0;
  color: var(--gray-600);
}
</style>
