import { useEffect, useState } from 'react'
import { api } from './api/client'
import { ConfirmProvider } from './components/ui/ConfirmDialog'
import { ToastProvider } from './components/ui/Toast'
import ChangePasswordPage from './pages/ChangePasswordPage'
import LoginPage from './pages/LoginPage'
import ServersPage from './pages/ServersPage'

type Session = {
  username: string
  mustChangePassword: boolean
}

export default function App() {
  const [session, setSession] = useState<Session | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.me()
      .then((res) =>
        setSession({
          username: res.username,
          mustChangePassword: res.must_change_password,
        }),
      )
      .catch(() => setSession(null))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return <div className="center">Загрузка...</div>
  }

  return (
    <ToastProvider>
      <ConfirmProvider>
        {!session ? (
          <LoginPage
            onSuccess={(res) => {
              setSession({
                username: res.username,
                mustChangePassword: res.must_change_password,
              })
            }}
          />
        ) : session.mustChangePassword ? (
          <ChangePasswordPage
            username={session.username}
            onSuccess={() => {
              setSession((prev) => (prev ? { ...prev, mustChangePassword: false } : prev))
            }}
          />
        ) : (
          <ServersPage
            username={session.username}
            onLogout={async () => {
              await api.logout()
              setSession(null)
            }}
          />
        )}
      </ConfirmProvider>
    </ToastProvider>
  )
}
