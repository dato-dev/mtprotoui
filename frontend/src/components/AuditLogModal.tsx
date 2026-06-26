import { useCallback, useEffect, useState } from 'react'
import { api, AuditEntry } from '../api/client'
import Button from './ui/Button'
import { useToast } from './ui/Toast'
import './AuditLogModal.css'

type Props = {
  onClose: () => void
}

function actionLabel(action: string) {
  switch (action) {
    case 'create':
      return 'Добавлен'
    case 'edit':
      return 'Изменён'
    case 'recreate':
      return 'Пересоздан'
    case 'delete':
      return 'Удалён'
    default:
      return action
  }
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export default function AuditLogModal({ onClose }: Props) {
  const toast = useToast()
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setEntries(await api.listAudit())
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Не удалось загрузить журнал')
    } finally {
      setLoading(false)
    }
  }, [toast])

  useEffect(() => {
    void load()
  }, [load])

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal audit-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal__header">
          <h2>Журнал действий</h2>
          <button type="button" className="modal__close" onClick={onClose} aria-label="Закрыть">
            ×
          </button>
        </div>

        <div className="audit-modal__body">
          {loading ? (
            <div className="audit-empty">Загрузка...</div>
          ) : entries.length === 0 ? (
            <div className="audit-empty">Записей пока нет</div>
          ) : (
            <table className="audit-table">
              <thead>
                <tr>
                  <th>Когда</th>
                  <th>Кто</th>
                  <th>Действие</th>
                  <th>Сервер</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e) => (
                  <tr key={e.id}>
                    <td className="audit-table__time">{formatDate(e.created_at)}</td>
                    <td>{e.actor}</td>
                    <td>
                      <span className={`audit-action audit-action--${e.action}`}>
                        {actionLabel(e.action)}
                      </span>
                    </td>
                    <td className="audit-table__server">
                      {e.server_name || '—'}
                      {e.details ? <span className="audit-details"> · {e.details}</span> : null}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        <div className="modal-actions">
          <Button type="button" variant="ghost" onClick={onClose}>
            Закрыть
          </Button>
          <Button type="button" variant="primary" disabled={loading} onClick={load}>
            {loading ? 'Обновление...' : 'Обновить'}
          </Button>
        </div>
      </div>
    </div>
  )
}
