<template>
  <div class="login-page">
    <div class="login-container">
      <div class="login-card">
        <!-- 左侧品牌展示区 -->
        <div class="brand-section">
          <div class="brand-header">
            <div class="brand-logo">
              <img src="/qavor-logo.png" alt="QAVOR Logo" />
            </div>
            <span class="brand-name">QAVOR</span>
          </div>

          <div class="brand-content">
            <h1>欢迎使用 QAVOR</h1>
            <p>你的专属智能助手，随时为你思考、回答与执行。</p>
          </div>

            <div class="brand-illustration">
              <div class="illustration-placeholder">
                <div class="main-brand-slogan">连接知识，激发智能</div>
              </div>
            </div>

          <div class="brand-footer">
            <p>© 2025 QAVOR · All rights reserved</p>
          </div>
        </div>

        <!-- 右侧登录表单区 -->
        <div class="form-section">
          <div class="form-wrapper">
            <div class="form-logo">
              <img src="/qavor-logo.png" alt="QAVOR Logo" />
            </div>

            <h2>登录</h2>
            <p class="subtitle">登录到你的个人智能助手</p>

            <a-form layout="vertical" :model="form" @finish="submit" class="login-form">
              <a-form-item
                label="账号"
                name="username"
                :rules="[{ required: true, message: '请输入账号' }]"
              >
                <a-input
                  v-model:value="form.username"
                  autocomplete="username"
                  placeholder="输入账号"
                  size="large"
                >
                  <template #prefix>
                    <UserOutlined />
                  </template>
                </a-input>
              </a-form-item>

              <a-form-item
                label="密码"
                name="password"
                :rules="[{ required: true, message: '请输入密码' }]"
              >
                <a-input-password
                  v-model:value="form.password"
                  autocomplete="current-password"
                  placeholder="输入密码"
                  size="large"
                >
                  <template #prefix>
                    <LockOutlined />
                  </template>
                </a-input-password>
              </a-form-item>

              <a-button
                type="primary"
                html-type="submit"
                block
                size="large"
                :loading="submitting"
                class="login-btn"
              >
                登录
              </a-button>
            </a-form>

            <p class="hint">账号和密码由管理员在配置文件中设置</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useRoute, useRouter } from 'vue-router'
import { UserOutlined, LockOutlined } from '@ant-design/icons-vue'

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
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fff;
  padding: 40px;
}

.login-container {
  width: 100%;
  max-width: 1100px;
}

.login-card {
  display: flex;
  min-height: 640px;
  border-radius: 20px;
  overflow: hidden;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.25);
}

/* 左侧品牌展示区 */
.brand-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 56px 64px;
  background: #f1f5f9;
  position: relative;
}

.brand-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.brand-logo {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  overflow: hidden;
}

.brand-logo img {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  object-fit: cover;
}

.brand-name {
  font-size: 20px;
  font-weight: 600;
  color: #1e293b;
}

.brand-content {
  margin-top: 80px;
  max-width: 400px;
}

.brand-content h1 {
  margin: 0 0 14px;
  font-size: 38px;
  font-weight: 700;
  color: #0f172a;
  line-height: 1.2;
}

.brand-content p {
  margin: 0;
  font-size: 16px;
  color: #64748b;
  line-height: 1.6;
}

.brand-illustration {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.illustration-placeholder {
  position: relative;
  width: 280px;
  height: 280px;
}

.main-brand-slogan {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 280px;
  color: #4f46e5;
  font-size: 27px;
  font-weight: 600;
  letter-spacing: 0.14em;
  line-height: 1.6;
  text-align: center;
  white-space: nowrap;
  text-shadow: 0 10px 30px rgba(99, 102, 241, 0.22);
}

.brand-footer {
  margin-top: auto;
}

.brand-footer p {
  margin: 0;
  font-size: 13px;
  color: #94a3b8;
}

/* 右侧登录表单区 */
.form-section {
  width: 480px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 56px 64px;
  background: #fff;
}

.form-wrapper {
  width: 100%;
  max-width: 360px;
}

.form-logo {
  width: 64px;
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  overflow: hidden;
  margin: 0 auto 24px;
}

.form-logo img {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  object-fit: cover;
}

h2 {
  margin: 0 0 8px;
  font-size: 26px;
  font-weight: 600;
  color: #0f172a;
  text-align: center;
}

.subtitle {
  margin: 0 0 36px;
  font-size: 14px;
  color: #64748b;
  text-align: center;
}

.login-form {
  margin-bottom: 20px;
}

:deep(.ant-form-item) {
  margin-bottom: 20px;
}

:deep(.ant-form-item-label > label) {
  font-weight: 500;
  color: #334155 !important;
  font-size: 14px !important;
  height: 22px !important;
}

:deep(.ant-form-item-label) {
  padding-bottom: 6px !important;
}

:deep(.ant-input-affix-wrapper) {
  border-radius: 8px !important;
  border: 1px solid #e2e8f0 !important;
  padding: 10px 14px !important;
  height: 44px !important;
  background: #fff !important;
}

:deep(.ant-input-affix-wrapper:hover) {
  border-color: #cbd5e1 !important;
}

:deep(.ant-input-affix-wrapper-focused) {
  border-color: #6366f1 !important;
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.1) !important;
}

:deep(.ant-input-prefix) {
  color: #94a3b8 !important;
  margin-right: 10px !important;
  font-size: 16px !important;
}

:deep(.ant-input) {
  font-size: 14px !important;
  color: #1e293b !important;
}

:deep(.ant-input::placeholder) {
  color: #cbd5e1 !important;
}

.login-btn {
  height: 44px !important;
  border-radius: 8px !important;
  font-size: 15px !important;
  font-weight: 500 !important;
  background: #6366f1 !important;
  border-color: #6366f1 !important;
  margin-top: 8px !important;
}

.login-btn:hover {
  background: #4f46e5 !important;
  border-color: #4f46e5 !important;
}

.hint {
  margin: 0;
  font-size: 12px;
  color: #94a3b8;
  text-align: center;
}

/* 响应式适配 */
@media (max-width: 960px) {
  .brand-section {
    display: none;
  }

  .form-section {
    width: 100%;
  }
}
</style>
