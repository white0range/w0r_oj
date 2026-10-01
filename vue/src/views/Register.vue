<template>
  <AuthLayout register>
    <form class="stack" @submit.prevent="handleSubmit">
        <div class="field">
          <label for="username">用户名</label>
          <input id="username" v-model.trim="form.username" class="input" placeholder="设置你的用户名" autocomplete="username" required />
        </div>

        <div class="field">
          <label for="password">密码</label>
          <input id="password" v-model="form.password" class="input" placeholder="至少 6 位字符" type="password" autocomplete="new-password" minlength="6" required />
        </div>

        <div class="field">
          <label for="confirm-password">确认密码</label>
          <input id="confirm-password" v-model="confirmPassword" class="input" placeholder="再次输入密码" type="password" autocomplete="new-password" minlength="6" required />
        </div>

        <div v-if="error" class="auth-message auth-error" role="alert">{{ error }}</div>
        <div v-if="success" class="auth-message auth-success" role="status">{{ success }}</div>

        <button class="btn btn-primary btn-block" type="submit" :disabled="loading">
          <span v-if="loading" class="spinner"></span><span v-if="loading">请稍候…</span>
          <span v-else>创建账户，开始练习</span>
        </button>
      </form>
  </AuthLayout>
</template>

<script setup>
import AuthLayout from '../components/AuthLayout.vue'
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
