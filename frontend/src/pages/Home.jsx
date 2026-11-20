import { useCallback, useEffect, useState } from 'react'
import { Link, useLocation, useSearchParams } from 'react-router'
import { getProblemDetail, getProblems, getProfile, getTags, searchProblems } from '../api'
import Icon from '../components/Icon'
import { Notice, Pager, ResourceState, State, positiveInt, usePageTitle } from '../components/ui'
import { useSession } from '../store'
import { getAcceptanceRate } from '../utils/normalizers'
import { useResource } from '../utils/useResource'

const PAGE_SIZE = 12

export default function Home() {
  const [params, setParams] = useSearchParams()
  const { pathname, search } = useLocation()
  const { userId, token } = useSession()
  const query = (params.get('q') || '').trim().slice(0, 100)
  const tagId = positiveInt(params.get('tag'), 0)
  const page = positiveInt(params.get('page'))
  const listPage = query ? 1 : page
  const [input, setInput] = useState(query)
  usePageTitle('题库')

  const tags = useResource(getTags)
  const activeTag = tags.data?.find((tag) => tag.id === tagId)
  const tagName = activeTag?.name
  const needsTagName = Boolean(query && tagId && !/^#?\d+$/.test(query))
  const waitingForTag = needsTagName && tags.loading
  const needsProgress = Boolean(query && token)
  const loadProgress = useCallback(() => needsProgress ? getProfile() : null, [needsProgress, userId])
  const progress = useResource(loadProgress)

  const load = useCallback(async () => {
    if (!query) return getProblems({ page: listPage, limit: PAGE_SIZE, ...(tagId ? { tag_id: tagId } : {}) })
    if (/^#?\d+$/.test(query)) {
      const id = positiveInt(query.replace('#', ''), 0)
      if (!id) return { total: 0, items: [] }
      try {
        const problem = await getProblemDetail(id)
        const items = !tagId || problem.tags.some((tag) => tag.id === tagId) ? [problem] : []
        return { total: items.length, items }
      } catch (err) {
        if (err.response?.status === 404) return { total: 0, items: [] }
        throw err
      }
    }
    if (needsTagName && !tagName) return { total: 0, items: [] }
    return searchProblems(query, tagName ? [tagName] : [])
  }, [query, listPage, tagId, tagName, needsTagName, userId])
  const result = useResource(load)

  useEffect(() => { setInput(query) }, [query])

  const items = result.data?.items || []
  const available = query ? items.length : result.data?.total || 0
  const pageCount = Math.max(1, Math.ceil(available / PAGE_SIZE))
  const shown = query ? items.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE) : items
  const solved = new Set(progress.data?.id === userId ? progress.data.solvedList : [])
  const knowProgress = Boolean(token && (!query || progress.data?.id === userId))
  const isSolved = (problem) => knowProgress && (query ? solved.has(problem.id) : problem.isAc)
  const total = result.data?.total || 0

  useEffect(() => {
    if (waitingForTag || (needsTagName && tags.error) || result.loading || result.error || page <= pageCount) return
    setParams((previous) => {
      const next = new URLSearchParams(previous)
      if (pageCount > 1) next.set('page', String(pageCount))
      else next.delete('page')
      return next
    }, { replace: true })
  }, [page, pageCount, result.loading, result.error, needsTagName, waitingForTag, tags.error, setParams])

  function apply(next) {
    setParams((previous) => {
      const updated = new URLSearchParams(previous)
      for (const [name, value] of Object.entries({ page: 1, ...next })) {
        if (!value || (name === 'page' && value === 1)) updated.delete(name)
        else updated.set(name, String(value))
      }
      return updated
    })
  }

  return <div className="page library">
    <section className="library-header">
      <div>
        <span className="library-category"><Icon name="code" /> Go 在线判题</span>
        <h1>算法题库</h1>
      </div>
      <Link to="/chat" className="btn on-dark"><Icon name="sparkles" /> AI 学习助手 <Icon name="arrow" /></Link>
    </section>

    <section className="panel catalog" aria-label="题目列表">
      <div className="catalog-tools">
        <form className="search-form" role="search" onSubmit={(event) => { event.preventDefault(); apply({ q: input.trim() }) }}>
          <div className="search-input">
            <Icon name="search" />
            <input aria-label="按题名或题号搜索" placeholder="搜索题名或输入题号" maxLength={100}
              value={input} onChange={(event) => setInput(event.target.value)} />
            {(input || query) && <button type="button" className="icon-button" aria-label="清空搜索" onClick={() => { setInput(''); apply({ q: '' }) }}><Icon name="close" /></button>}
          </div>
          <button className="btn primary">搜索</button>
        </form>
        <label className="catalog-filter"><Icon name="tag" />
          <select aria-label="知识点" value={tagId || ''} onChange={(event) => apply({ tag: event.target.value })}>
            <option value="">全部知识点</option>
            {tagId > 0 && !activeTag && <option value={tagId}>标签 #{tagId}</option>}
            {tags.data?.map((tag) => <option key={tag.id} value={tag.id}>{tag.name}</option>)}
          </select>
        </label>
      </div>
      {tags.error && <div className="panel-note"><Notice>知识点标签加载失败。 <button className="text-button" onClick={tags.reload}>重试</button></Notice></div>}
      <ResourceState resource={waitingForTag ? { loading: true } : result} failure="题库加载失败" />
      {!waitingForTag && !(needsTagName && tags.error) && !result.loading && !result.error && <>
        <div className="catalog-summary">
          <span>{query ? `“${query}” 匹配 ${total} 道题` : `共 ${total} 道题`}
            {query && total > available && ` · 当前显示前 ${available} 道`}</span>
          {knowProgress && <span><Icon name="check" /> 本页通过 {shown.filter(isSolved).length} 道</span>}
        </div>
        {shown.length ? <>
          <div className="problem-table">
            <div className="problem-row problem-columns" aria-hidden="true"><span>题号</span><span>题目</span><span>知识点</span><span>通过率</span></div>
            {shown.map((problem) => <Link key={problem.id} className="problem-row problem-link"
              to={`/problems/${problem.id}`} state={{ libraryFrom: pathname + search }}>
              <span className="problem-number">#{problem.id}</span>
              <span className="problem-name">{isSolved(problem) && <span className="solved-mark" title="已通过" aria-label="已通过"><Icon name="check" /></span>}<strong>{problem.title}</strong></span>
              <span className="row-tags">{problem.tags.map((tag) => <span key={tag.id || tag.name} className="tag">{tag.name}</span>)}</span>
              <span className="problem-rate">{problem.submitCount ? `${getAcceptanceRate(problem)}%` : '—'}</span>
            </Link>)}
          </div>
          <Pager page={page} limit={PAGE_SIZE} total={available} onChange={(number) => apply({ page: number })} />
        </> : <State title={query ? '没有找到匹配的题目' : '当前没有题目'}>
          {(query || tagId > 0) && <button className="btn outline" onClick={() => apply({ q: '', tag: '' })}>清除筛选</button>}
        </State>}
      </>}
    </section>
  </div>
}
