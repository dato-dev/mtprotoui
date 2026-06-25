import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, Server } from '../api/client'
import AddServerModal from '../components/AddServerModal'
import EditServerModal from '../components/EditServerModal'
import ServersTable, { SortDir, SortKey } from '../components/ServersTable'
import Button from '../components/ui/Button'
import { useConfirm } from '../components/ui/ConfirmDialog'
import { useToast } from '../components/ui/Toast'
import './ServersPage.css'

type Props = {
  username: string
  onLogout: () => void
}

const STATUS_RANK: Record<string, number> = { online: 0, degraded: 1, offline: 2, unknown: 3 }
const DEPLOY_RANK: Record<string, number> = {
  ready: 0,
  deploying: 1,
  deleting: 2,
  failed: 3,
  pending: 4,
}

export default function ServersPage({ username, onLogout }: Props) {
  const toast = useToast()
  const confirm = useConfirm()
  const [servers, setServers] = useState<Server[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showAdd, setShowAdd] = useState(false)
  const [editServer, setEditServer] = useState<Server | null>(null)

  const [search, setSearch] = useState('')
  const [tagFilter, setTagFilter] = useState<string | null>(null)
  const [sortKey, setSortKey] = useState<SortKey>('created')
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [bulkBusy, setBulkBusy] = useState(false)

  const loadServers = useCallback(async () => {
    try {
      const data = await api.listServers()
      setServers(data)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось загрузить серверы')
    } finally {
      setLoading(false)
    }
  }, [])

  const hasActiveOperations = servers.some(
    (s) => s.deploy_status === 'deploying' || s.deploy_status === 'deleting',
  )

  useEffect(() => {
    loadServers()
    const interval = hasActiveOperations ? 2000 : 5000
    const timer = setInterval(loadServers, interval)
    return () => clearInterval(timer)
  }, [loadServers, hasActiveOperations])

  const allTags = useMemo(() => {
    const set = new Set<string>()
    servers.forEach((s) => s.tags?.forEach((t) => set.add(t)))
    return Array.from(set).sort((a, b) => a.localeCompare(b))
  }, [servers])

  const displayed = useMemo(() => {
    const q = search.trim().toLowerCase()
    let list = servers.filter((s) => {
      if (tagFilter && !(s.tags ?? []).includes(tagFilter)) return false
      if (!q) return true
      const haystack = [s.name, s.host, ...(s.tags ?? [])].join(' ').toLowerCase()
      return haystack.includes(q)
    })

    list = [...list].sort((a, b) => {
      let cmp = 0
      switch (sortKey) {
        case 'name':
          cmp = a.name.localeCompare(b.name)
          break
        case 'host':
          cmp = a.host.localeCompare(b.host)
          break
        case 'status':
          cmp = (STATUS_RANK[a.status] ?? 9) - (STATUS_RANK[b.status] ?? 9)
          break
        case 'deploy':
          cmp = (DEPLOY_RANK[a.deploy_status] ?? 9) - (DEPLOY_RANK[b.deploy_status] ?? 9)
          break
        case 'created':
          cmp = new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
          break
      }
      return sortDir === 'asc' ? cmp : -cmp
    })
    return list
  }, [servers, search, tagFilter, sortKey, sortDir])

  function handleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir(key === 'created' ? 'desc' : 'asc')
    }
  }

  function toggleSelect(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const displayedIds = displayed.map((s) => s.id)
  const allSelected = displayedIds.length > 0 && displayedIds.every((id) => selected.has(id))

  function toggleSelectAll() {
    setSelected((prev) => {
      if (displayedIds.every((id) => prev.has(id))) {
        const next = new Set(prev)
        displayedIds.forEach((id) => next.delete(id))
        return next
      }
      return new Set([...prev, ...displayedIds])
    })
  }

  function clearSelection() {
    setSelected(new Set())
  }

  async function bulkHealth() {
    const ids = displayedIds.filter((id) => selected.has(id))
    if (ids.length === 0) return
    setBulkBusy(true)
    try {
      await Promise.allSettled(ids.map((id) => api.checkHealth(id)))
      toast.info(`Проверка запущена: ${ids.length}`)
      await loadServers()
    } finally {
      setBulkBusy(false)
    }
  }

  async function bulkDelete() {
    const ids = displayedIds.filter((id) => selected.has(id))
    if (ids.length === 0) return
    const ok = await confirm({
      title: 'Удалить выбранные серверы?',
      message: `Будут удалены ${ids.length} сервер(ов) вместе с контейнерами и образами на VPS.`,
      confirmLabel: 'Удалить',
      variant: 'danger',
    })
    if (!ok) return

    setBulkBusy(true)
    try {
      const results = await Promise.allSettled(ids.map((id) => api.deleteServer(id)))
      const failed = results.filter((r) => r.status === 'rejected').length
      if (failed > 0) toast.error(`Не удалось запустить удаление: ${failed}`)
      else toast.info(`Удаление запущено: ${ids.length}`)
      clearSelection()
      await loadServers()
    } finally {
      setBulkBusy(false)
    }
  }

  async function handleLogout() {
    await onLogout()
    toast.info('Вы вышли из системы')
  }

  const selectedCount = displayedIds.filter((id) => selected.has(id)).length

  return (
    <div className="servers-page">
      <header className="topbar">
        <div>
          <h1>MTProto серверы</h1>
          <p className="muted">Пользователь: {username}</p>
        </div>
        <div className="topbar-actions">
          <Button variant="primary" onClick={() => setShowAdd(true)}>
            + Добавить сервер
          </Button>
          <Button variant="ghost" onClick={handleLogout}>
            Выйти
          </Button>
        </div>
      </header>

      {error && <div className="banner error">{error}</div>}
      {loading && servers.length === 0 && <div className="banner">Загрузка...</div>}

      {servers.length > 0 && (
        <div className="toolbar">
          <input
            className="toolbar__search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Поиск по имени, адресу, тегу..."
          />
          {allTags.length > 0 && (
            <div className="toolbar__tags">
              <button
                type="button"
                className={`tag-chip ${tagFilter === null ? 'is-active' : ''}`}
                onClick={() => setTagFilter(null)}
              >
                Все
              </button>
              {allTags.map((tag) => (
                <button
                  key={tag}
                  type="button"
                  className={`tag-chip ${tagFilter === tag ? 'is-active' : ''}`}
                  onClick={() => setTagFilter((cur) => (cur === tag ? null : tag))}
                >
                  {tag}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {selectedCount > 0 && (
        <div className="bulkbar">
          <span className="bulkbar__count">Выбрано: {selectedCount}</span>
          <Button variant="secondary" size="sm" disabled={bulkBusy} onClick={bulkHealth}>
            Проверить
          </Button>
          <Button variant="danger" size="sm" disabled={bulkBusy} onClick={bulkDelete}>
            Удалить
          </Button>
          <Button variant="ghost" size="sm" disabled={bulkBusy} onClick={clearSelection}>
            Снять выделение
          </Button>
        </div>
      )}

      {servers.length > 0 && (
        <ServersTable
          servers={displayed}
          onChanged={loadServers}
          selected={selected}
          onToggleSelect={toggleSelect}
          onToggleSelectAll={toggleSelectAll}
          allSelected={allSelected}
          sortKey={sortKey}
          sortDir={sortDir}
          onSort={handleSort}
          onEdit={setEditServer}
        />
      )}

      {servers.length > 0 && displayed.length === 0 && (
        <div className="banner">Ничего не найдено по заданным фильтрам</div>
      )}

      {!loading && servers.length === 0 && (
        <div className="empty-state">
          <p>Серверов пока нет</p>
          <span>Добавьте первый MTProto-сервер для начала работы</span>
          <Button variant="primary" onClick={() => setShowAdd(true)}>
            Добавить сервер
          </Button>
        </div>
      )}

      {showAdd && (
        <AddServerModal
          onClose={() => setShowAdd(false)}
          onCreated={() => {
            setShowAdd(false)
            loadServers()
          }}
        />
      )}

      {editServer && (
        <EditServerModal
          server={editServer}
          onClose={() => setEditServer(null)}
          onSaved={() => {
            setEditServer(null)
            loadServers()
          }}
        />
      )}
    </div>
  )
}
