import './OperationProgress.css'

type Props = {
  operation: string
  message: string
  progress: number
  compact?: boolean
}

function operationTitle(operation: string) {
  switch (operation) {
    case 'deploy':
      return 'Развёртывание'
    case 'delete':
      return 'Удаление'
    default:
      return 'Операция'
  }
}

export default function OperationProgress({ operation, message, progress, compact = false }: Props) {
  const safeProgress = Math.max(0, Math.min(100, progress))

  if (compact) {
    return (
      <div className="op-progress op-progress--compact" title={message}>
        <div className="op-progress__bar">
          <div className="op-progress__fill" style={{ width: `${safeProgress}%` }} />
        </div>
        <span className="op-progress__pct">{safeProgress}%</span>
      </div>
    )
  }

  return (
    <div className={`op-progress op-progress--${operation}`}>
      <div className="op-progress__header">
        <span className="op-progress__title">{operationTitle(operation)}</span>
        <span className="op-progress__pct">{safeProgress}%</span>
      </div>
      <div className="op-progress__bar">
        <div className="op-progress__fill" style={{ width: `${safeProgress}%` }} />
      </div>
      {message && <p className="op-progress__message">{message}</p>}
    </div>
  )
}
