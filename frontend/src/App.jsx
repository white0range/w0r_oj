import { lazy, Suspense, useEffect, useState } from 'react'
import { Link, NavLink, Navigate, Outlet, Route, Routes, useLocation, useNavigate } from 'react-router'
import { logoutUser } from './api'
import Icon from './components/Icon'
import { PageHeader, State } from './components/ui'
import { useSession } from './store'

const Home = lazy(() => import('./pages/Home'))
const Auth = lazy(() => import('./pages/Auth'))
const Problem = lazy(() => import('./pages/Problem'))
const Practice = lazy(() => import('./pages/Practice'))
const Chat = lazy(() => import('./pages/Chat'))
const Admin = lazy(() => import('./pages/Admin'))

function RequireSession({ admin = false }) {
  const session = useSession()
  const location = useLocation()
  if (!session.token) {
    const redirect = new URLSearchParams({ redirect: location.pathname + location.search })
    return <Navigate replace to={`/login?${redirect}`} />
  }
  if (admin && session.role !== 1) return <State title="需要管理员权限"><Link to="/" className="btn outline">返回题库</Link></State>
  return <Outlet />
}

function Header() {
  const session = useSession()
  const navigate = useNavigate()
  const location = useLocation()
  const [signingOut, setSigningOut] = useState(false)
  const inLibrary = location.pathname === '/' || location.pathname.startsWith('/problems/')
  async function logout() {
    setSigningOut(true)
    try {
      await logoutUser()
    } catch {
      // The session store is cleared even if the logout request fails.
    } finally {
      setSigningOut(false)
      navigate('/', { replace: true })
    }
  }

  return <header className="site-header">
    <div className="header-inner">
      <Link to="/" className="wordmark" aria-label="w0r_oj 首页"><span><Icon name="code" /></span>w0r_oj</Link>
      <nav className="site-nav" aria-label="主导航">
        <Link to="/" className={inLibrary ? 'active' : undefined} aria-current={inLibrary ? 'page' : undefined}><Icon name="book" />题库</Link>
        {session.token && <NavLink to="/my-submissions"><Icon name="history" />提交记录</NavLink>}
        <NavLink to="/chat"><Icon name="sparkles" />AI 助手</NavLink>
        <NavLink to="/leaderboard"><Icon name="trophy" />排行榜</NavLink>
        {session.token && session.role === 1 && <Link to="/admin/problems" className={location.pathname.startsWith('/admin/') ? 'active' : undefined}><Icon name="grid" />管理</Link>}
      </nav>
      <div className="account-links">
        {session.token ? <>
          <Link className="account-link" to="/profile"><span className="avatar" aria-hidden="true">{session.username.slice(0, 1).toUpperCase() || 'U'}</span><span>{session.username || '个人中心'}</span></Link>
          <button className="icon-button" aria-label="退出登录" title="退出登录" disabled={signingOut} onClick={logout}><Icon name="logout" /></button>
        </> : <>
          <Link className="btn ghost" to="/login">登录</Link>
          <Link className="btn primary" to="/register">注册</Link>
        </>}
      </div>
    </div>
  </header>
}

function Footer() {
  return <footer className="site-footer">
    <span>w0r_oj</span>
    <div>
      <a href="https://beian.miit.gov.cn/" target="_blank" rel="noreferrer">鲁ICP备2026058391号-1</a>
      <a href="https://beian.mps.gov.cn/#/query/webSearch?code=37130202373179" target="_blank" rel="noreferrer">
        <img src="/police-filing.png" alt="" width="14" height="14" />鲁公网安备37130202373179号
      </a>
    </div>
  </footer>
}

function NotFound() {
  return <div className="page"><PageHeader title="页面不存在" /><State title="找不到这个页面">
    <Link className="btn primary" to="/">返回题库</Link>
  </State></div>
}

export default function App() {
  const { pathname } = useLocation()
  useEffect(() => { window.scrollTo(0, 0) }, [pathname])

  return <div className="site-shell">
    <a href="#main-content" className="skip-link">跳到主要内容</a>
    <Header />
    <main id="main-content" className="site-main" tabIndex={-1}>
      <Suspense fallback={<div className="state" role="status"><span className="spinner" />正在打开页面…</div>}>
        <Routes>
          <Route index element={<Home />} />
          <Route path="problems/:id" element={<Problem />} />
          <Route path="leaderboard" element={<Practice view="leaderboard" />} />
          <Route path="login" element={<Auth key="login" />} />
          <Route path="register" element={<Auth key="register" register />} />
          <Route element={<RequireSession />}>
            <Route path="profile" element={<Practice view="profile" />} />
            <Route path="my-submissions" element={<Practice view="submissions" />} />
            <Route path="submissions/:id" element={<Practice view="submission" />} />
            <Route path="chat" element={<Chat />} />
          </Route>
          <Route element={<RequireSession admin />}>
            <Route path="admin/users" element={<Admin view="users" />} />
            <Route path="admin/problems" element={<Admin view="problems" />} />
            <Route path="admin/problems/new" element={<Admin key="new" view="edit" />} />
            <Route path="admin/problems/:id/edit" element={<Admin view="edit" />} />
            <Route path="admin/tags" element={<Admin view="tags" />} />
          </Route>
          <Route path="*" element={<NotFound />} />
        </Routes>
      </Suspense>
    </main>
    <Footer />
  </div>
}
