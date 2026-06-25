import { FormEvent, useState } from 'react'
import { api, EditPayload, Server } from '../api/client'
import { formatTags, parseTags } from '../utils/tags'
import Button from './ui/Button'
import { useToast } from './ui/Toast'
import './AddServerModal.css'

type Props = {
  server: Server
  onClose: () => void
  onSaved: () => void
}

export default function EditServerModal({ server, onClose, onSaved }: Props) {
  const toast = useToast()
  const [name, setName] = useState(server.name)
  const [host, setHost] = useState(server.host)
  const [sshPort, setSSHPort] = useState(String(server.ssh_port))
  const [sshUser, setSSHUser] = useState(server.ssh_user)
  const [mtprotoPort, setMtprotoPort] = useState(String(server.mtproto_port))
  const [tags, setTags] = useState(formatTags(server.tags))

  const [changeAuth, setChangeAuth] = useState(false)
  const [authType, setAuthType] = useState<'password' | 'key'>(
    server.ssh_auth_type === 'key' ? 'key' : 'password',
  )
  const [password, setPassword] = useState('')
  const [privateKey, setPrivateKey] = useState('')
  const [passphrase, setPassphrase] = useState('')

  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const portChanged = Number(mtprotoPort) !== server.mtproto_port
  const hostChanged = host.trim() !== server.host

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)

    const payload: EditPayload = {
      name: name.trim() || undefined,
      host: host.trim() || undefined,
      ssh_port: Number(sshPort) || undefined,
      ssh_user: sshUser.trim() || undefined,
      mtproto_port: Number(mtprotoPort) || undefined,
      tags: parseTags(tags),
    }

    if (changeAuth) {
      payload.ssh_auth_type = authType
      if (authType === 'password') {
        payload.password = password
      } else {
        payload.private_key = privateKey
        payload.passphrase = passphrase || undefined
      }
    }

    try {
      const res = await api.editServer(server.id, payload)
      toast.success(res.redeploying ? 'Сохранено, идёт пересоздание' : 'Изменения сохранены')
      onSaved()
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Не удалось сохранить'
      setError(message)
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={handleSubmit}>
        <div className="modal__header">
          <h2>Редактировать сервер</h2>
          <button type="button" className="modal__close" onClick={onClose} aria-label="Закрыть">
            ×
          </button>
        </div>

        <label>
          Имя
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Мой VPS" />
        </label>

        <label>
          Теги (через запятую)
          <input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="fr, vpn, fast" />
        </label>

        <label>
          IP / Host
          <input value={host} onChange={(e) => setHost(e.target.value)} required />
        </label>

        <div className="row">
          <label>
            SSH порт
            <input value={sshPort} onChange={(e) => setSSHPort(e.target.value)} inputMode="numeric" />
          </label>
          <label>
            SSH пользователь
            <input value={sshUser} onChange={(e) => setSSHUser(e.target.value)} required />
          </label>
        </div>

        <label>
          Порт прокси
          <input value={mtprotoPort} onChange={(e) => setMtprotoPort(e.target.value)} inputMode="numeric" />
        </label>

        {(portChanged || hostChanged) && (
          <div className="form-hint">
            Смена {portChanged && hostChanged ? 'порта и хоста' : portChanged ? 'порта' : 'хоста'} вызовет
            пересоздание контейнера (новый деплой).
          </div>
        )}

        <label className="checkbox-row">
          <input type="checkbox" checked={changeAuth} onChange={(e) => setChangeAuth(e.target.checked)} />
          <span>Изменить SSH-доступ</span>
        </label>

        {changeAuth && (
          <>
            <label>
              Тип авторизации
              <select value={authType} onChange={(e) => setAuthType(e.target.value as 'password' | 'key')}>
                <option value="password">Пароль</option>
                <option value="key">SSH-ключ</option>
              </select>
            </label>

            {authType === 'password' ? (
              <label>
                Новый пароль
                <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
              </label>
            ) : (
              <>
                <label>
                  Приватный ключ
                  <textarea
                    value={privateKey}
                    onChange={(e) => setPrivateKey(e.target.value)}
                    rows={5}
                    placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                    required
                  />
                </label>
                <label>
                  Passphrase (опционально)
                  <input type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} />
                </label>
              </>
            )}
          </>
        )}

        {error && <div className="form-error">{error}</div>}

        <div className="modal-actions">
          <Button type="button" variant="ghost" onClick={onClose}>
            Отмена
          </Button>
          <Button type="submit" variant="primary" disabled={loading}>
            {loading ? 'Сохранение...' : 'Сохранить'}
          </Button>
        </div>
      </form>
    </div>
  )
}
