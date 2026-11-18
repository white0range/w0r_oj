<template>
  <div ref="host" class="editor-host"></div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { basicSetup, EditorView } from 'codemirror'
import { completeFromList, ifNotIn, snippetCompletion } from '@codemirror/autocomplete'
import { go, goLanguage } from '@codemirror/lang-go'

const props = defineProps({ modelValue: { type: String, required: true } })
const emit = defineEmits(['update:modelValue'])
const host = ref(null)
let editor

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

onMounted(() => {
  editor = new EditorView({
    parent: host.value,
    doc: props.modelValue,
    extensions: [
      basicSetup,
      go(),
      goLanguage.data.of({ autocomplete: packageCompletion }),
      goLanguage.data.of({
        autocomplete: ifNotIn(['String', 'LineComment', 'BlockComment'], completeFromList([ioSnippet])),
      }),
      EditorView.contentAttributes.of({
        'aria-label': 'Go 代码编辑器',
        autocapitalize: 'off',
        autocomplete: 'off',
        spellcheck: 'false',
      }),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          const code = update.state.doc.toString()
          if (code !== props.modelValue) emit('update:modelValue', code)
        }
      }),
    ],
  })
})

watch(() => props.modelValue, (code) => {
  if (editor && editor.state.doc.toString() !== code) {
    editor.dispatch({ changes: { from: 0, to: editor.state.doc.length, insert: code } })
  }
})

onBeforeUnmount(() => editor?.destroy())
</script>

<style scoped>
.editor-host {
  min-width: 0;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius-sm);
  background: #fff;
}

.editor-host:focus-within {
  border-color: var(--brand);
}

.editor-host :deep(.cm-editor) {
  min-height: 340px;
  max-height: 540px;
}

.editor-host :deep(.cm-editor.cm-focused) {
  outline: none;
}

.editor-host :deep(.cm-scroller) {
  line-height: 1.65;
}
</style>
