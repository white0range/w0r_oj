<template>
  <div class="auth-shell">
    <section class="auth-story">
      <span class="eyebrow">Login</span>
      <h1>回到你的判题台。</h1>
      <p>
        这里的登录状态会直接接上你的 JWT 后端鉴权。登录后即可查看个人中心、提交记录和管理员后台入口。
      </p>
      <div class="auth-points">
        <div class="auth-point">
          <strong>JWT</strong>
          <span>登录成功后自动解析角色与用户名</span>
        </div>
        <div class="auth-point">
          <strong>Admin</strong>
          <span>管理员账号会显示后台入口与独立路由守卫</span>
        </div>
      </div>
    </section>

    <section class="card auth-card">
      <div class="section-title">
        <h2>登录账号</h2>
      </div>

      <form class="stack" @submit.prevent="handleSubmit">
        <div class="field">
          <label for="username">用户名</label>
          <input id="username" v-model.trim="form.username" class="input" autocomplete="username" required />
        </div>

        <div class="field">
          <label for="password">密码</label>
          <input id="password" v-model="form.password" class="input" type="password" autocomplete="current-password" required />
        </div>

        <div v-if="error" class="auth-message auth-error">{{ error }}</div>

        <button class="btn btn-primary btn-block" type="submit" :disabled="loading">
          <span v-if="loading" class="spinner"></span>
          <span v-else>登录并进入工作台</span>
        </button>
      </form>

      <p class="auth-footer">
        还没有账号？
        <router-link to="/register">去注册</router-link>
      </p>
    </section>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { loginUser } from '../api'
import { store } from '../store'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const error = ref('')
const form = reactive({
  username: '',
  password: '',
})

async function handleSubmit() {
  loading.value = true
  error.value = ''

  try {
    const result = await loginUser(form)
    store.login(result.token)
    router.push((route.query.redirect || '/')?.toString())
  } catch (requestError) {
    error.value = requestError.response?.data?.error || '登录失败，请检查账号和密码。'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-shell {
  display: grid;
  grid-template-columns: 1.1fr minmax(320px, 430px);
  gap: 28px;
  align-items: stretch;
  min-height: calc(100vh - 220px);
}

.auth-story {
  padding: 34px;
  border-radius: var(--radius-lg);
  background:
    linear-gradient(145deg, rgba(20, 33, 61, 0.92), rgba(29, 122, 116, 0.88)),
    linear-gradient(180deg, rgba(255, 255, 255, 0.08), transparent);
  color: #f6f5f2;
  box-shadow: var(--shadow-lg);
}

.auth-story h1 {
  margin: 18px 0 12px;
  font-size: clamp(34px, 5vw, 56px);
  line-height: 0.98;
  letter-spacing: -0.05em;
}

.auth-story p {
  max-width: 560px;
  color: rgba(246, 245, 242, 0.82);
  font-size: 16px;
}

.auth-points {
  display: grid;
  gap: 16px;
  margin-top: 28px;
}

.auth-point {
  padding: 18px 20px;
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.1);
}

.auth-point strong {
  display: block;
  margin-bottom: 6px;
  font-size: 20px;
}

.auth-card {
  align-self: center;
}

.auth-message {
  padding: 12px 14px;
  border-radius: 16px;
  font-size: 14px;
  font-weight: 700;
}

.auth-error {
  background: rgba(187, 77, 58, 0.12);
  color: var(--danger);
}

.auth-footer {
  margin: 20px 0 0;
  color: var(--ink-soft);
}

@media (max-width: 860px) {
  .auth-shell {
    grid-template-columns: 1fr;
    min-height: auto;
  }

  .auth-story {
    padding: 24px;
  }
}
</style>
