import { createContext, ReactNode, useCallback, useContext, useRef, useState } from 'react'
import Button from './Button'
import './ConfirmDialog.css'

export type ConfirmOptions = {
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  variant?: 'danger' | 'primary'
}

type ConfirmContextValue = {
  confirm: (options: ConfirmOptions) => Promise<boolean>
}

const ConfirmContext = createContext<ConfirmContextValue | null>(null)

type DialogState = ConfirmOptions & { open: boolean }

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [dialog, setDialog] = useState<DialogState | null>(null)
  const resolver = useRef<((value: boolean) => void) | null>(null)

  const confirm = useCallback((options: ConfirmOptions) => {
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve
      setDialog({ ...options, open: true })
    })
  }, [])

  const close = (result: boolean) => {
    setDialog(null)
    resolver.current?.(result)
    resolver.current = null
  }

  return (
    <ConfirmContext.Provider value={{ confirm }}>
      {children}
      {dialog?.open && (
        <div className="confirm-backdrop" onClick={() => close(false)}>
          <div
            className="confirm-dialog"
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="confirm-title"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 id="confirm-title">{dialog.title}</h2>
            <p>{dialog.message}</p>
            <div className="confirm-dialog__actions">
              <Button variant="ghost" onClick={() => close(false)}>
                {dialog.cancelLabel ?? 'Отмена'}
              </Button>
              <Button
                variant={dialog.variant === 'danger' ? 'danger' : 'primary'}
                onClick={() => close(true)}
              >
                {dialog.confirmLabel ?? 'Подтвердить'}
              </Button>
            </div>
          </div>
        </div>
      )}
    </ConfirmContext.Provider>
  )
}

export function useConfirm() {
  const ctx = useContext(ConfirmContext)
  if (!ctx) {
    throw new Error('useConfirm must be used within ConfirmProvider')
  }
  return ctx.confirm
}
