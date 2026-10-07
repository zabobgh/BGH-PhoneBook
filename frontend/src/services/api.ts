import type { Entry, BuildingMeta, Stats, ImportPayload } from '../types'
import * as wailsApp from '../../wailsjs/go/main/App'
import { isSupabaseConfigured, supabaseService } from './supabase'

const hasWails = () => typeof window !== 'undefined' && !!(window as any).go?.main?.App

async function req<T>(url: string, options?: RequestInit): Promise<T> {
  const r = await fetch(url, options)
  if (!r.ok) {
    let msg = `HTTP ${r.status}`
    try {
      const j = await r.json()
      msg = j.error || msg
    } catch {}
    throw new Error(msg)
  }
  if (r.status === 204) return undefined as T
  return r.json()
}

export const api = {
  isWails: hasWails,
  isSupabase: isSupabaseConfigured,

  list: async (q = '', building = '', floor = ''): Promise<Entry[]> => {
    if (hasWails()) {
      return (await wailsApp.ListEntries(q, building, floor)) as Entry[]
    }
    if (isSupabaseConfigured()) {
      return supabaseService.list(q, building, floor)
    }
    const params = new URLSearchParams()
    if (q) params.set('q', q)
    if (building) params.set('building', building)
    if (floor) params.set('floor', floor)
    const qs = params.toString()
    return req<Entry[]>(`/api/entries${qs ? '?' + qs : ''}`)
  },

  meta: async (): Promise<BuildingMeta[]> => {
    if (hasWails()) {
      return (await wailsApp.GetMeta()) as BuildingMeta[]
    }
    if (isSupabaseConfigured()) {
      return supabaseService.meta()
    }
    return req<BuildingMeta[]>('/api/meta')
  },

  stats: async (): Promise<Stats> => {
    if (hasWails()) {
      return (await wailsApp.GetStats()) as Stats
    }
    if (isSupabaseConfigured()) {
      return supabaseService.stats()
    }
    return req<Stats>('/api/stats')
  },

  create: async (e: Omit<Entry, 'id'>): Promise<Entry> => {
    if (hasWails()) {
      return (await wailsApp.CreateEntry({ ...e, id: 0 } as any)) as Entry
    }
    if (isSupabaseConfigured()) {
      return supabaseService.create(e)
    }
    return req<Entry>('/api/entries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(e),
    })
  },

  update: async (e: Entry): Promise<Entry> => {
    if (hasWails()) {
      return (await wailsApp.UpdateEntry(e as any)) as Entry
    }
    if (isSupabaseConfigured()) {
      return supabaseService.update(e)
    }
    return req<Entry>(`/api/entries/${e.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(e),
    })
  },

  remove: async (id: number): Promise<void> => {
    if (hasWails()) {
      await wailsApp.DeleteEntry(id)
      return
    }
    if (isSupabaseConfigured()) {
      return supabaseService.remove(id)
    }
    return req<void>(`/api/entries/${id}`, {
      method: 'DELETE',
    })
  },

  importBatch: async (payload: ImportPayload): Promise<{ success: boolean; imported: number }> => {
    if (hasWails()) {
      const count = await wailsApp.ImportBatch(payload.mode, payload.items as any)
      return { success: true, imported: count }
    }
    if (isSupabaseConfigured()) {
      return supabaseService.importBatch(payload)
    }
    return req<{ success: boolean; imported: number }>('/api/import', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
  },

  relocate: async (ids: number[], targetBuilding: string, targetFloor: string): Promise<{ success: boolean; count: number }> => {
    const app = wailsApp as Record<string, any>
    if (hasWails() && typeof app.RelocateEntries === 'function') {
      const count = await app.RelocateEntries(ids, targetBuilding, targetFloor)
      return { success: true, count }
    }
    if (isSupabaseConfigured()) {
      return supabaseService.relocate(ids, targetBuilding, targetFloor)
    }
    return req<{ success: boolean; count: number }>('/api/entries/relocate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids, target_building: targetBuilding, target_floor: targetFloor }),
    })
  },

  backupUrl: '/api/backup',
  backupDownload: async () => {
    if (hasWails()) {
      const data = await wailsApp.BackupJSON()
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'bgh-phonebook-backup.json'
      a.click()
      URL.revokeObjectURL(url)
      return
    }
    if (isSupabaseConfigured()) {
      const data = await supabaseService.backupJSON()
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'bgh-phonebook-backup.json'
      a.click()
      URL.revokeObjectURL(url)
      return
    }
    window.location.href = '/api/backup'
  },
}
