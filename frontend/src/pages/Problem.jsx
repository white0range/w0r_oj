import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react'
import { Link, useLocation, useParams } from 'react-router'
import { getErrorMessage, getProblemDetail, runProblemCode, submitCode } from '../api'
import Icon from '../components/Icon'
import RichText from '../components/RichText'
import { Notice, ResourceState, State, Status, canReviewSubmission, usePageTitle } from '../components/ui'
import { useSession } from '../store'
import { safeReturnTo } from '../utils/navigation'
import { getAcceptanceRate } from '../utils/normalizers'
import { useResource } from '../utils/useResource'
import { watchSubmissionStatus } from '../utils/submissionStatus'

const CodeEditor = lazy(() => import('../components/GoCodeEditor'))
const template = 'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("Hello w0r_oj")\n}\n'
const paneTabs = [['statement', '题面', 'book'], ['code', '代码', 'code'], ['results', '结果', 'terminal']]
const consoleTabs = [['input', '自定义输入'], ['run', '运行输出'], ['judge', '判题结果']]
const MIN_WIDTH = 30
const MAX_WIDTH = 62

function Statement({ problem }) {
  return <section className="panel statement" id="workspace-statement">
    <div className="statement-head">
      <h2><Icon name="book" />题目描述</h2>
      <div className="problem-facts">
        <span><Icon name="clock" />{problem.timeLimit} ms</span>
        <span>{problem.memoryLimit} MB</span>
        {problem.submitCount > 0 && <span>通过率 {getAcceptanceRate(problem)}%</span>}
      </div>
      {problem.tags.length > 0 && <div className="stack-inline">{problem.tags.map((tag) => <span key={tag.id} className="tag">{tag.name}</span>)}</div>}
    </div>
    <RichText text={problem.description} className="statement-body" />
  </section>
}

function Splitter({ workspace, width, changeWidth }) {
  function move(event) {
    if (!event.currentTarget.hasPointerCapture(event.pointerId)) return
    const bounds = workspace.current.getBoundingClientRect()
    if (bounds.width) changeWidth(100 * (event.clientX - bounds.left - 6) / bounds.width)
  }

  return <div className="workspace-divider" role="separator" aria-label="调整题面宽度" aria-controls="workspace-statement"
    aria-orientation="vertical" aria-valuemin={MIN_WIDTH} aria-valuemax={MAX_WIDTH} aria-valuenow={width}
    aria-valuetext={`题面占 ${width}%`} tabIndex={0}
    onPointerDown={(event) => {
      if (event.button === 0) event.currentTarget.setPointerCapture(event.pointerId)
    }}
    onPointerMove={move}
    onKeyDown={(event) => {
      const next = { ArrowLeft: width - 2, ArrowRight: width + 2, Home: MIN_WIDTH, End: MAX_WIDTH }[event.key]
      if (next === undefined) return
      event.preventDefault()
      changeWidth(next)
    }}>
    <Icon name="grip" />
  </div>
}

