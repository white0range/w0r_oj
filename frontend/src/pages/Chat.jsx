import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import {
  createChatSession, deleteChatSession, getChatMessages, getChatPlanFeedback, getChatSession,
  getErrorMessage, listChatSessions, sendChatMessage, submitChatPlanFeedback,
} from '../api'
import Icon from '../components/Icon'
import { Notice, PageHeader, ResourceState, State, formatDate, positiveInt } from '../components/ui'
import { useSession } from '../store'
import { pendingTurn, watchChatTurn } from '../utils/chatTurn'
import { useResource } from '../utils/useResource'

const prompts = ['分析我最近一次没通过的提交', '推荐一道适合我练习的题', '讲讲二分查找容易写错的边界']
const listSessions = () => listChatSessions({ limit: 50 })
const sessionTitle = (session) => session?.title?.trim() || '新会话'

function Feedback({ turnId }) {
  const load = useCallback(async () => {
    try { return await getChatPlanFeedback(turnId) } catch (error) {
      if (error.response?.status === 404) return null
      throw error
    }
  }, [turnId])
  const resource = useResource(load)
  const [saved, setSaved] = useState(null)
  const [comment, setComment] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const active = saved || resource.data
  useEffect(() => { if (resource.data) setComment(resource.data.comment || '') }, [resource.data])

  async function save(helpful) {
    if (busy) return
    setBusy(true)
    setError('')
    try {
      setSaved(await submitChatPlanFeedback(turnId, { helpful, comment: comment.trim() }))
    } catch (err) {
      setError(getErrorMessage(err, '保存反馈失败，请重试。'))
    } finally {
      setBusy(false)
    }
  }
  return <details className="feedback">
    <summary>{active ? '已提交反馈' : '评价这条回复'}</summary>
    <label className="field">补充说明（可选）<input value={comment} onChange={(event) => setComment(event.target.value)} maxLength={1000} /></label>
    <div className="stack-inline">
      <button className="btn outline" aria-pressed={active?.helpful === true} disabled={busy} onClick={() => save(true)}>有帮助</button>
      <button className="btn outline" aria-pressed={active?.helpful === false} disabled={busy} onClick={() => save(false)}>需改进</button>
    </div>
    <Notice>{error}</Notice>
  </details>
}

function getAssistantPayload(value) {
  if (!value) return null
  try { return typeof value === 'string' ? JSON.parse(value) : value } catch { return null }
}

function Message({ item }) {
  const assistant = item.role === 'assistant'
  const payload = assistant ? getAssistantPayload(item.structured_payload) : null
  const tags = Array.isArray(payload?.weak_tags) ? payload.weak_tags : []
  const recommendations = Array.isArray(payload?.recommended_problems) ? payload.recommended_problems : []
  return <article className={`chat-message ${assistant ? 'assistant' : 'user'}`}>
    <div className="message-meta"><strong>{assistant ? '学习助手' : '你'}</strong><span>{formatDate(item.created_at)}</span></div>
    <div className="message-content">{payload?.answer || item.content}</div>
    {tags.length > 0 && <div className="stack-inline">{tags.map((tag, index) => <span className="tag" key={index}>{tag}</span>)}</div>}
    {recommendations.map((problem) => <Link className="recommendation" key={problem.problem_id} to={`/problems/${problem.problem_id}`}>
      <strong>{problem.title || `题目 #${problem.problem_id}`} <Icon name="arrow" /></strong>
      {problem.reason && <span>{problem.reason}</span>}
    </Link>)}
    {assistant && item.turn_id > 0 && <Feedback key={item.turn_id} turnId={item.turn_id} />}
  </article>
}

