import { FormEvent, useState } from 'react'
import { api } from '../api/client'
import Button from '../components/ui/Button'
import './LoginPage.css'

type Props = {
  onSuccess: (session: { username: string; must_change_password: boolean }) => void
}

export default function LoginPage({ onSuccess }: Props) {
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await api.login(username, password)
      onSuccess(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка входа')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-page">
      <form className="login-card" onSubmit={handleSubmit}>
        <div className="login-card__brand">
          <div className="login-card__logo">M</div>
          <div>
            <h1>MTProto UI</h1>
            <p className="muted">Консоль управления MTProto-прокси</p>
          </div>
        </div>

        <label>
          Логин
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
        </label>

        <label>
          Пароль
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
          />
        </label>

        {error && <div className="form-error">{error}</div>}

        <Button type="submit" variant="primary" fullWidth disabled={loading}>
          {loading ? 'Вход...' : 'Войти'}
        </Button>
      </form>
    </div>
  )
}
