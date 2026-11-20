import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react'
import { Link, useLocation, useParams } from 'react-router'
import { getErrorMessage, getProblemDetail, runProblemCode, submitCode } from '../api'
import Icon from '../components/Icon'
import { Notice, PageHeader, ResourceState, State, Status } from '../components/ui'
import { useSession } from '../store'
import { safeReturnTo } from '../utils/navigation'
import { getAcceptanceRate } from '../utils/normalizers'
import { useResource } from '../utils/useResource'
import { watchSubmissionStatus } from '../utils/submissionStatus'

const CodeEditor = lazy(() => import('../components/GoCodeEditor'))
const template = 'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("Hello w0r_oj")\n}\n'

function Statement({ problem }) {
  const blocks = problem.description.split(/```/)
  return <section className="panel statement stack">
    <h2>题目描述</h2>
    <div className="problem-facts">
      <span><Icon name="clock" /> {problem.timeLimit} ms</span>
      <span>{problem.memoryLimit} MB</span>
      {problem.submitCount > 0 && <span>通过率 {getAcceptanceRate(problem)}%</span>}
    </div>
    {problem.tags.length > 0 && <div className="stack-inline">{problem.tags.map((tag) => <span key={tag.id} className="tag">{tag.name}</span>)}</div>}
    <div className="statement-body">
      {blocks.map((part, index) => index % 2 ? <pre key={index}>{part.replace(/^\w+\n/, '').trim()}</pre> : <div key={index}>{part.trim()}</div>)}
    </div>
  </section>
}

function Workbench({ problem, userId }) {
  const draftKey = `gojo:draft:${userId}:${problem.id}:go`
  const [code, setCode] = useState(() => {
    try { return localStorage.getItem(draftKey) ?? template } catch { return template }
  })
  const [draftError, setDraftError] = useState(false)
  const [input, setInput] = useState('')
  const [running, setRunning] = useState(false)
  const [runError, setRunError] = useState('')
  const [run, setRun] = useState(null)
  const [submit, setSubmit] = useState(null)
  const [submitError, setSubmitError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const pendingRun = useRef(null)
  const stopWatching = useRef(null)
  const alive = useRef(true)
  const sendingSubmit = useRef(false)

  useEffect(() => {
    return () => {
      alive.current = false
      pendingRun.current?.abort()
      stopWatching.current?.()
    }
  }, [])

  function updateCode(next) {
    setCode(next)
    try {
      localStorage.setItem(draftKey, next)
      setDraftError(false)
    } catch {
      setDraftError(true)
    }
  }

  async function runCode() {
    if (!code.trim() || pendingRun.current) return
    const controller = new AbortController()
    pendingRun.current = controller
    setRunning(true)
    setRun(null)
    setRunError('')
    const started = performance.now()
    try {
      const result = await runProblemCode(problem.id, { code, input }, controller.signal)
      if (!controller.signal.aborted) setRun({ ...result, elapsed: Math.round(performance.now() - started) })
    } catch (error) {
      if (!controller.signal.aborted) setRunError(getErrorMessage(error, '运行请求失败，请稍后重试。'))
    } finally {
      if (pendingRun.current === controller) pendingRun.current = null
      if (alive.current) setRunning(false)
    }
  }

  async function sendSubmission() {
    if (!code.trim() || sendingSubmit.current) return
    sendingSubmit.current = true
    setSubmitting(true)
    setSubmitError('')
    try {
      const entry = await submitCode({ problem_id: problem.id, language: 'go', code })
      if (!alive.current) return
      if (!entry.submissionId) throw new Error('Missing submission ID')
      stopWatching.current?.()
      setSubmit({ id: entry.submissionId, status: entry.status, delayed: entry.dispatchPending })
      stopWatching.current = watchSubmissionStatus(entry.submissionId, (result) => {
        setSubmitError('')
        setSubmit({ id: entry.submissionId, status: result.status, message: result.actualOutput })
      }, (_, retrying) => {
        setSubmitError(retrying ? '判题状态暂时无法读取，系统会自动重试。' : '无法读取最新判定，可打开提交详情重试。')
      })
    } catch (error) {
      if (alive.current) setSubmitError(getErrorMessage(error, '提交失败，请稍后重试。'))
    } finally {
      sendingSubmit.current = false
      if (alive.current) setSubmitting(false)
    }
  }

  return <section id="code-workbench" className="panel workbench stack" tabIndex={-1} aria-label="代码工作区">
    <div className="section-head">
      <div><h2>代码工作区</h2><span className="tag mono">Go</span></div>
      <button className="text-button" type="button" disabled={code === template}
        onClick={() => { if (window.confirm('恢复模板会覆盖当前草稿。继续吗？')) updateCode(template) }}>恢复模板</button>
    </div>
    <Suspense fallback={<div role="status" className="state">正在打开编辑器…</div>}><CodeEditor value={code} onChange={updateCode} /></Suspense>
    {draftError && <Notice>浏览器无法保存草稿，请在离开页面前复制代码。</Notice>}
    <details className="run-input-panel">
      <summary>自定义输入 <span>{input ? '已填写' : '可选'}</span></summary>
      <textarea aria-label="自定义输入" rows={3} value={input} onChange={(event) => setInput(event.target.value)}
        placeholder="程序的标准输入，可以留空" />
    </details>
    <div className="stack-inline run-actions">
      <button type="button" className="btn outline" disabled={running || !code.trim()} onClick={runCode}>{running ? '等待运行结果…' : '运行代码'}</button>
      <button type="button" className="btn primary" disabled={submitting || !code.trim()} onClick={sendSubmission}>{submitting ? '提交中…' : '提交判题'}</button>
    </div>
    <Notice>{runError}</Notice>
    {running && <p className="run-progress" role="status"><span className="spinner" />正在等待编译和运行结果。</p>}
    {run && <section className="result-panel" aria-live="polite">
      <div className="section-head"><h3>运行结果</h3>{run.status === 'ok' ? <span className="status good">运行成功</span> : <Status value={run.status} />}</div>
      <pre className="code-output">{run.output || '（无输出）'}</pre>
      <p className="muted">本次等待 {(run.elapsed / 1000).toFixed(2)} 秒{run.status !== 'CE' && ` · 程序执行 ${run.duration_ms} ms`}{run.truncated && ' · 输出已截断'}</p>
    </section>}
    <Notice>{submitError}</Notice>
    {submit && <section className="result-panel" aria-live="polite">
      <div className="section-head"><h3>提交 #{submit.id}</h3><Status value={submit.status} /></div>
      {submit.status === 'Pending' ? <p>提交已保存，{submit.delayed ? '等待系统安排判题。' : '完成后自动更新结果。'}</p>
        : submit.message && <p>{submit.message}</p>}
      <div className="stack-inline">
        <Link className="text-link" to={`/submissions/${submit.id}`}>查看提交详情</Link>
        {submit.status !== 'Pending' && submit.status !== 'AC' && <Link className="text-link" to={`/chat?submission=${submit.id}`}>用 AI 分析这次提交</Link>}
      </div>
    </section>}
  </section>
}

export default function Problem() {
  const { id } = useParams()
  const location = useLocation()
  const { token, userId } = useSession()
  const fetchProblem = useCallback(() => getProblemDetail(id), [id])
  const resource = useResource(fetchProblem)
  const back = safeReturnTo(location.state?.libraryFrom)
  const problem = resource.data?.id === Number(id) ? resource.data : null
  const redirect = new URLSearchParams({ redirect: location.pathname + location.search })

  return <div className="page problem-page">
    <Link className="text-link back-link" to={back}>← 返回题库</Link>
    <PageHeader title={problem?.title || '题目详情'} detail={problem ? `题目 #${problem.id}` : undefined}>
      {token && !resource.loading && problem && <a className="btn outline jump-workbench" href="#code-workbench">去写代码</a>}
    </PageHeader>
    {resource.error?.response?.status === 404 ? <State title="题目不存在"><Link className="btn outline" to={back}>回到题库</Link></State>
      : <ResourceState resource={resource} failure="题目加载失败" />}
    {!resource.loading && !resource.error && problem && <div className="problem-grid">
      <Statement problem={problem} />
      {token ? <Workbench key={`${userId}:${problem.id}`} problem={problem} userId={userId} />
        : <section className="panel"><State title="登录后运行和提交代码">
          <Link className="btn primary" to={`/login?${redirect}`} state={{ libraryFrom: back }}>去登录</Link>
        </State></section>}
    </div>}
  </div>
}