function Conversation({ id, listEntry, submission, onSelect, onArchive, onUpdateSessions, consumeSubmission }) {
  const load = useCallback(async () => {
    if (!id) return { session: null, items: [] }
    const [session, items] = await Promise.all([getChatSession(id), getChatMessages(id)])
    return { session, items }
  }, [id])
  const thread = useResource(load)
  const reloadThread = thread.reload
  const [draft, setDraft] = useState(submission ? `请帮我分析提交 #${submission} 没通过的原因，并给出修改建议。` : '')
  const [sending, setSending] = useState(false)
  const [archiving, setArchiving] = useState(false)
  const [error, setError] = useState('')
  const [statusError, setStatusError] = useState('')
  const [sentTurn, setSentTurn] = useState(null)
  const [lastQuestion, setLastQuestion] = useState('')
  const [observed, setObserved] = useState(null)
  const [watchVersion, setWatchVersion] = useState(0)
  const temporaryId = useRef(0)
  const active = useRef(true)
  const threadView = useRef(null)
  const didRefresh = useRef(0)
  const composerInput = useRef(null)
  const items = thread.data?.items || []
  const latestMessage = items.findLast((item) => item.turn_id > 0)
  const watchingId = positiveInt(sentTurn?.id || latestMessage?.turn_id, 0)
  const status = observed?.id === watchingId ? observed.status : sentTurn?.status
  const isPending = watchingId > 0 && (pendingTurn(status) || (!status && latestMessage?.role === 'user' && !statusError))
  const busy = sending || archiving || isPending
  const gone = [403, 404].includes(thread.error?.response?.status)

  useEffect(() => () => { active.current = false }, [])
  useEffect(() => {
    if (submission) setDraft(`请帮我分析提交 #${submission} 没通过的原因，并给出修改建议。`)
  }, [submission])
  useEffect(() => {
    const view = threadView.current
    if (view) view.scrollTop = view.scrollHeight
  }, [items.length, isPending])

  useEffect(() => {
    if (!watchingId) return
    return watchChatTurn(watchingId, (turn) => {
      setObserved(turn)
      setStatusError('')
      if (!pendingTurn(turn.status) && didRefresh.current !== watchingId) {
        didRefresh.current = watchingId
        reloadThread()
        onUpdateSessions()
      }
    }, (failed) => setStatusError(failed ? '无法读取本回合的状态。' : '同步暂时中断，正在重试。'))
  }, [watchingId, watchVersion, reloadThread, onUpdateSessions])

  async function submit(event) {
    event?.preventDefault()
    const content = draft.trim()
    if (!content || busy || gone) return
    setSending(true)
    setError('')
    try {
      let sessionId = id || temporaryId.current
      if (!sessionId) {
        const created = await createChatSession({ title: '' })
        sessionId = positiveInt(created.id, 0)
        if (!sessionId) throw new Error('Missing session ID')
        temporaryId.current = sessionId
      }
      const response = await sendChatMessage(sessionId, { content })
      if (!active.current) return
      setLastQuestion(content)
      setDraft('')
      if (!id) onSelect(sessionId)
      else {
        consumeSubmission()
        setSentTurn({ id: positiveInt(response.turn_id, 0), status: response.status })
        reloadThread()
      }
      onUpdateSessions()
    } catch (err) {
      if (active.current) setError(getErrorMessage(err, '发送失败，草稿已保留。'))
    } finally {
      if (active.current) setSending(false)
    }
  }

  async function archive() {
    if (!id || busy || !window.confirm('归档当前会话？')) return
    setArchiving(true)
    setError('')
    try {
      await deleteChatSession(id)
      if (active.current) onArchive(id)
    } catch (err) {
      if (active.current) {
        setError(getErrorMessage(err, '归档失败，请重试。'))
        setArchiving(false)
      }
    }
  }

  return <section className="panel conversation">
    <header className="conversation-head"><h2>{id ? sessionTitle(thread.data?.session || listEntry) : '新会话'}</h2>
      {id > 0 && <div className="stack-inline">
        <button className="text-button" disabled={thread.loading} onClick={() => { setStatusError(''); reloadThread(); setWatchVersion((value) => value + 1) }}>刷新</button>
        <button className="text-button danger-text" disabled={busy} onClick={archive}>{archiving ? '归档中…' : '归档'}</button>
      </div>}
    </header>
    <div className="thread" ref={threadView}>
      {id > 0 && !items.length && <ResourceState resource={thread} loading="正在读取消息…" failure="会话加载失败" />}
      {thread.data?.session?.summary_text && <details className="memory-note"><summary>会话摘要</summary><p>{thread.data.session.summary_text}</p></details>}
      {!thread.loading && !thread.error && !items.length && <div className="chat-welcome">
        <span className="chat-symbol"><Icon name="sparkles" /></span><h3>从一个算法问题开始</h3>
        <div className="prompt-list">{prompts.map((prompt) => <button className="btn outline" key={prompt}
          disabled={busy} onClick={() => { setDraft(prompt); composerInput.current?.focus() }}>{prompt}<Icon name="arrow" /></button>)}</div>
      </div>}
      {items.map((item) => <Message key={item.id} item={item} />)}
      {isPending && <div className="conversation-status" role="status"><span className="spinner" />正在生成回复…</div>}
      {status === 'failed' && <Notice>这次回复生成失败。 <button className="text-button" onClick={() => {
        setDraft(lastQuestion || items.findLast((item) => item.role === 'user' && item.turn_id === watchingId)?.content || '')
        composerInput.current?.focus()
      }}>恢复上次提问</button></Notice>}
      <Notice>{statusError}</Notice>
    </div>
    <form className="composer stack" onSubmit={submit}>
      <Notice>{error}</Notice>
      <textarea ref={composerInput} aria-label="向 AI 学习助手提问" rows={3} maxLength={8000} value={draft} placeholder="输入你的问题…"
        onChange={(event) => setDraft(event.target.value)} disabled={gone}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) void submit(event)
        }} />
      <div className="section-head"><span className="muted">Enter 发送 · Shift + Enter 换行</span>
        <button className="btn primary" disabled={busy || gone || !draft.trim()}>{sending ? '发送中…' : '发送'}<Icon name="arrow" /></button>
      </div>
    </form>
  </section>
}

