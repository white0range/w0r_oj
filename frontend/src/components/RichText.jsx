const labels = /^(输入格式|输出格式|输入示例|输出示例|样例输入|样例输出|示例|数据范围|提示|结论|原因|验证结果|修改建议)[：:]?\s*$/

function inline(text) {
  return text.split(/(`[^`\n]+`|\*\*[^*\n]+\*\*)/g).map((part, index) => {
    if (index % 2 === 0) return part
    return part.startsWith('`') ? <code key={index}>{part.slice(1, -1)}</code> : <strong key={index}>{part.slice(2, -2)}</strong>
  })
}

function TextBlock({ text }) {
  return text.split(/\n{2,}|\n(?=#{1,4}\s)/).map((paragraph, index) => {
    const [first, ...body] = paragraph.split('\n')
    const line = first.trim()
    const heading = line.match(/^#{1,4}\s+(.+)/)?.[1] || (labels.test(line) ? line.replace(/[：:]$/, '') : '')
    if (!heading) return <p key={index}>{inline(paragraph)}</p>
    return <div className="rich-section" key={index}>
      <h3>{inline(heading)}</h3>
      {body.length > 0 && <p>{inline(body.join('\n'))}</p>}
    </div>
  })
}

export default function RichText({ text, className = '' }) {
  const chunks = (text || '').replace(/\r\n/g, '\n').split('```')
  return <div className={`rich-text ${className}`}>
    {chunks.map((chunk, index) => {
      if (index % 2 === 0) return chunk.trim() && <TextBlock text={chunk.trim()} key={index} />
      if (index === chunks.length - 1) return <p key={index}>{inline('```' + chunk)}</p>
      return <pre key={index}><code>{chunk.replace(/^(?:[A-Za-z][\w+-]*)?\n/, '').replace(/\n$/, '')}</code></pre>
    })}
  </div>
}
