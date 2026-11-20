import { useEffect, useRef } from 'react'
import { basicSetup, EditorView } from 'codemirror'
import { completeFromList, ifNotIn, snippetCompletion } from '@codemirror/autocomplete'
import { go, goLanguage } from '@codemirror/lang-go'

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
