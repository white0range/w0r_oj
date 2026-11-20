import { useCallback, useEffect, useState } from 'react'
import { Link, useLocation, useNavigate, useParams, useSearchParams } from 'react-router'
import { getLeaderboard, getMySubmissions, getProfile, logoutUser } from '../api'
import Icon from '../components/Icon'
import { Notice, PageHeader, Pager, ResourceState, State, Status, formatDate, positiveInt } from '../components/ui'
import { store, useSession } from '../store'
import { safeReturnTo } from '../utils/navigation'
import { useResource } from '../utils/useResource'
import { watchSubmissionStatus } from '../utils/submissionStatus'

const PAGE_SIZE = 12
const results = ['AC', 'WA', 'CE', 'RE', 'TLE', 'MLE', 'OLE', 'SE', 'Pending']

function Submissions() {
  const [params, setParams] = useSearchParams()
  const { pathname, search } = useLocation()
  const page = positiveInt(params.get('page'))
  const status = params.get('status') || ''
  const load = useCallback(() => getMySubmissions({ page, limit: PAGE_SIZE }), [page])
  const resource = useResource(load)
  const items = resource.data?.items.filter((item) => !status || item.status === status) || []
  const pageCount = Math.max(1, Math.ceil((resource.data?.total || 0) / PAGE_SIZE))
  useEffect(() => {
    if (resource.loading || resource.error || page <= pageCount) return
    setParams((previous) => {
      const next = new URLSearchParams(previous)
      if (pageCount > 1) next.set('page', String(pageCount))
      else next.delete('page')
      return next
    }, { replace: true })
  }, [page, pageCount, resource.loading, resource.error, setParams])

  function update(name, value) {
    setParams((previous) => {
      const next = new URLSearchParams(previous)
      if (value) next.set(name, String(value))
      else next.delete(name)
      return next
    })
  }

  return <div className="page">
    <PageHeader title="提交记录" detail="查看正式判题结果，回顾提交代码。"><Link to="/" className="btn outline">去刷题</Link></PageHeader>
    <section className="panel catalog">
      <div className="catalog-tools">
        <span className="muted">{resource.data ? `共 ${resource.data.total} 条提交` : '我的提交'}</span>
        <label className="catalog-filter"><select aria-label="按本页判定筛选" value={status} onChange={(event) => update('status', event.target.value)}>
          <option value="">本页全部判定</option>
          {results.map((value) => <option key={value} value={value}>{value}</option>)}
        </select></label>
      </div>
      <ResourceState resource={resource} failure="提交记录加载失败" />
      {!resource.loading && !resource.error && <>
        {items.length ? <div className="table-scroll"><table className="data-table">
          <thead><tr><th>提交 / 题目</th><th>判定</th><th>耗时</th><th>提交时间</th></tr></thead>
          <tbody>{items.map((item) => <tr key={item.id}>
            <td><Link className="text-link" to={`/submissions/${item.id}`} state={{ submissionsFrom: pathname + search }}>
              #{item.id} · {item.problemTitle || `题目 #${item.problemId}`}
            </Link></td><td><Status value={item.status} /></td>
            <td className="mono">{item.status === 'Pending' ? '—' : `${item.timeCost} ms`}</td><td>{formatDate(item.createdAt)}</td>
          </tr>)}</tbody>
        </table></div> : <State title={status ? '本页没有匹配的提交' : '暂时没有提交记录'}>
          {status && <button className="btn outline" onClick={() => update('status', '')}>清除判定筛选</button>}
        </State>}
        <Pager page={page} limit={PAGE_SIZE} total={resource.data.total} onChange={(value) => update('page', value === 1 ? '' : value)} />
      </>}
    </section>
  </div>
}