function Workspace({ problem, userId, initialPane }) {
  const draftKey = `gojo:draft:${userId}:${problem.id}:go`
  const [draft, setDraft] = useState(() => {
    try {
      const cached = localStorage.getItem(draftKey)
      return { code: cached ?? template, state: cached === null ? 'template' : 'saved' }
    } catch { return { code: template, state: 'unavailable' } }
  })
  const [width, setWidth] = useState(41)
  const [pane, setPane] = useState(initialPane)
  const [consoleTab, setConsoleTab] = useState('input')
  const [consoleOpen, setConsoleOpen] = useState(false)
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
  const workspace = useRef(null)
  const executionPanel = useRef(null)
  const code = draft.code

  useEffect(() => {
    return () => {
      alive.current = false
      pendingRun.current?.abort()
      stopWatching.current?.()
    }
  }, [])

  function selectPane(next) {
    setPane(next)
    requestAnimationFrame(() => {
      if (window.matchMedia('(max-width: 820px)').matches) workspace.current?.scrollIntoView({ block: 'nearest' })
    })
  }

  function selectConsole(tab) {
    setConsoleTab(tab)
    setConsoleOpen(true)
    selectPane('results')
    requestAnimationFrame(() => {
      if (!window.matchMedia('(max-width: 820px)').matches) executionPanel.current?.scrollIntoView({ block: 'nearest' })
    })
  }

  function updateCode(next) {
    try {
      localStorage.setItem(draftKey, next)
      setDraft({ code: next, state: 'saved' })
    } catch {
      setDraft({ code: next, state: 'unavailable' })
    }
  }

  async function runCode() {
    if (!code.trim() || pendingRun.current) return
    const controller = new AbortController()
    pendingRun.current = controller
    selectConsole('run')
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
    selectConsole('judge')
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

  return <div className="workspace" ref={workspace} data-pane={pane} style={{ '--statement-width': `${width}%` }}>
    <nav className="workspace-nav segment" aria-label="写题面板">
      {paneTabs.map(([key, name, icon]) => <button key={key} type="button" aria-pressed={pane === key}
        onClick={() => selectPane(key)}><Icon name={icon} />{name}</button>)}
    </nav>
    <Statement problem={problem} />
    <Splitter workspace={workspace} width={width} changeWidth={(next) => setWidth(Math.round(Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, next))))} />
    <div className="workbench">
      <section className="editor-panel" aria-label="代码工作区">
        <header className="editor-head">
          <div><Icon name="code" /><h2>代码工作区</h2><span className="editor-file">main.go</span></div>
          <div className="editor-options">
            <span className={`draft-status ${draft.state}`} role="status">
              {draft.state === 'saved' ? '草稿已保存' : draft.state === 'template' ? 'Go 模板' : '草稿无法保存'}
            </span>
            <button className="text-button" type="button" disabled={code === template}
              onClick={() => { if (window.confirm('恢复模板会覆盖当前草稿。继续吗？')) updateCode(template) }}>恢复模板</button>
          </div>
        </header>
        <Suspense fallback={<div role="status" className="state">正在打开编辑器…</div>}><CodeEditor value={code} onChange={updateCode} /></Suspense>
        {draft.state === 'unavailable' && <Notice>浏览器无法保存草稿，请在离开页面前复制代码。</Notice>}
      </section>

      <div className="workbench-actions">
        <button type="button" className="btn ghost" onClick={() => selectConsole('input')}><Icon name="terminal" />输入</button>
        <div className="stack-inline">
          <button type="button" className="btn outline" disabled={running || !code.trim()} onClick={runCode}><Icon name="play" />{running ? '运行中…' : '运行代码'}</button>
          <button type="button" className="btn primary" disabled={submitting || !code.trim()} onClick={sendSubmission}><Icon name="upload" />{submitting ? '提交中…' : '提交判题'}</button>
        </div>
      </div>

      <section ref={executionPanel} className="panel console" aria-label="执行面板" data-expanded={consoleOpen}>
        <div className="console-head">
          <nav className="segment" aria-label="执行结果">
            {consoleTabs.map(([key, label]) => <button key={key} type="button" aria-pressed={consoleTab === key}
              onClick={() => selectConsole(key)}>{label}</button>)}
          </nav>
          <button type="button" className="text-button console-toggle" aria-controls="console-body"
            aria-expanded={consoleOpen} onClick={() => setConsoleOpen((open) => !open)}>{consoleOpen ? '收起' : '展开'}</button>
        </div>
        <div className="console-body" id="console-body">
          {consoleTab === 'input' && <label className="console-input">程序的标准输入
            <textarea aria-label="自定义输入" rows={4} value={input} onChange={(event) => setInput(event.target.value)}
              placeholder="没有输入时可以留空" />
          </label>}
          {consoleTab === 'run' && <div className="stack">
            <Notice>{runError}</Notice>
            {running && <p className="run-progress" role="status"><span className="spinner" />正在等待编译和运行结果。</p>}
            {!running && !run && !runError && <p className="muted">点击「运行代码」查看输出。</p>}
            {run && <div className="result-panel" aria-live="polite">
              <div className="section-head"><h3>运行结果</h3>{run.status === 'ok' ? <span className="status good">运行成功</span> : <Status value={run.status} />}</div>
              <pre className="code-output">{run.output || '（无输出）'}</pre>
              <p className="muted">本次等待 {(run.elapsed / 1000).toFixed(2)} 秒{run.status !== 'CE' && ` · 程序执行 ${run.duration_ms} ms`}{run.truncated && ' · 输出已截断'}</p>
            </div>}
          </div>}
          {consoleTab === 'judge' && <div className="stack">
            <Notice>{submitError}</Notice>
            {submitting && <p className="run-progress" role="status"><span className="spinner" />正在提交代码…</p>}
            {!submitting && !submit && !submitError && <p className="muted">提交代码后，判题结果会在这里自动更新。</p>}
            {!submitting && submit && <div className="result-panel" aria-live="polite">
              <div className="section-head"><h3>提交 #{submit.id}</h3><Status value={submit.status} /></div>
              {submit.status === 'Pending' ? <p>提交已保存，{submit.delayed ? '等待系统安排判题。' : '完成后自动更新结果。'}</p>
                : submit.message && <p className="verdict-message">{submit.message}</p>}
              <div className="stack-inline">
                <Link className="text-link" to={`/submissions/${submit.id}`}>查看提交详情</Link>
                {canReviewSubmission(submit.status) && <Link className="btn outline sm" to={`/chat?submission=${submit.id}`}><Icon name="sparkles" />用 AI 分析这次提交</Link>}
              </div>
            </div>}
          </div>}
        </div>
      </section>
    </div>
  </div>
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
  usePageTitle(problem?.title || '题目详情')

  return <div className="page problem-page">
    <header className="workspace-heading">
      <Link className="text-link back-link" to={back}>← 返回题库</Link>
      <div><span className="problem-number">#{id}</span><h1>{problem?.title || '题目详情'}</h1></div>
    </header>
    {resource.error?.response?.status === 404 ? <State title="题目不存在"><Link className="btn outline" to={back}>回到题库</Link></State>
      : <ResourceState resource={resource} failure="题目加载失败" />}
    {!resource.loading && !resource.error && problem && (token ?
      <Workspace key={`${userId}:${problem.id}`} problem={problem} userId={userId} initialPane={location.state?.workbenchPane === 'code' ? 'code' : 'statement'} /> :
      <div className="problem-grid guest-workspace"><Statement problem={problem} />
        <section className="panel"><State title="登录后运行和提交代码">
          <Link className="btn primary" to={`/login?${redirect}`} state={{ libraryFrom: back, workbenchPane: 'code' }}>去登录</Link>
        </State></section>
      </div>)}
  </div>
}
