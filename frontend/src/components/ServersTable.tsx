import { Fragment, useState } from 'react'
import { api, ContainerStats, Server } from '../api/client'
import OperationProgress from './OperationProgress'
import ServerLogsModal from './ServerLogsModal'
import Button from './ui/Button'
import { useConfirm } from './ui/ConfirmDialog'
import { useToast } from './ui/Toast'
import './ServersTable.css'

export type SortKey = 'name' | 'host' | 'status' | 'deploy' | 'created'
export type SortDir = 'asc' | 'desc'

type StatsState = { loading: boolean; data?: ContainerStats; error?: string }

type Props = {
  servers: Server[]
  onChanged: () => void
  selected: Set<string>
  onToggleSelect: (id: string) => void
  onToggleSelectAll: () => void
  allSelected: boolean
  sortKey: SortKey
  sortDir: SortDir
  onSort: (key: SortKey) => void
  onEdit: (server: Server) => void
}

function statusLabel(status: string) {
  switch (status) {
    case 'online':
      return 'Онлайн'
    case 'degraded':
      return 'Деградация'
    case 'offline':
      return 'Офлайн'
    default:
      return 'Неизвестно'
  }
}

function proxyTypeLabel(type: string) {
  return type === 'tg-ws-proxy' ? 'TG WS Proxy' : 'MTG'
}

function proxyImage(type: string) {
  return type === 'tg-ws-proxy' ? 'dato1/tg-ws-proxy:latest' : 'nineseconds/mtg:2'
}

function deployLabel(status: string) {
  switch (status) {
    case 'ready':
      return 'Готов'
    case 'deploying':
      return 'Развёртывание'
    case 'deleting':
      return 'Удаление'
    case 'failed':
      return 'Ошибка'
    default:
      return status
  }
}

function isOperationActive(server: Server) {
  return server.deploy_status === 'deploying' || server.deploy_status === 'deleting'
}

function checkLabel(status: string, rttMs?: number) {
  switch (status) {
    case 'ok':
      return rttMs != null ? `${Math.round(rttMs)} ms` : 'OK'
    case 'fail':
      return 'Нет'
    default:
      return '—'
  }
}

