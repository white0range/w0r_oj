import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, NavLink, useLocation, useNavigate, useParams, useSearchParams } from 'react-router'
import {
  adminAddTestCase, adminBanUser, adminCreateProblem, adminCreateTag, adminDeleteProblem,
  adminDeleteTag, adminDeleteTestCase, adminGetTestCases, adminGetUsers, adminUnbanUser,
  adminUpdateProblem, adminUpdateProblemTags, getErrorMessage, getProblemDetail, getProblems, getTags,
} from '../api'
import { Notice, PageHeader, Pager, ResourceState, State, positiveInt } from '../components/ui'
import { useResource } from '../utils/useResource'

const PAGE_SIZE = 12

function ActionsNotice({ result }) {
  return <Notice kind={result?.kind}>{result?.text}</Notice>
}

function AdminUsers() {
  const [draft, setDraft] = useState('')
  const [keyword, setKeyword] = useState('')
  const load = useCallback(() => adminGetUsers({ keyword, limit: 100 }), [keyword])
  const users = useResource(load)
  const [ban, setBan] = useState(null)
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState(null)

  async function changeStatus(user, banning) {
    setBusy(true)
    setResult(null)
    try {
      if (banning) await adminBanUser(user.id, { reason: reason.trim() })
      else await adminUnbanUser(user.id)
      setBan(null)
      setResult({ kind: 'success', text: `已${banning ? '封禁' : '解封'} ${user.username}` })
      users.reload()
    } catch (error) {
      setResult({ kind: 'error', text: getErrorMessage(error, '操作失败，请重试。') })
    } finally {
      setBusy(false)
    }
  }

  return <div className="stack">
    <form className="panel padded stack-inline" onSubmit={(event) => {
      event.preventDefault()
      if (draft.trim() === keyword) users.reload()
      else setKeyword(draft.trim())
    }}>
      <input aria-label="搜索用户名" placeholder="输入用户名" value={draft} onChange={(event) => setDraft(event.target.value)} />
      <button className="btn primary">搜索用户</button>
    </form>
    <ActionsNotice result={result} />
    {ban && <form className="panel padded stack" onSubmit={(event) => { event.preventDefault(); void changeStatus(ban, true) }}>
      <label>封禁 {ban.username} 的原因 <input autoFocus maxLength={500} value={reason} onChange={(event) => setReason(event.target.value)} /></label>
      <div className="stack-inline">
        <button className="btn danger" disabled={busy}>确认封禁</button>
        <button className="btn outline" type="button" disabled={busy} onClick={() => setBan(null)}>取消</button>
      </div>
    </form>}
    <section className="panel">
      <ResourceState resource={users} failure="用户列表加载失败" />
      {!users.loading && users.data && (users.data.length ? <div className="table-scroll">
        <table><thead><tr><th scope="col">用户</th><th scope="col">角色</th><th scope="col">状态</th><th scope="col">通过数</th><th scope="col">封禁原因</th><th scope="col">操作</th></tr></thead>
          <tbody>{users.data.map((user) => <tr key={user.id}>
            <td><strong>{user.username}</strong> <span className="muted">#{user.id}</span></td>
            <td>{user.role === 1 ? '管理员' : '用户'}</td>
            <td><span className={`status ${user.status === 'banned' ? 'bad' : 'good'}`}>{user.status === 'banned' ? '已封禁' : '正常'}</span></td>
            <td>{user.solved_count}</td><td>{user.ban_reason || '—'}</td>
            <td>{user.role !== 1 && (user.status === 'banned' ?
              <button className="btn outline sm" disabled={busy} onClick={() => changeStatus(user, false)}>解封</button> :
              <button className="btn outline sm" disabled={busy} onClick={() => { setBan(user); setReason(''); setResult(null) }}>封禁</button>)}</td>
          </tr>)}</tbody>
        </table>
      </div> : <State title="没有找到用户" />)}
    </section>
  </div>
}

