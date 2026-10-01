<template>
  <AuthLayout>
    <form class="stack" @submit.prevent="handleSubmit">
        <div class="field">
          <label for="username">用户名</label>
          <input id="username" v-model.trim="form.username" class="input" placeholder="输入用户名" autocomplete="username" required />
        </div>

        <div class="field">
          <label for="password">密码</label>
          <input id="password" v-model="form.password" class="input" placeholder="输入密码" type="password" autocomplete="current-password" required />
        </div>

        <div v-if="error" class="auth-message auth-error" role="alert">{{ error }}</div>

        <button class="btn btn-primary btn-block" type="submit" :disabled="loading">
          <span v-if="loading" class="spinner"></span><span v-if="loading">请稍候…</span>
          <span v-else>登录并开始练习</span>
        </button>
      </form>
  </AuthLayout>
</template>

<script setup>
import AuthLayout from '../components/AuthLayout.vue'
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getErrorMessage, loginUser } from '../api'
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
    store.login(result.accessToken)
    router.push((route.query.redirect || '/')?.toString())
  } catch (requestError) {
    error.value = getErrorMessage(requestError, '登录失败，请检查用户名和密码。')
  } finally {
    loading.value = false
  }
}
</script>