function formatDate(value?: string) {
  if (!value) return '—'
  return new Date(value).toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export default function ServersTable({
  servers,
  onChanged,
  selected,
  onToggleSelect,
  onToggleSelectAll,
  allSelected,
  sortKey,
  sortDir,
  onSort,
  onEdit,
}: Props) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [busyId, setBusyId] = useState<string | null>(null)
  const [qrOpen, setQrOpen] = useState<Set<string>>(new Set())
  const [logsServer, setLogsServer] = useState<Server | null>(null)
  const [stats, setStats] = useState<Record<string, StatsState>>({})
  const confirm = useConfirm()
  const toast = useToast()

  async function loadStats(id: string) {
    setStats((prev) => ({ ...prev, [id]: { loading: true } }))
    try {
      const data = await api.serverStats(id)
      setStats((prev) => ({ ...prev, [id]: { loading: false, data } }))
    } catch (err) {
      setStats((prev) => ({
        ...prev,
        [id]: { loading: false, error: err instanceof Error ? err.message : 'Ошибка' },
      }))
    }
  }

  function sortIndicator(key: SortKey) {
    if (key !== sortKey) return ''
    return sortDir === 'asc' ? ' ▲' : ' ▼'
  }

  function toggleQR(id: string) {
    setQrOpen((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  function toggleExpanded(id: string) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  async function copyLink(link: string) {
    try {
      await navigator.clipboard.writeText(link)
      toast.success('Ссылка скопирована')
    } catch {
      toast.error('Не удалось скопировать')
    }
  }

  async function handleDelete(server: Server) {
    const ok = await confirm({
      title: 'Удалить сервер?',
      message: `Сервер «${server.name}» будет удалён из консоли. На VPS остановится контейнер ${server.container_name} и удалится Docker-образ ${proxyImage(server.proxy_type)}.`,
      confirmLabel: 'Удалить',
      variant: 'danger',
    })
    if (!ok) return

    setBusyId(server.id)
    try {
      await api.deleteServer(server.id)
      toast.info(`Удаление «${server.name}» запущено`)
      onChanged()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Ошибка удаления')
    } finally {
      setBusyId(null)
    }
  }

  async function handleRecreate(server: Server) {
    const willPickSNI = server.proxy_type !== 'tg-ws-proxy' || server.fake_tls
    const ok = await confirm({
      title: 'Пересоздать прокси?',
      message: willPickSNI
        ? 'Будут сгенерированы новый secret и SNI. Старая ссылка tg://proxy перестанет работать.'
        : 'Будет сгенерирован новый secret. Старая ссылка tg://proxy перестанет работать.',
      confirmLabel: 'Пересоздать',
      variant: 'primary',
    })
    if (!ok) return

    setBusyId(server.id)
    try {
      await api.recreateServer(server.id)
      toast.info('Развёртывание запущено')
      onChanged()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Ошибка пересоздания')
    } finally {
      setBusyId(null)
    }
  }

  async function handleHealth(server: Server) {
    setBusyId(server.id)
    try {
      const res = await api.checkHealth(server.id)
      toast.info(`Проверка: ${statusLabel(res.status).toLowerCase()}`)
      onChanged()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Ошибка проверки')
    } finally {
      setBusyId(null)
    }
  }

  return (
    <div className="servers-table-wrap">
      <table className="servers-table">
        <colgroup>
          <col className="cg-check" />
          <col className="cg-expand" />
          <col className="cg-name" />
          <col className="cg-address" />
          <col className="cg-status" />
          <col className="cg-ping" />
          <col className="cg-tcp" />
          <col className="cg-deploy" />
          <col className="cg-checked" />
          <col className="cg-actions" />
        </colgroup>
        <thead>
          <tr>
            <th className="col-check">
              <input
                type="checkbox"
                checked={allSelected}
                onChange={onToggleSelectAll}
                aria-label="Выбрать все"
              />
            </th>
            <th className="col-expand" />
            <th className="th-sort" onClick={() => onSort('name')}>
              Сервер{sortIndicator('name')}
            </th>
            <th className="th-sort" onClick={() => onSort('host')}>
              Адрес{sortIndicator('host')}
            </th>
            <th className="th-sort" onClick={() => onSort('status')}>
              Статус{sortIndicator('status')}
            </th>
            <th>Ping</th>
            <th>TCP</th>
            <th className="th-sort" onClick={() => onSort('deploy')}>
              Деплой{sortIndicator('deploy')}
            </th>
            <th>Проверка</th>
            <th className="col-actions" />
          </tr>
        </thead>
        <tbody>
          {servers.map((server) => {
            const isOpen = expanded.has(server.id) || isOperationActive(server)
            const busy = busyId === server.id || isOperationActive(server)
            const showProgress = isOperationActive(server) && server.operation

            return (
              <Fragment key={server.id}>
                <tr
                  className={`servers-table__row ${isOpen ? 'is-expanded' : ''}`}
                  onClick={() => toggleExpanded(server.id)}
                >
                  <td className="col-check" onClick={(e) => e.stopPropagation()}>
                    <input
                      type="checkbox"
                      checked={selected.has(server.id)}
                      onChange={() => onToggleSelect(server.id)}
                      aria-label={`Выбрать ${server.name}`}
                    />
                  </td>
                  <td className="col-expand">
                    <button
                      type="button"
                      className={`expand-btn ${isOpen ? 'is-open' : ''}`}
                      aria-label={isOpen ? 'Свернуть' : 'Развернуть'}
                      onClick={(e) => {
                        e.stopPropagation()
                        toggleExpanded(server.id)
                      }}
                    >
                      ›
                    </button>
                  </td>
                  <td className="col-name">
                    <span className="server-name">{server.name}</span>
                    {(server.tags?.length ?? 0) > 0 && (
                      <span className="server-tags">
                        {server.tags.map((tag) => (
                          <span key={tag} className="server-tag">
                            {tag}
                          </span>
                        ))}
                      </span>
                    )}
                  </td>
                  <td className="mono">
                    {server.host}:{server.mtproto_port || 443}
                  </td>
                  <td>
                    <span className={`pill pill--${server.status}`}>
                      <span className="pill__dot" />
                      {statusLabel(server.status)}
                    </span>
                  </td>
                  <td>
                    <span className={`check check--${server.ping_status || 'unknown'}`}>
                      {checkLabel(server.ping_status, server.ping_rtt_ms)}
                    </span>
                  </td>
                  <td>
                    <span className={`check check--${server.tcp_status || 'unknown'}`}>
                      {server.tcp_status === 'ok' ? 'Открыт' : server.tcp_status === 'fail' ? 'Закрыт' : '—'}
                    </span>
                  </td>
                  <td className="col-deploy">
                    <span className={`pill pill--deploy pill--${server.deploy_status}`}>
                      {deployLabel(server.deploy_status)}
                    </span>
                    {showProgress && (
                      <OperationProgress
                        compact
                        operation={server.operation}
                        message={server.operation_message}
                        progress={server.operation_progress}
                      />
                    )}
                  </td>
                  <td className="muted-cell">{formatDate(server.last_check_at)}</td>
                  <td className="col-actions" onClick={(e) => e.stopPropagation()}>
                    <Button variant="ghost" size="sm" disabled={busy} onClick={() => handleHealth(server)}>
                      ↻
                    </Button>
                  </td>
                </tr>

                {isOpen && (
                  <tr className="servers-table__details-row">
                    <td colSpan={10}>
                      <div className="server-details">
                        {showProgress && (
                          <div className="server-details__progress">
                            <OperationProgress
                              operation={server.operation}
                              message={server.operation_message}
                              progress={server.operation_progress}
                            />
                          </div>
                        )}

                        <div className="server-details__grid">
                          <div className="detail-cell">
                            <span className="detail-label">Тип прокси</span>
                            <span className="detail-value">
                              {proxyTypeLabel(server.proxy_type)}
                              {server.proxy_type === 'tg-ws-proxy' && server.fake_tls ? ' · Fake TLS' : ''}
                            </span>
                          </div>
                          <div className="detail-cell">
                            <span className="detail-label">SNI</span>
                            <span className="detail-value">{server.sni_domain || '—'}</span>
                          </div>
                          <div className="detail-cell">
                            <span className="detail-label">SSH</span>
                            <span className="detail-value">
                              {server.ssh_user}@{server.host}:{server.ssh_port}
                            </span>
                          </div>
                          <div className="detail-cell">
                            <span className="detail-label">Контейнер</span>
                            <span className="detail-value mono">{server.container_name}</span>
                          </div>
                          <div className="detail-cell">
                            <span className="detail-label">Создан</span>
                            <span className="detail-value">{formatDate(server.created_at)}</span>
                          </div>
                          <div className="detail-cell">
                            <span className="detail-label">Авторизация</span>
                            <span className="detail-value">
                              {server.ssh_auth_type === 'key' ? 'SSH-ключ' : 'Пароль'}
                            </span>
                          </div>
                          <div className="detail-cell detail-cell--full">
                            <span className="detail-label">Secret</span>
                            <span className="detail-value mono" title={server.secret}>
                              {server.secret || '—'}
                            </span>
                          </div>
                        </div>

                        {server.proxy_link && (
                          <>
                            <div className="server-details__proxy">
                              <input readOnly value={server.proxy_link} onFocus={(e) => e.target.select()} />
                              <Button variant="secondary" size="sm" onClick={() => copyLink(server.proxy_link)}>
                                Копировать
                              </Button>
                              <Button variant="ghost" size="sm" onClick={() => toggleQR(server.id)}>
                                {qrOpen.has(server.id) ? 'Скрыть QR' : 'QR-код'}
                              </Button>
                            </div>
                            {qrOpen.has(server.id) && (
                              <div className="server-details__qr">
                                <img
                                  src={api.qrUrl(server.id)}
                                  alt="QR-код прокси"
                                  width={200}
                                  height={200}
                                />
                                <span className="server-details__qr-hint">
                                  Отсканируйте в Telegram, чтобы подключить прокси
                                </span>
                              </div>
                            )}
                          </>
                        )}

                        {server.last_error && <div className="server-details__error">{server.last_error}</div>}

                        <div className="server-details__metrics">
                          <div className="server-details__metrics-head">
                            <span className="detail-label">Метрики контейнера</span>
                            <Button
                              variant="ghost"
                              size="sm"
                              disabled={stats[server.id]?.loading}
                              onClick={() => loadStats(server.id)}
                            >
                              {stats[server.id]?.loading
                                ? 'Загрузка...'
                                : stats[server.id]?.data
                                  ? 'Обновить'
                                  : 'Показать'}
                            </Button>
                          </div>
                          {stats[server.id]?.error && (
                            <div className="server-details__metrics-error">{stats[server.id]?.error}</div>
                          )}
                          {stats[server.id]?.data && (
                            <div className="server-details__metrics-grid">
                              <div className="metric">
                                <span className="metric__label">CPU</span>
                                <span className="metric__value">{stats[server.id]?.data?.cpu_perc}</span>
                              </div>
                              <div className="metric">
                                <span className="metric__label">RAM</span>
                                <span className="metric__value">
                                  {stats[server.id]?.data?.mem_usage} ({stats[server.id]?.data?.mem_perc})
                                </span>
                              </div>
                              <div className="metric">
                                <span className="metric__label">Сеть (RX/TX)</span>
                                <span className="metric__value">{stats[server.id]?.data?.net_io}</span>
                              </div>
                              <div className="metric">
                                <span className="metric__label">PIDs</span>
                                <span className="metric__value">{stats[server.id]?.data?.pids}</span>
                              </div>
                            </div>
                          )}
                        </div>

                        <div className="server-details__actions">
                          <Button variant="secondary" size="sm" disabled={busy} onClick={() => handleHealth(server)}>
                            Проверить
                          </Button>
                          <Button variant="secondary" size="sm" disabled={busy} onClick={() => onEdit(server)}>
                            Изменить
                          </Button>
                          <Button variant="secondary" size="sm" disabled={busy} onClick={() => setLogsServer(server)}>
                            Логи
                          </Button>
                          <Button variant="secondary" size="sm" disabled={busy} onClick={() => handleRecreate(server)}>
                            Пересоздать
                          </Button>
                          <Button variant="danger" size="sm" disabled={busy} onClick={() => handleDelete(server)}>
                            Удалить
                          </Button>
                        </div>
                      </div>
                    </td>
                  </tr>
                )}
              </Fragment>
            )
          })}
        </tbody>
      </table>

      {logsServer && (
        <ServerLogsModal server={logsServer} onClose={() => setLogsServer(null)} />
      )}
    </div>
  )
}
