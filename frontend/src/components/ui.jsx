import { useEffect } from 'react'
import { getErrorMessage } from '../api'

export function usePageTitle(title) {
  useEffect(() => { document.title = `${title} | w0r_oj` }, [title])
}

export function PageHeader({ title, detail, children }) {
  usePageTitle(title)
  return <header className="page-header">
    <div><h1>{title}</h1>{detail && <p>{detail}</p>}</div>
    {children && <div className="header-actions">{children}</div>}
  </header>
}

export function Notice({ children, kind = 'error' }) {
  return children ? <div className={`notice ${kind}`} role={kind === 'error' ? 'alert' : 'status'}>{children}</div> : null
}

export function State({ title, detail, children }) {
  return <div className="state"><strong>{title}</strong>{detail && <p>{detail}</p>}{children}</div>
}

export function ResourceState({ resource, loading = '正在加载…', failure = '加载失败' }) {
  if (resource.loading) return <div role="status" className="state"><span className="spinner" />{loading}</div>
  if (!resource.error) return null
  return <State title={failure} detail={getErrorMessage(resource.error, '请检查网络后重试。')}>
    <button className="btn outline" onClick={resource.reload}>重试</button>
  </State>
}

export function Pager({ page, total, limit, onChange, busy = false }) {
  const pages = Math.max(1, Math.ceil(total / limit))
  if (pages === 1) return null
  const start = Math.max(1, Math.min(page - 2, pages - 4))
  const options = Array.from({ length: Math.min(pages, 5) }, (_, index) => start + index)
  return <nav className="pager" aria-label="分页">
    <span className="muted">第 {page} / {pages} 页</span>
    <div className="pager-buttons">
      <button type="button" disabled={busy || page === 1} onClick={() => onChange(page - 1)}>上一页</button>
      {options.map((number) => <button type="button" key={number} disabled={busy}
        aria-current={number === page ? 'page' : undefined} onClick={() => onChange(number)}>{number}</button>)}
      <button type="button" disabled={busy || page >= pages} onClick={() => onChange(page + 1)}>下一页</button>
    </div>
  </nav>
}

const statuses = {
  AC: '已通过', WA: '答案错误', CE: '编译错误', RE: '运行错误', TLE: '超时',
  MLE: '内存超限', OLE: '输出超限', SE: '系统错误', Pending: '判题中',
}

export function Status({ value }) {
  const tone = value === 'AC' ? 'good' : value === 'Pending' ? 'waiting' : 'bad'
  return <span className={`status ${tone}`} title={value}>{statuses[value] || value}</span>
}

export function formatDate(value) {
  const date = new Date(value)
  return value && !Number.isNaN(date.getTime()) ? date.toLocaleString('zh-CN', { hour12: false }) : '—'
}

export function positiveInt(value, fallback = 1) {
  const number = Number(value)
  return Number.isSafeInteger(number) && number > 0 ? number : fallback
}