function AdminProblems() {
  const [params, setParams] = useSearchParams()
  const page = positiveInt(params.get('page'))
  const { state } = useLocation()
  const [result, setResult] = useState(state?.message ? { kind: 'success', text: state.message } : null)
  const [busy, setBusy] = useState(false)
  const load = useCallback(() => getProblems({ page, limit: PAGE_SIZE }), [page])
  const problems = useResource(load)
  const lastPage = Math.max(1, Math.ceil((problems.data?.total || 0) / PAGE_SIZE))

  useEffect(() => {
    if (!problems.loading && problems.data && page > lastPage) setParams(lastPage > 1 ? { page: String(lastPage) } : {}, { replace: true })
  }, [problems.loading, problems.data, lastPage, page, setParams])

  async function remove(problem) {
    if (!window.confirm(`删除「${problem.title}」？`)) return
    setBusy(true)
    setResult(null)
    try {
      await adminDeleteProblem(problem.id)
      setResult({ kind: 'success', text: '题目已删除。' })
      problems.reload()
    } catch (error) {
      setResult({ kind: 'error', text: getErrorMessage(error, '删除失败，请重试。') })
    } finally {
      setBusy(false)
    }
  }

  return <div className="stack">
    <ActionsNotice result={result} />
    <section className="panel">
      <ResourceState resource={problems} failure="题目加载失败" />
      {!problems.loading && problems.data && (problems.data.items.length ? <>
        <div className="table-scroll">
          <table><thead><tr><th scope="col">题号</th><th scope="col">题目</th><th scope="col">知识点</th><th scope="col">操作</th></tr></thead>
            <tbody>{problems.data.items.map((problem) => <tr key={problem.id}>
              <td>#{problem.id}</td>
              <td><Link to={`/problems/${problem.id}`}><strong>{problem.title}</strong></Link></td>
              <td><div className="stack-inline">{problem.tags.map((tag) => <span key={tag.id} className="tag">{tag.name}</span>)}</div></td>
              <td><div className="stack-inline">
                <Link className="btn outline sm" to={`/admin/problems/${problem.id}/edit`}>编辑</Link>
                <button className="text-button danger-text" disabled={busy} onClick={() => remove(problem)}>删除</button>
              </div></td>
            </tr>)}</tbody>
          </table>
        </div>
        <Pager page={page} limit={PAGE_SIZE} total={problems.data.total} onChange={(next) => setParams(next > 1 ? { page: String(next) } : {})} />
      </> : <State title="还没有题目"><Link className="btn primary" to="/admin/problems/new">新建题目</Link></State>)}
    </section>
  </div>
}

function AdminTags() {
  const tags = useResource(getTags)
  const [name, setName] = useState('')
  const [result, setResult] = useState(null)
  const [busy, setBusy] = useState(false)

  async function mutate(tag) {
    if (tag && !window.confirm(`删除标签「${tag.name}」？`)) return
    if (!tag && !name.trim()) return
    setBusy(true)
    setResult(null)
    try {
      if (tag) await adminDeleteTag(tag.id)
      else { await adminCreateTag({ name: name.trim() }); setName('') }
      setResult({ kind: 'success', text: tag ? '标签已删除。' : '标签已创建。' })
      tags.reload()
    } catch (error) {
      setResult({ kind: 'error', text: getErrorMessage(error, '操作失败，请重试。') })
    } finally {
      setBusy(false)
    }
  }

  return <div className="stack">
    <form className="panel padded stack-inline" onSubmit={(event) => { event.preventDefault(); void mutate() }}>
      <input aria-label="新标签名称" placeholder="输入标签名称" maxLength={50} required disabled={busy} value={name} onChange={(event) => setName(event.target.value)} />
      <button className="btn primary" disabled={busy || !name.trim()}>添加标签</button>
    </form>
    <ActionsNotice result={result} />
    <section className="panel">
      <ResourceState resource={tags} failure="标签加载失败" />
      {!tags.loading && tags.data && (tags.data.length ? <div className="table-scroll">
        <table><thead><tr><th scope="col">标签</th><th scope="col">操作</th></tr></thead>
          <tbody>{tags.data.map((tag) => <tr key={tag.id}><td>{tag.name}</td>
            <td><button className="text-button danger-text" disabled={busy} onClick={() => mutate(tag)}>删除</button></td>
          </tr>)}</tbody>
        </table>
      </div> : <State title="还没有标签" />)}
    </section>
  </div>
}

