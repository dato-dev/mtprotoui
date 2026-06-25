export type PasswordRule = {
  id: string
  label: string
  test: (password: string, username: string) => boolean
}

export const passwordRules: PasswordRule[] = [
  {
    id: 'length',
    label: 'Минимум 12 символов',
    test: (p) => p.length >= 12,
  },
  {
    id: 'upper',
    label: 'Заглавная буква',
    test: (p) => /[A-ZА-Я]/.test(p),
  },
  {
    id: 'lower',
    label: 'Строчная буква',
    test: (p) => /[a-zа-я]/.test(p),
  },
  {
    id: 'digit',
    label: 'Цифра',
    test: (p) => /\d/.test(p),
  },
  {
    id: 'special',
    label: 'Спецсимвол (!@#$%...)',
    test: (p) => /[!@#$%^&*()_+\-=[\]{}|;:,.<>?]/.test(p),
  },
  {
    id: 'username',
    label: 'Не совпадает с логином',
    test: (p, username) => p.toLowerCase() !== username.toLowerCase(),
  },
]

export function validatePassword(password: string, username: string): string | null {
  for (const rule of passwordRules) {
    if (!rule.test(password, username)) {
      return rule.label
    }
  }
  return null
}

export function getPasswordStrength(password: string, username: string): number {
  if (!password) return 0
  const passed = passwordRules.filter((r) => r.test(password, username)).length
  return Math.round((passed / passwordRules.length) * 100)
}
