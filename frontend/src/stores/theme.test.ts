import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useThemeStore } from '@/stores/theme'
import { themes } from '@/themes'

vi.stubGlobal('localStorage', {
  getItem: vi.fn(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
})

describe('Theme Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('should initialize with midnight theme by default', () => {
    const store = useThemeStore()
    expect(store.currentTheme).toBe('midnight')
  })

  it('should apply theme correctly', () => {
    const store = useThemeStore()
    store.applyTheme('dracula')
    expect(store.currentTheme).toBe('dracula')
    expect(localStorage.setItem).toHaveBeenCalledWith('theme', 'dracula')
  })

  it('should have access to all themes', () => {
    const store = useThemeStore()
    expect(Object.keys(store.themes).length).toBe(11)
  })
})