function AdminEditor({ id }) {
  const load = useCallback(async () => {
    const [tags, detail] = await Promise.all([getTags(), id ? getProblemDetail(id) : null])
    return { tags, detail }
  }, [id])
  const problem = useResource(load)
  return <><ResourceState resource={problem} failure="编辑数据加载失败" />
    {problem.data && <ProblemForm id={id} tags={problem.data.tags} detail={problem.data.detail} />}
  </>
}

function ProblemForm({ id, tags, detail }) {
  const navigate = useNavigate()
  const [title, setTitle] = useState(detail?.title || '')
  const [description, setDescription] = useState(detail?.description || '')
  const [timeLimit, setTimeLimit] = useState(detail?.timeLimit || 1000)
  const [memoryLimit, setMemoryLimit] = useState(detail?.memoryLimit || 256)
  const [tagIds, setTagIds] = useState(() => (detail?.tags || []).map((tag) => tag.id))
  const [drafts, setDrafts] = useState([])
  const sequence = useRef(0)
  const [casePage, setCasePage] = useState(1)
  const loadCases = useCallback(() => id ? adminGetTestCases(id, { page: casePage, limit: 10 }) : null, [id, casePage])
  const cases = useResource(loadCases)
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState(null)

  function editCase(key, field, value) {
    setDrafts((items) => items.map((item) => item.key === key ? { ...item, [field]: value } : item))
  }

  async function removeCase(item) {
    if (!window.confirm(`删除测试用例 #${item.id}？`)) return
    setBusy(true)
    setResult(null)
    try {
      await adminDeleteTestCase(item.id)
      if (casePage > 1 && cases.data.items.length === 1) setCasePage(casePage - 1)
      else cases.reload()
      setResult({ kind: 'success', text: '测试用例已删除。' })
    } catch (error) {
      setResult({ kind: 'error', text: getErrorMessage(error, '用例删除失败，请重试。') })
    } finally {
      setBusy(false)
    }
  }

  async function save(event) {
    event.preventDefault()
    if (!title.trim() || !description.trim()) {
      setResult({ kind: 'error', text: '请填写题名和题面。' })
      return
    }
    setResult(null)
    setBusy(true)
    let basicsSaved = false
    try {
      const payload = { title: title.trim(), description, time_limit: Number(timeLimit), memory_limit: Number(memoryLimit) }
      const testCases = drafts.map(({ input, expected_output }) => ({ input, expected_output }))
      if (!id) {
        await adminCreateProblem({ ...payload, tag_ids: tagIds, test_cases: testCases })
        navigate('/admin/problems', { state: { message: '题目已创建。' } })
        return
      }
      await adminUpdateProblem(id, payload)
      basicsSaved = true
      await adminUpdateProblemTags(id, { tag_ids: tagIds })
      for (const { key, input, expected_output } of drafts) {
        await adminAddTestCase(id, { input, expected_output })
        setDrafts((items) => items.filter((item) => item.key !== key))
      }
      setResult({ kind: 'success', text: '题目已保存。' })
    } catch (error) {
      const detail = getErrorMessage(error, '请检查输入后重试。')
      setResult({ kind: 'error', text: basicsSaved ? `题目信息已保存，其余更改可能未全部生效。 ${detail}` : detail })
    } finally {
      if (id && drafts.length) cases.reload()
      setBusy(false)
    }
  }

  return <form className="stack admin-form" onSubmit={save}>
    <ActionsNotice result={result} />
    <fieldset className="panel padded stack" disabled={busy}>
      <h2>题目信息</h2>
      <label>题名<input required maxLength={200} value={title} onChange={(event) => setTitle(event.target.value)} /></label>
      <label>题面<textarea required rows={9} maxLength={32768} value={description} onChange={(event) => setDescription(event.target.value)} /></label>
      <div className="two-columns">
        <label>时间限制（ms）<input required type="number" min={100} max={10000} value={timeLimit} onChange={(event) => setTimeLimit(event.target.value)} /></label>
        <label>内存限制（MB）<input required type="number" min={16} max={512} value={memoryLimit} onChange={(event) => setMemoryLimit(event.target.value)} /></label>
      </div>
      <div className="stack">
        <h3>知识点</h3>
        <div className="stack-inline">{tags.map((tag) => <label key={tag.id} className="check-chip">
          <input type="checkbox" checked={tagIds.includes(tag.id)} disabled={tagIds.length >= 20 && !tagIds.includes(tag.id)}
            onChange={(event) => setTagIds((ids) => event.target.checked ? [...ids, tag.id] : ids.filter((value) => value !== tag.id))} />
          {tag.name}
        </label>)}
          {!tags.length && <Link to="/admin/tags" className="text-link">创建知识点标签</Link>}
        </div>
      </div>
    </fieldset>

    <fieldset className="panel padded stack" disabled={busy}>
      <div className="section-head"><h2>测试用例</h2>
        <button type="button" className="btn outline sm" disabled={drafts.length >= 10} onClick={() => {
          const key = ++sequence.current
          setDrafts((items) => [...items, { key, input: '', expected_output: '' }])
        }}>添加用例</button>
      </div>
      {id > 0 && <ResourceState resource={cases} failure="已有用例加载失败" />}
      {cases.data?.items.map((item) => <details className="case-preview" key={item.id}>
        <summary>用例 #{item.id}</summary>
        <div className="two-columns">
          <div><h3>输入</h3><pre>{item.input}</pre></div>
          <div><h3>预期输出</h3><pre>{item.expectedOutput}</pre></div>
        </div>
        <button type="button" className="text-button danger-text" onClick={() => removeCase(item)}>删除该用例</button>
      </details>)}
      {cases.data && <Pager page={casePage} limit={10} total={cases.data.total} busy={busy || cases.loading} onChange={setCasePage} />}
      {drafts.map((item, index) => <div className="case-preview stack" key={item.key}>
        <div className="section-head"><h3>新增用例 {index + 1}</h3>
          <button type="button" className="text-button" onClick={() => setDrafts((items) => items.filter(({ key }) => key !== item.key))}>移除</button>
        </div>
        <div className="two-columns">
          <label>输入<textarea rows={3} value={item.input} maxLength={65536} required onChange={(event) => editCase(item.key, 'input', event.target.value)} /></label>
          <label>预期输出<textarea rows={3} value={item.expected_output} maxLength={65536} required onChange={(event) => editCase(item.key, 'expected_output', event.target.value)} /></label>
        </div>
      </div>)}
      {!drafts.length && !cases.loading && !cases.data?.items.length && <p className="muted">还没有测试用例。</p>}
    </fieldset>
    <div className="stack-inline">
      <button className="btn primary" disabled={busy}>{busy ? '保存中…' : id ? '保存修改' : '创建题目'}</button>
      <Link to="/admin/problems" className="btn outline">返回题目管理</Link>
    </div>
  </form>
}

export default function Admin({ view }) {
  const { id } = useParams()
  const problemId = positiveInt(id, 0)
  const title = { problems: '题目管理', users: '用户管理', tags: '知识点管理', edit: id ? `编辑题目 #${id}` : '新建题目' }[view]
  return <div className="page">
    <PageHeader title={title}>{view === 'problems' && <Link to="/admin/problems/new" className="btn primary">新建题目</Link>}</PageHeader>
    <nav className="tabs" aria-label="管理后台">
      <NavLink to="/admin/problems">题目</NavLink><NavLink to="/admin/tags">知识点</NavLink><NavLink to="/admin/users">用户</NavLink>
    </nav>
    {view === 'problems' && <AdminProblems />}
    {view === 'tags' && <AdminTags />}
    {view === 'users' && <AdminUsers />}
    {view === 'edit' && (id && !problemId ? <State title="题号无效" /> : <AdminEditor key={problemId} id={problemId} />)}
  </div>
}