function Submission({ id }) {
  const location = useLocation()
  const back = safeReturnTo(location.state?.submissionsFrom, '/my-submissions')
  const [submission, setSubmission] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    setLoading(true)
    setError('')
    return watchSubmissionStatus(id, (value) => {
      setSubmission(value)
      setError('')
      setLoading(false)
    }, (_, retrying) => {
      setLoading(false)
      setError(retrying ? '暂时无法读取判定，正在自动重试。' : '查不到这条提交，或当前账号没有查看权限。')
    })
  }, [id, attempt])

  return <div className="page">
    <Link className="text-link back-link" to={back}>← 返回提交记录</Link>
    <PageHeader title={`提交 #${id}`} detail={submission?.problemTitle}>
      {submission && <Link to={`/problems/${submission.problemId}`} className="btn outline">返回题目</Link>}
    </PageHeader>
    {loading && <div role="status" className="state"><span className="spinner" />正在读取判定…</div>}
    <Notice>{error && <>{error} <button className="text-button" onClick={() => setAttempt((value) => value + 1)}>重新查询</button></>}</Notice>
    {submission && <div className="problem-grid">
      <section className="panel stack"><h2>提交代码</h2><pre className="code-output">{submission.code || '（无代码）'}</pre></section>
      <section className="panel stack">
        <div className="section-head"><h2>判题结果</h2><Status value={submission.status} /></div>
        <div className="problem-facts"><span>{submission.timeCost} ms</span><span>{submission.memoryCost} KB</span><span>{submission.language.toUpperCase()}</span></div>
        <p>{submission.actualOutput || (submission.status === 'Pending' ? '判定完成后这里会自动更新。' : '暂无附加说明。')}</p>
        {submission.status !== 'Pending' && submission.status !== 'AC' &&
          <Link className="btn outline fit" to={`/chat?submission=${id}`}><Icon name="sparkles" />用 AI 分析这次提交</Link>}
      </section>
    </div>}
  </div>
}

function Leaderboard() {
  const { token, userId } = useSession()
  const load = useCallback(() => getLeaderboard(), [userId])
  const resource = useResource(load)

  return <div className="page">
    <PageHeader title="排行榜" detail="按通过的题目数排名。"><button className="btn outline" disabled={resource.loading} onClick={resource.reload}><Icon name="refresh" />刷新</button></PageHeader>
    {token && resource.data && <div className="panel personal-rank">
      <span>我的排名 <strong>{resource.data.myRank > 0 ? `#${resource.data.myRank}` : '暂无排名'}</strong></span>
      <span>通过题数 <strong>{resource.data.myScore}</strong></span>
    </div>}
    <section className="panel catalog">
      <ResourceState resource={resource} failure="排行榜加载失败" />
      {!resource.loading && !resource.error && (resource.data.top50.length ? <div className="table-scroll"><table className="data-table">
        <thead><tr><th>排名</th><th>用户</th><th>通过题数</th></tr></thead>
        <tbody>{resource.data.top50.map((item) => <tr className={item.userId === userId ? 'current-row' : ''} key={item.userId}>
          <td className="mono">#{item.rank}</td><td>{item.username}{item.userId === userId && ' · 你'}</td><td className="mono">{item.score}</td>
        </tr>)}</tbody>
      </table></div> : <State title="暂无排行记录" />)}
    </section>
  </div>
}

function Profile() {
  const profile = useResource(getProfile)
  const navigate = useNavigate()
  const [leaving, setLeaving] = useState(false)
  useEffect(() => { if (profile.data) store.hydrateProfile(profile.data) }, [profile.data])
  async function logout() {
    setLeaving(true)
    try { await logoutUser() } catch {} finally { navigate('/', { replace: true }) }
  }

  return <div className="page">
    <PageHeader title="个人中心"><button className="btn outline" disabled={leaving} onClick={logout}><Icon name="logout" />退出登录</button></PageHeader>
    <ResourceState resource={profile} failure="资料加载失败" />
    {!profile.loading && !profile.error && <>
      <section className="panel profile-card">
        <div className="avatar">{profile.data.username.slice(0, 1).toUpperCase()}</div>
        <div><h2>{profile.data.username}</h2><span className="muted">{profile.data.role === 1 ? '管理员' : '练习者'}</span></div>
        <div className="profile-score"><strong>{profile.data.solvedCount}</strong><span>已通过</span></div>
      </section>
      <section className="panel stack">
        <div className="section-head"><h2>通过的题目</h2><Link to="/my-submissions" className="text-link">查看全部提交</Link></div>
        {profile.data.solvedList.length ? <div className="stack-inline">
          {profile.data.solvedList.map((id) => <Link key={id} className="tag linked" to={`/problems/${id}`}>题目 #{id}</Link>)}
        </div> : <State title="还没有通过的题目"><Link to="/" className="btn primary">找一道题练习</Link></State>}
      </section>
    </>}
  </div>
}

export default function Practice({ view }) {
  const { id } = useParams()
  if (view === 'submission') return <Submission id={id} key={id} />
  if (view === 'submissions') return <Submissions />
  if (view === 'profile') return <Profile />
  return <Leaderboard />
}