export default function Chat() {
  const { userId } = useSession()
  const [params, setParams] = useSearchParams()
  const sessions = useResource(listSessions)
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState('')
  const storageKey = `gojo:chatSessionId:${userId}`
  let stored = 0
  try { stored = positiveInt(localStorage.getItem(storageKey) || localStorage.getItem('gojo:chatSessionId'), 0) } catch {}
  const recent = sessions.data || []
  const selected = params.has('session') ? positiveInt(params.get('session'), 0)
    : (recent.find((item) => item.id === stored) || recent[0])?.id || 0
  const submission = positiveInt(params.get('submission'), 0)

  function select(id) {
    try { localStorage.setItem(storageKey, String(id)) } catch {}
    setParams((previous) => {
      const next = new URLSearchParams(previous)
      next.set('session', String(id))
      next.delete('submission')
      return next
    }, { replace: true })
  }

  async function create() {
    setCreating(true)
    setCreateError('')
    try {
      const session = await createChatSession({ title: '' })
      select(session.id)
      sessions.reload()
    } catch (error) {
      setCreateError(getErrorMessage(error, '新建会话失败，请重试。'))
    } finally {
      setCreating(false)
    }
  }

  return <div className="page chat-page">
    <PageHeader title="AI 学习助手" detail="分析提交、理解算法，或找一道适合继续练的题。">
      <button className="btn primary" disabled={creating} onClick={create}>{creating ? '新建中…' : '新建会话'}</button>
    </PageHeader>
    <Notice>{createError}</Notice>
    <div className="chat-grid">
      <aside className={`panel chat-history${!sessions.loading && !sessions.error && !recent.length ? ' empty' : ''}`}>
        <div className="section-head"><h2>会话</h2><button className="icon-button" aria-label="刷新会话" disabled={sessions.loading} onClick={sessions.reload}><Icon name="refresh" /></button></div>
        <ResourceState resource={sessions} loading="读取会话…" failure="会话列表加载失败" />
        {!sessions.loading && !sessions.error && !recent.length && <State title="还没有会话" />}
        {recent.map((session) => <button key={session.id} className="session-item" aria-current={selected === session.id ? 'true' : undefined}
          onClick={() => select(session.id)}>
          <strong>{sessionTitle(session)}</strong><span>{formatDate(session.last_message_at || session.updated_at)}</span>
        </button>)}
      </aside>
      <Conversation key={selected} id={selected} submission={submission}
        listEntry={recent.find((entry) => entry.id === selected)} onSelect={select}
        onUpdateSessions={sessions.reload}
        onArchive={(archived) => { select(recent.find((entry) => entry.id !== archived)?.id || 0); sessions.reload() }}
        consumeSubmission={() => {
          if (!params.has('submission')) return
          setParams((previous) => { const next = new URLSearchParams(previous); next.delete('submission'); return next }, { replace: true })
        }} />
    </div>
  </div>
}
