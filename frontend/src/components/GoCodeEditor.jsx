import { useEffect, useRef } from 'react'
import { basicSetup, EditorView } from 'codemirror'
import { completeFromList, ifNotIn, snippetCompletion } from '@codemirror/autocomplete'
import { go, goLanguage } from '@codemirror/lang-go'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { tags } from '@lezer/highlight'

const ink = EditorView.theme({
  '&': { backgroundColor: '#141a2d', color: '#e2e8f8' },
  '.cm-content': { caretColor: '#e2e8f8' },
  '.cm-cursor, .cm-dropCursor': { borderLeftColor: '#d7d9ff' },
  '.cm-gutters': { backgroundColor: '#141a2d', color: '#8695b1', borderRight: '1px solid #252d44' },
  '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: '#212943' },
  '.cm-selectionBackground, &.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground': { backgroundColor: '#374470' },
  '.cm-tooltip': { backgroundColor: '#202941', color: '#e2e8f8', borderColor: '#424e6e' },
  '.cm-tooltip-autocomplete ul li[aria-selected]': { backgroundColor: '#393d76', color: '#fff' },
  '.cm-matchingBracket': { backgroundColor: '#33415c', outline: '1px solid #6f80ab' },
}, { dark: true })

const syntax = HighlightStyle.define([
  { tag: [tags.keyword, tags.modifier], color: '#c5adff' },
  { tag: [tags.string, tags.special(tags.string)], color: '#a6e0be' },
  { tag: [tags.number, tags.bool], color: '#ffd294' },
  { tag: [tags.typeName, tags.className, tags.definition(tags.variableName)], color: '#99ceff' },
  { tag: tags.comment, color: '#a0abc4', fontStyle: 'italic' },
  { tag: tags.operator, color: '#b8c4e1' },
])

const members = {
  fmt: ['Print', 'Printf', 'Println', 'Scan', 'Scanln', 'Fprint', 'Fprintf', 'Fprintln', 'Fscan', 'Sprintf'],
  bufio: ['NewReader', 'NewWriter', 'NewScanner'],
  os: ['Stdin', 'Stdout', 'Stderr'],
  strconv: ['Atoi', 'Itoa', 'ParseInt', 'ParseFloat'],
  strings: ['Split', 'Fields', 'Join', 'TrimSpace', 'Builder', 'NewReader'],
  sort: ['Ints', 'Strings', 'Slice', 'Search'],
  slices: ['Sort', 'SortFunc', 'BinarySearch', 'Reverse', 'Clone'],
}

const packageCompletion = ifNotIn(['String', 'LineComment', 'BlockComment'], (context) => {
  const token = context.matchBefore(/\b(?:fmt|bufio|os|strconv|strings|sort|slices)\.[A-Za-z]*$/)
  if (!token) return null
  const dot = token.text.indexOf('.')
  return {
    from: token.from + dot + 1,
    options: members[token.text.slice(0, dot)].map((label) => ({ label })),
    validFor: /^[A-Za-z]*$/,
  }
})

const ioSnippet = snippetCompletion(
  'in := bufio.NewReader(os.Stdin)\nout := bufio.NewWriter(os.Stdout)\ndefer out.Flush()\n',
  { label: 'fastio', detail: '缓冲输入输出，需要导入 bufio 和 os', type: 'snippet' },
)

export default function GoCodeEditor({ value, onChange }) {
  const host = useRef(null)
  const editor = useRef(null)
  const latest = useRef({ value, onChange })
  latest.current = { value, onChange }

  useEffect(() => {
    const view = new EditorView({
      parent: host.current,
      doc: latest.current.value,
      extensions: [
        basicSetup,
        ink,
        syntaxHighlighting(syntax),
        go(),
        goLanguage.data.of({ autocomplete: packageCompletion }),
        goLanguage.data.of({ autocomplete: ifNotIn(['String', 'LineComment', 'BlockComment'], completeFromList([ioSnippet])) }),
        EditorView.contentAttributes.of({
          'aria-label': 'Go 代码编辑器', autocapitalize: 'off', autocomplete: 'off', spellcheck: 'false',
        }),
        EditorView.updateListener.of((update) => {
          if (!update.docChanged) return
          const code = update.state.doc.toString()
          if (code !== latest.current.value) latest.current.onChange(code)
        }),
      ],
    })
    editor.current = view
    return () => {
      editor.current = null
      view.destroy()
    }
  }, [])

  useEffect(() => {
    const view = editor.current
    if (view && view.state.doc.toString() !== value) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } })
    }
  }, [value])

  return <div ref={host} className="code-editor" />
}
