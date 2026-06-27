<template>
  <div class="auth-shell">
    <section class="auth-story register-story">
      <span class="eyebrow">Create Account</span>
      <h1>把这套 OJ 体验完整跑通。</h1>
      <p>
        注册后你就能从“题库浏览、代码提交、判题回写、结果复盘”这条主链路开始体验整个系统，
        也更适合向面试官展示你对后端能力和产品完整度的把控。
      </p>
      <div class="auth-points">
        <div class="auth-point">
          <strong>Queue Pipeline</strong>
          <span>提交会进入 Redis 判题队列，再由后端 worker 异步消费并回写结果。</span>
        </div>
        <div class="auth-point">
          <strong>Sandbox Judge</strong>
          <span>运行阶段由沙箱负责隔离资源限制，这是整条判题链路最核心的一环。</span>
        </div>
      </div>
    </section>

    <section class="card auth-card">
      <div class="section-title">
        <h2>创建账号</h2>
      </div>

      <form class="stack" @submit.prevent="handleSubmit">
        <div class="field">
          <label for="username">用户名</label>
          <input id="username" v-model.trim="form.username" class="input" required />
        </div>

        <div class="field">
          <label for="password">密码</label>
          <input id="password" v-model="form.password" class="input" type="password" minlength="6" required />
        </div>

        <div class="field">
          <label for="confirm-password">确认密码</label>
          <input id="confirm-password" v-model="confirmPassword" class="input" type="password" minlength="6" required />
        </div>

        <div v-if="error" class="auth-message auth-error">{{ error }}</div>
        <div v-if="success" class="auth-message auth-success">{{ success }}</div>

        <button class="btn btn-primary btn-block" type="submit" :disabled="loading">
          <span v-if="loading" class="spinner"></span>
          <span v-else>创建账号</span>
        </button>
      </form>

      <p class="auth-footer">
        已经注册过了？
        <router-link to="/login">直接登录</router-link>
      </p>
    </section>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getErrorMessage, registerUser } from '../api'

const router = useRouter()
const loading = ref(false)
const error = ref('')
const success = ref('')
const confirmPassword = ref('')
const form = reactive({
  username: '',
  password: '',
})

async function handleSubmit() {
  error.value = ''
  success.value = ''

  if (form.password !== confirmPassword.value) {
    error.value = '两次输入的密码不一致。'
    return
  }

  if (form.password.length < 6) {
    error.value = '密码长度至少需要 6 位。'
    return
  }

  loading.value = true

  try {
    await registerUser(form)
    success.value = '注册成功，正在带你跳转到登录页。'
    setTimeout(() => {
      router.push('/login')
    }, 900)
  } catch (requestError) {
    error.value = getErrorMessage(requestError, '注册失败，可能用户名已存在。')
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
    linear-gradient(145deg, rgba(17, 32, 58, 0.95), rgba(15, 139, 131, 0.88)),
    linear-gradient(180deg, rgba(255, 255, 255, 0.08), transparent);
  color: #f7f4ef;
  box-shadow: var(--shadow-lg);
}

.register-story {
  background:
    linear-gradient(145deg, rgba(15, 139, 131, 0.95), rgba(209, 98, 57, 0.86)),
    linear-gradient(180deg, rgba(255, 255, 255, 0.08), transparent);
}

.auth-story h1 {
  margin: 18px 0 12px;
  font-size: clamp(34px, 5vw, 56px);
  line-height: 0.98;
  letter-spacing: -0.05em;
}

.auth-story p {
  max-width: 560px;
  color: rgba(247, 244, 239, 0.84);
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
