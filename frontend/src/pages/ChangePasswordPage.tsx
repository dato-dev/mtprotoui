import { FormEvent, useMemo, useState } from 'react'
import { api } from '../api/client'
import Button from '../components/ui/Button'
import { useToast } from '../components/ui/Toast'
import { getPasswordStrength, passwordRules, validatePassword } from '../utils/password'
import './ChangePasswordPage.css'

type Props = {
  username: string
  onSuccess: () => void
}

export default function ChangePasswordPage({ username, onSuccess }: Props) {
  const toast = useToast()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const strength = useMemo(() => getPasswordStrength(newPassword, username), [newPassword, username])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')

    if (newPassword !== confirmPassword) {
      setError('Пароли не совпадают')
      return
    }
    if (currentPassword === newPassword) {
      setError('Новый пароль должен отличаться от текущего')
      return
    }

    const validationError = validatePassword(newPassword, username)
    if (validationError) {
      setError(`Требование: ${validationError}`)
      return
    }

    setLoading(true)
    try {
      await api.changePassword(currentPassword, newPassword)
      toast.success('Пароль успешно изменён')
      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось сменить пароль')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="change-password-page">
      <form className="change-password-card" onSubmit={handleSubmit}>
        <div className="change-password-card__header">
          <h1>Смена пароля</h1>
          <p className="muted">
            Для безопасности задайте новый надёжный пароль перед началом работы с консолью.
          </p>
        </div>

        <label>
          Текущий пароль
          <input
            type="password"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>

        <label>
          Новый пароль
          <input
            type="password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            autoComplete="new-password"
            required
          />
        </label>

        <div className="password-strength">
          <div className="password-strength__bar">
            <div className="password-strength__fill" style={{ width: `${strength}%` }} />
          </div>
          <span className="muted">Надёжность: {strength}%</span>
        </div>

        <ul className="password-rules">
          {passwordRules.map((rule) => {
            const ok = rule.test(newPassword, username)
            return (
              <li key={rule.id} className={ok ? 'is-ok' : ''}>
                {ok ? '✓' : '○'} {rule.label}
              </li>
            )
          })}
        </ul>

        <label>
          Подтверждение пароля
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            autoComplete="new-password"
            required
          />
        </label>

        {error && <div className="form-error">{error}</div>}

        <Button type="submit" variant="primary" fullWidth disabled={loading}>
          {loading ? 'Сохранение...' : 'Сохранить пароль'}
        </Button>
      </form>
    </div>
  )
}
