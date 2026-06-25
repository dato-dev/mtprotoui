import { FormEvent, useState } from 'react'
import { api } from '../api/client'
import { parseTags } from '../utils/tags'
import Button from './ui/Button'
import { useToast } from './ui/Toast'
import './AddServerModal.css'

type Props = {
  onClose: () => void
  onCreated: () => void
}

export default function AddServerModal({ onClose, onCreated }: Props) {
  const toast = useToast()
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [proxyType, setProxyType] = useState<'mtg' | 'tg-ws-proxy'>('mtg')
  const [mtprotoPort, setMtprotoPort] = useState('443')
  const [portTouched, setPortTouched] = useState(false)
  const [fakeTLS, setFakeTLS] = useState(false)
  const [tags, setTags] = useState('')
  const [sshPort, setSSHPort] = useState('22')
  const [sshUser, setSSHUser] = useState('root')
  const [authType, setAuthType] = useState<'password' | 'key'>('password')
  const [password, setPassword] = useState('')
  const [privateKey, setPrivateKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  function defaultPort(type: 'mtg' | 'tg-ws-proxy') {
    return type === 'tg-ws-proxy' ? '1443' : '443'
  }

  function handleProxyTypeChange(type: 'mtg' | 'tg-ws-proxy') {
    setProxyType(type)
    if (type === 'mtg') {
      setFakeTLS(false)
    }
    if (!portTouched) {
      setMtprotoPort(defaultPort(type))
    }
  }

  function payload() {
    return {
      name: name || undefined,
      host,
      ssh_port: Number(sshPort) || 22,
      ssh_user: sshUser,
      ssh_auth_type: authType,
      password: authType === 'password' ? password : undefined,
      private_key: authType === 'key' ? privateKey : undefined,
      passphrase: authType === 'key' ? passphrase : undefined,
      proxy_type: proxyType,
      mtproto_port: Number(mtprotoPort) || defaultPortNumber(proxyType),
      fake_tls: proxyType === 'tg-ws-proxy' ? fakeTLS : undefined,
      tags: parseTags(tags),
    }
  }

  function defaultPortNumber(type: 'mtg' | 'tg-ws-proxy') {
    return type === 'tg-ws-proxy' ? 1443 : 443
  }

  async function handleTest() {
    setError('')
    setLoading(true)
    try {
      await api.testSSH(payload())
      toast.success('SSH-соединение успешно')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'SSH test failed'
      setError(message)
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await api.createServer(payload())
      toast.success('Сервер добавлен, идёт развёртывание')
      onCreated()
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Не удалось добавить сервер'
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
          <h2>Добавить сервер</h2>
          <button type="button" className="modal__close" onClick={onClose} aria-label="Закрыть">
            ×
          </button>
        </div>

        <label>
          Имя (опционально)
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Мой VPS" />
        </label>

        <label>
          Теги (через запятую)
          <input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="fr, vpn, fast" />
        </label>

        <label>
          IP / Host
          <input value={host} onChange={(e) => setHost(e.target.value)} required placeholder="185.236.22.5" />
        </label>

        <div className="row">
          <label>
            Тип прокси
            <select
              value={proxyType}
              onChange={(e) => handleProxyTypeChange(e.target.value as 'mtg' | 'tg-ws-proxy')}
            >
              <option value="mtg">MTG (mtg v2)</option>
              <option value="tg-ws-proxy">TG WS Proxy</option>
            </select>
          </label>
          <label>
            <span className="label-with-hint">
              Порт прокси
              <span
                className="hint"
                tabIndex={0}
                data-tip={
                  'Порт на VPS, который слушает прокси.\n' +
                  '443 — рекомендуется: похож на HTTPS, проходит большинство фаерволов.\n' +
                  'По умолчанию: MTG → 443, TG WS Proxy → 1443.\n' +
                  'Внутри контейнера TG WS Proxy всегда слушает 1443, наружу пробрасывается выбранный порт.'
                }
                aria-label="Подсказка о порте"
              >
                i
              </span>
            </span>
            <input
              value={mtprotoPort}
              inputMode="numeric"
              onChange={(e) => {
                setPortTouched(true)
                setMtprotoPort(e.target.value)
              }}
              placeholder={defaultPort(proxyType)}
            />
          </label>
        </div>

        {proxyType === 'tg-ws-proxy' && (
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={fakeTLS}
              onChange={(e) => setFakeTLS(e.target.checked)}
            />
            <span>
              <span className="label-with-hint">
                Fake TLS (маскировка под HTTPS)
                <span
                  className="hint"
                  tabIndex={0}
                  data-tip={
                    'Включает ee-секрет с SNI-доменом из whitelist — трафик маскируется под TLS к реальному сайту.\n' +
                    'Без него используется обычный dd-секрет.'
                  }
                  aria-label="Подсказка о Fake TLS"
                >
                  i
                </span>
              </span>
            </span>
          </label>
        )}

        <div className="row">
          <label>
            SSH порт
            <input value={sshPort} onChange={(e) => setSSHPort(e.target.value)} />
          </label>
          <label>
            SSH пользователь
            <input value={sshUser} onChange={(e) => setSSHUser(e.target.value)} required />
          </label>
        </div>

        <label>
          Тип авторизации
          <select value={authType} onChange={(e) => setAuthType(e.target.value as 'password' | 'key')}>
            <option value="password">Пароль</option>
            <option value="key">SSH-ключ</option>
          </select>
        </label>

        {authType === 'password' ? (
          <label>
            Пароль
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

        {error && <div className="form-error">{error}</div>}

        <div className="modal-actions">
          <Button type="button" variant="ghost" onClick={onClose}>
            Отмена
          </Button>
          <Button type="button" variant="secondary" disabled={loading} onClick={handleTest}>
            Проверить SSH
          </Button>
          <Button type="submit" variant="primary" disabled={loading}>
            {loading ? 'Добавление...' : 'Добавить и развернуть'}
          </Button>
        </div>
      </form>
    </div>
  )
}
