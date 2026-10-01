<template>
  <div class="auth-shell">
    <section class="auth-story">
      <span class="auth-story-kicker"><AppIcon name="code" /> GOJO OJ / START YOUR JOURNEY</span>
      <div class="auth-story-copy"><h1>把思考写成代码，<br>把练习变成实力。</h1><p>在这里，找到值得投入的题目，<br>也找到属于自己的进步节奏。</p></div>
      <div class="auth-illustration" aria-hidden="true"><div class="code-window-head"><i></i><i></i><i></i><span>solution.go</span></div><pre><span class="code-comment">// A little better, every day.</span>
<span class="code-keyword">func</span> main() {
    practice()
    reflect()
    grow()
}</pre><span class="code-result"><AppIcon name="check" /> Accepted <small>继续向前</small></span></div>
      <div class="auth-story-bottom"><span>专注算法，持续进步。</span><span>01 — ∞</span></div>
    </section>
    <section class="auth-form-panel">
      <div class="auth-form-heading"><span class="eyebrow">{{ register ? 'JOIN THE COMMUNITY' : 'WELCOME BACK' }}</span><h2>{{ register ? '开启你的算法之旅' : '欢迎回来' }}</h2><p>{{ register ? '创建账户，记录每一次突破。' : '登录 Gojo OJ，继续今天的练习。' }}</p></div>
      <slot />
      <p class="auth-footer"><template v-if="register">已经有账号？<router-link to="/login">立即登录 <AppIcon name="arrow" /></router-link></template><template v-else>还没有账号？<router-link to="/register">免费注册 <AppIcon name="arrow" /></router-link></template></p>
      <div class="auth-safe-note"><AppIcon name="lock" />专注学习，安心练习</div>
    </section>
  </div>
</template>
<script setup>
import AppIcon from './AppIcon.vue'
defineProps({ register: { type: Boolean, default: false } })
</script>
<style scoped>
.auth-shell {
  display: grid;
  grid-template-columns: 1.05fr 1fr;
  max-width: 1040px;
  margin: 18px auto;
  min-height: 620px;
  background: white;
  border: 1px solid var(--line);
  border-radius: 18px;
  overflow: hidden;
  box-shadow: var(--shadow-lg);
}

.auth-story {
  display: flex;
  flex-direction: column;
  position: relative;
  padding: 40px;
  background: radial-gradient(ellipse at top left, #284372, transparent 65%), #142440;
  color: white;
}

.auth-story-kicker {
  display: flex;
  align-items: center;
  gap: 9px;
  font-size: 8px;
  letter-spacing: 1.7px;
  color: #a3b7d9;
}

.auth-story-kicker .app-icon {
  width: 18px;
  height: 18px;
}

.auth-story-copy h1 {
  margin: 42px 0 16px;
  font-size: clamp(28px, 3vw, 36px);
  line-height: 1.6;
  font-weight: 650;
  letter-spacing: .5px;
}

.auth-story-copy p {
  color: #a1b3ce;
  font-size: 13px;
  line-height: 1.9;
  margin: 0;
}

.auth-illustration {
  margin-top: 32px;
  border: 1px solid #718bb432;
  border-radius: 10px;
  overflow: hidden;
  background: #0e1d34a0;
  transform: rotate(-2deg);
}

.code-window-head {
  padding: 12px 16px;
  display: flex;
  gap: 6px;
  align-items: center;
  border-bottom: 1px solid #718bb422;
}

.code-window-head i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #8299ba;
}

.code-window-head i:nth-child(1) {
  background: #db857c;
}

.code-window-head i:nth-child(2) {
  background: #d9b578;
}

.code-window-head i:nth-child(3) {
  background: #81ba9c;
}

.code-window-head span {
  font: 9px Consolas, monospace;
  margin-left: auto;
  color: #8ea6cc;
}

.auth-illustration pre {
  margin: 0;
  padding: 18px 22px;
  color: #bcd1f1;
  font: 12px/1.9 Consolas, monospace;
}

.code-comment {
  color: #6f89ac;
}

.code-keyword {
  color: #9ebafa;
}

.code-result {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  color: #b3efcd;
  background: #72b7910a;
  border-top: 1px solid #718bb422;
  font: 11px Consolas, monospace;
}

.code-result .app-icon {
  width: 14px;
  height: 14px;
}

.code-result small {
  margin-left: auto;
  color: #779b8f;
  font-size: 9px;
}

.auth-story-bottom {
  display: flex;
  justify-content: space-between;
  color: #7089af;
  font-size: 10px;
  margin-top: auto;
  padding-top: 34px;
}

.auth-story-bottom > span:last-child {
  letter-spacing: 2px;
}

.auth-form-panel {
  align-self: center;
  padding: 48px 52px;
}

.auth-form-heading {
  margin-bottom: 30px;
}

.auth-form-heading h2 {
  margin: 14px 0 10px;
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -.5px;
}

.auth-form-heading p {
  color: var(--ink-faint);
  font-size: 13px;
  margin: 0;
}

.auth-footer {
  margin: 26px 0 0;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  justify-content: center;
  font-size: 12px;
  color: var(--ink-faint);
}

.auth-footer a {
  color: var(--brand);
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 5px;
}

.auth-footer .app-icon {
  width: 13px;
  height: 13px;
}

.auth-safe-note {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  margin-top: 32px;
  color: #a4adbd;
  font-size: 10px;
}

.auth-safe-note .app-icon {
  width: 12px;
  height: 12px;
}

@media (max-width: 860px) {
  .auth-story {
    padding: 32px;
  }
  .auth-form-panel {
    padding: 36px 28px;
  }
  .auth-story-copy h1 {
    font-size: 28px;
  }
}

@media (max-width: 680px) {
  .auth-shell {
    grid-template-columns: 1fr;
    margin: 0 auto;
    min-height: auto;
    max-width: 460px;
  }
  .auth-story {
    padding: 24px;
  }
  .auth-story-copy h1 {
    font-size: 25px;
    margin: 22px 0 10px;
  }
  .auth-story-copy h1 br {
    display: none;
  }
  .auth-story-copy p {
    font-size: 12px;
  }
  .auth-story-copy p br {
    display: none;
  }
  .auth-illustration, .auth-story-bottom {
    display: none;
  }
  .auth-form-panel {
    padding: 28px 24px;
  }
  .auth-form-heading h2 {
    font-size: 25px;
  }
  .auth-safe-note {
    margin-top: 24px;
  }
}
</style>
