import { createClient, type SupabaseClient, type User } from '@supabase/supabase-js'
import type { Entry, BuildingMeta, Stats, ImportPayload, LocationItem } from '../types'

const supabaseUrl = import.meta.env.VITE_SUPABASE_URL || ''
const supabaseAnonKey = import.meta.env.VITE_SUPABASE_ANON_KEY || ''

export const isSupabaseConfigured = (): boolean => {
  return !!supabaseUrl && !!supabaseAnonKey && supabaseUrl.startsWith('https://')
}

export const supabase: SupabaseClient | null = isSupabaseConfigured()
  ? createClient(supabaseUrl, supabaseAnonKey)
  : null

export const supabaseService = {
  // Auth methods
  async getCurrentUser(): Promise<User | null> {
    if (!supabase) return null
    const { data: { session } } = await supabase.auth.getSession()
    return session?.user ?? null
  },

  async login(email: string, password: string) {
    if (!supabase) throw new Error('Supabase is not configured')
    const { data, error } = await supabase.auth.signInWithPassword({ email, password })
    if (error) throw error
    return data.user
  },

  async logout() {
    if (!supabase) return
    const { error } = await supabase.auth.signOut()
    if (error) throw error
  },

  onAuthStateChange(callback: (user: User | null) => void) {
    if (!supabase) return { unsubscribe: () => {} }
    const { data: { subscription } } = supabase.auth.onAuthStateChange((_event, session) => {
      callback(session?.user ?? null)
    })
    return { unsubscribe: () => subscription.unsubscribe() }
  },

  // Data methods
  async list(q = '', building = '', floor = ''): Promise<Entry[]> {
    if (!supabase) throw new Error('Supabase is not configured')
    let query = supabase
      .from('entries')
      .select('id, building, floor, department, internal_phone, external_phone, sort_order')
      .order('sort_order', { ascending: true })
      .order('id', { ascending: true })

    if (building && building !== 'ทั้งหมด' && building.toLowerCase() !== 'all') {
      query = query.eq('building', building)
    }

    if (floor && floor !== 'ทั้งหมด' && floor.toLowerCase() !== 'all') {
      query = query.eq('floor', floor)
    }

    if (q.trim()) {
      const term = `%${q.trim()}%`
      query = query.or(`building.ilike.${term},floor.ilike.${term},department.ilike.${term},internal_phone.ilike.${term},external_phone.ilike.${term}`)
    }

    const { data, error } = await query
    if (error) throw error
    return (data || []) as Entry[]
  },

  async meta(): Promise<BuildingMeta[]> {
    if (!supabase) throw new Error('Supabase is not configured')
    const { data, error } = await supabase
      .from('entries')
      .select('building')

    if (error) throw error
    const counts: Record<string, number> = {}
    for (const row of data || []) {
      if (row.building) {
        counts[row.building] = (counts[row.building] || 0) + 1
      }
    }

    return Object.entries(counts).map(([building, count]) => ({
      building,
      count,
    }))
  },

  async stats(): Promise<Stats> {
    if (!supabase) throw new Error('Supabase is not configured')
    const { data, error } = await supabase
      .from('entries')
      .select('building, floor')

    if (error) throw error
    const total_entries = data?.length || 0
    const buildings = new Set<string>()
    const floors = new Set<string>()

    for (const row of data || []) {
      if (row.building) buildings.add(row.building)
      if (row.floor) floors.add(row.floor)
    }

    return {
      total_entries,
      total_buildings: buildings.size,
      total_floors: floors.size,
    }
  },

  async create(e: Omit<Entry, 'id'>): Promise<Entry> {
    if (!supabase) throw new Error('Supabase is not configured')
    const { data, error } = await supabase
      .from('entries')
      .insert([{
        building: e.building.trim(),
        floor: e.floor.trim(),
        department: e.department.trim(),
        internal_phone: (e.internal_phone || '').trim(),
        external_phone: (e.external_phone || '').trim(),
        sort_order: e.sort_order || 0,
      }])
      .select()
      .single()

    if (error) throw error
    return data as Entry
  },

  async update(e: Entry): Promise<Entry> {
    if (!supabase) throw new Error('Supabase is not configured')
    const { data, error } = await supabase
      .from('entries')
      .update({
        building: e.building.trim(),
        floor: e.floor.trim(),
        department: e.department.trim(),
        internal_phone: (e.internal_phone || '').trim(),
        external_phone: (e.external_phone || '').trim(),
        sort_order: e.sort_order || 0,
        updated_at: new Date().toISOString(),
      })
      .eq('id', e.id)
      .select()
      .single()

    if (error) throw error
    return data as Entry
  },

  async remove(id: number): Promise<void> {
    if (!supabase) throw new Error('Supabase is not configured')
    const { error } = await supabase
      .from('entries')
      .delete()
      .eq('id', id)

    if (error) throw error
  },

  async relocate(ids: number[], targetBuilding: string, targetFloor: string): Promise<{ success: boolean; count: number }> {
    if (!supabase) throw new Error('Supabase is not configured')
    const validIds = ids.filter((id) => id > 0)
    if (validIds.length === 0) return { success: true, count: 0 }

    const { error } = await supabase
      .from('entries')
      .update({
        building: targetBuilding.trim(),
        floor: targetFloor.trim(),
        updated_at: new Date().toISOString(),
      })
      .in('id', validIds)

    if (error) throw error
    return { success: true, count: validIds.length }
  },

  async importBatch(payload: ImportPayload): Promise<{ success: boolean; imported: number }> {
    if (!supabase) throw new Error('Supabase is not configured')
    if (payload.mode === 'replace') {
      // Delete all existing
      const { error: delErr } = await supabase.from('entries').delete().neq('id', 0)
      if (delErr) throw delErr
    }

    const rows = payload.items.map((item, idx) => ({
      building: item.building.trim(),
      floor: item.floor.trim(),
      department: item.department.trim(),
      internal_phone: (item.internal_phone || '').trim(),
      external_phone: (item.external_phone || '').trim(),
      sort_order: item.sort_order || (idx + 1),
    }))

    // Insert in batches of 200
    const chunkSize = 200
    let imported = 0
    for (let i = 0; i < rows.length; i += chunkSize) {
      const chunk = rows.slice(i, i + chunkSize)
      const { error } = await supabase.from('entries').insert(chunk)
      if (error) throw error
      imported += chunk.length
    }

    return { success: true, imported }
  },

  async backupJSON(): Promise<Entry[]> {
    if (!supabase) throw new Error('Supabase is not configured')
    const { data, error } = await supabase
      .from('entries')
      .select('id, building, floor, department, internal_phone, external_phone, sort_order')
      .order('sort_order', { ascending: true })

    if (error) throw error
    return (data || []) as Entry[]
  },

  // Location management methods
  async getLocations(): Promise<LocationItem[]> {
    if (!supabase) throw new Error('Supabase is not configured')
    const { data, error } = await supabase
      .from('locations')
      .select('id, building, floor, sort_order')
      .order('building', { ascending: true })
      .order('sort_order', { ascending: true })
      .order('id', { ascending: true })

    if (error) {
      console.warn('Could not load locations from supabase table:', error)
      return []
    }
    return (data || []) as LocationItem[]
  },

  async addLocation(building: string, floor: string): Promise<LocationItem> {
    if (!supabase) throw new Error('Supabase is not configured')
    const b = building.trim()
    const f = floor.trim()
    if (!b || !f) throw new Error('กรุณาระบุอาคารและชั้น')

    const { data, error } = await supabase
      .from('locations')
      .upsert({ building: b, floor: f }, { onConflict: 'building,floor' })
      .select()
      .single()

    if (error) throw error
    return data as LocationItem
  },

  async deleteLocation(building: string, floor: string): Promise<void> {
    if (!supabase) throw new Error('Supabase is not configured')
    const { error } = await supabase
      .from('locations')
      .delete()
      .eq('building', building.trim())
      .eq('floor', floor.trim())

    if (error) throw error
  },
}
