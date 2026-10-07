import type { Entry, BuildingMeta, Stats, ImportPayload, LocationItem } from '../types'
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

  getLocations: async (): Promise<LocationItem[]> => {
    if (isSupabaseConfigured()) {
      try {
        const supaLocs = await supabaseService.getLocations()
        if (supaLocs && supaLocs.length > 0) return supaLocs
      } catch (e) {
        console.warn('Supabase getLocations error:', e)
      }
    }
    try {
      const raw = localStorage.getItem('bgh_custom_locations')
      if (raw) return JSON.parse(raw)
    } catch {}
    return []
  },

  addLocation: async (building: string, floor: string): Promise<LocationItem> => {
    let item: LocationItem = { building: building.trim(), floor: floor.trim() }
    if (isSupabaseConfigured()) {
      try {
        item = await supabaseService.addLocation(building, floor)
      } catch (e) {
        console.warn('Supabase addLocation error, fallback local:', e)
      }
    }
    try {
      const raw = localStorage.getItem('bgh_custom_locations')
      const list: LocationItem[] = raw ? JSON.parse(raw) : []
      if (!list.some((x) => x.building === item.building && x.floor === item.floor)) {
        list.push(item)
        localStorage.setItem('bgh_custom_locations', JSON.stringify(list))
      }
    } catch {}
    return item
  },

  deleteLocation: async (building: string, floor: string): Promise<void> => {
    if (isSupabaseConfigured()) {
      try {
        await supabaseService.deleteLocation(building, floor)
      } catch (e) {
        console.warn('Supabase deleteLocation error:', e)
      }
    }
    try {
      const raw = localStorage.getItem('bgh_custom_locations')
      if (raw) {
        const list: LocationItem[] = JSON.parse(raw)
        const filtered = list.filter((x) => !(x.building === building.trim() && x.floor === floor.trim()))
        localStorage.setItem('bgh_custom_locations', JSON.stringify(filtered))
      }
    } catch {}
  },

  renameBuilding: async (oldName: string, newName: string): Promise<{ updatedLocations: number; updatedEntries: number }> => {
    const from = oldName.trim()
    const to = newName.trim()
    let result = { updatedLocations: 0, updatedEntries: 0 }
    if (isSupabaseConfigured()) {
      result = await supabaseService.renameBuilding(from, to)
    } else {
      try {
        result = await req<{ updatedLocations: number; updatedEntries: number }>('/api/buildings/rename', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ old_building: from, new_building: to }),
        })
      } catch (e) {
        console.warn('Local renameBuilding error:', e)
      }
    }
    // Update local cache if any
    try {
      const raw = localStorage.getItem('bgh_custom_locations')
      if (raw) {
        const list: LocationItem[] = JSON.parse(raw)
        let changed = false
        for (const item of list) {
          if (item.building === from) {
            item.building = to
            changed = true
          }
        }
        if (changed) {
          localStorage.setItem('bgh_custom_locations', JSON.stringify(list))
        }
      }
    } catch {}
    return result
  },

  deleteBuilding: async (building: string): Promise<void> => {
    const b = building.trim()
    if (isSupabaseConfigured()) {
      await supabaseService.deleteBuilding(b)
    } else {
      try {
        await req('/api/buildings/delete', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ building: b }),
        })
      } catch (e) {
        console.warn('Local deleteBuilding error:', e)
      }
    }
    try {
      const raw = localStorage.getItem('bgh_custom_locations')
      if (raw) {
        const list: LocationItem[] = JSON.parse(raw)
        const filtered = list.filter((x) => x.building !== b)
        localStorage.setItem('bgh_custom_locations', JSON.stringify(filtered))
      }
    } catch {}
  },
}

