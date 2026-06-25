import { useCallback, useEffect, useState } from 'react'
import { api, Server } from '../api/client'
import Button from './ui/Button'
import { useToast } from './ui/Toast'
import './ServerLogsModal.css'

type Props = {
  server: Server
  onClose: () => void
}

export default function ServerLogsModal({ server, onClose }: Props) {
  const toast = useToast()
  const [logs, setLogs] = useState('')
  const [loading, setLoading] = useState(false)
  const [loaded, setLoaded] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await api.serverLogs(server.id)
      setLogs(res.logs.trimEnd())
      setLoaded(true)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Не удалось получить логи'
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }, [server.id, toast])

  useEffect(() => {
    void load()
  }, [load])

  async function copyLogs() {
    try {
      await navigator.clipboard.writeText(logs)
      toast.success('Логи скопированы')
    } catch {
      toast.error('Не удалось скопировать')
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal logs-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal__header">
          <h2>
            Логи · <span className="mono">{server.container_name}</span>
          </h2>
          <button type="button" className="modal__close" onClick={onClose} aria-label="Закрыть">
            ×
          </button>
        </div>

        <pre className="logs-modal__body">
          {loading && !loaded ? 'Загрузка логов...' : logs || 'Логи пусты.'}
        </pre>

        <div className="modal-actions">
          <Button type="button" variant="ghost" onClick={onClose}>
            Закрыть
          </Button>
          <Button type="button" variant="secondary" disabled={!logs} onClick={copyLogs}>
            Копировать
          </Button>
          <Button type="button" variant="primary" disabled={loading} onClick={load}>
            {loading ? 'Обновление...' : 'Обновить'}
          </Button>
        </div>
      </div>
    </div>
  )
}
