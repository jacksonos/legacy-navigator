import { useEffect, useRef } from 'react'
import { useT } from './i18n'

export default function CodeViewer({ file, lines, highlight }: { file: string; lines: string[]; highlight?: number }) {
  const t = useT()
  const hl = useRef<HTMLDivElement>(null)
  useEffect(() => {
    hl.current?.scrollIntoView({ block: 'center' })
  }, [file, lines, highlight])

  return (
    <div className="viewer" role="region" aria-label={`${t('viewer')}: ${file}`} tabIndex={0}>
      <div className="viewer-name">{file}</div>
      <div className="code">
        {lines.map((text, i) => (
          <div key={i} className={i + 1 === highlight ? 'line hl' : 'line'} ref={i + 1 === highlight ? hl : undefined}>
            <span className="no">{i + 1}</span>
            <span className="tx">{text}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
