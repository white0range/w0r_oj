import { useState } from 'react'
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router'
import { getErrorMessage, loginUser, registerUser } from '../api'
import Icon from '../components/Icon'
import { Notice, usePageTitle } from '../components/ui'
import { store } from '../store'
import { safeReturnTo } from '../utils/navigation'

export default function Auth({ register = false }) {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const location = useLocation()
  const returnTo = safeReturnTo(params.get('redirect'))
  const libraryFrom = location.state?.libraryFrom
  const alternate = register ? '/login' : '/register'
  const query = returnTo === '/' ? '' : `?${new URLSearchParams({ redirect: returnTo })}`
  const [username, setUsername] = useState(location.state?.username || '')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const title = register ? '注册账号' : '登录'
  usePageTitle(title)

  async function submit(event) {
    event.preventDefault()
    if (loading) return
    setError('')
    if (register && password !== confirm) {
      setError('两次输入的密码不一致。')
      return
    }
    setLoading(true)
    const payload = { username: username.trim(), password }
    try {
      if (register) {
        await registerUser(payload)
        navigate('/login' + query, { replace: true, state: { registered: true, username: payload.username, libraryFrom } })
      } else {
        const { accessToken } = await loginUser(payload)
        if (!accessToken) throw new Error('Missing access token')
        store.login(accessToken)
        navigate(returnTo, { replace: true, state: libraryFrom ? { libraryFrom } : undefined })
      }
    } catch (err) {
      setError(getErrorMessage(err, register ? '注册失败，请检查输入后重试。' : '登录失败，请检查用户名和密码。'))
    } finally {
      setLoading(false)
    }
  }

  return <section className="auth-panel">
    <div className="auth-intro">
      <Icon name="code" />
      <h1>{title}</h1>
      <p>{register ? '保存你的提交记录和刷题进度。' : '登录后继续写题、查看提交或使用 AI 助手。'}</p>
    </div>
    <form className="panel auth-form stack" onSubmit={submit}>
      {location.state?.registered && <Notice kind="success">账号已创建，请登录。</Notice>}
      <Notice>{error}</Notice>
      <label className="field">用户名<input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" maxLength={32} required /></label>
      <label className="field">密码<input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete={register ? 'new-password' : 'current-password'} minLength={register ? 6 : undefined} maxLength={128} required /></label>
      {register && <label className="field">确认密码<input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" required /></label>}
      <button className="btn primary" disabled={loading || !username.trim()}>{loading ? '正在处理…' : title}</button>
      <Link className="text-link" to={alternate + query} state={{ libraryFrom }}>{register ? '已有账号，去登录' : '还没有账号，立即注册'}</Link>
    </form>
  </section>
}
