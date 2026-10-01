import request from '@/utils/request'
import type { UserGarden } from '@/types/api'

export function listGardens() {
  return request.get<never, UserGarden[]>('/gardens')
}

export interface GardenPayload {
  plant_species_id: number
  nickname?: string
  owned_since?: string
  location?: string
}

export function addGarden(payload: GardenPayload) {
  return request.post<never, UserGarden>('/gardens', payload)
}

export function updateGarden(id: number, payload: { nickname?: string; location?: string; owned_since?: string }) {
  return request.put<never, UserGarden>(`/gardens/${id}`, payload)
}

export function repotGarden(id: number, payload: { owned_since: string; location?: string; nickname?: string }) {
  return request.post<never, UserGarden>(`/gardens/${id}/repot`, payload)
}

export function bindReminder(id: number, careReminderId: number) {
  return request.put<never, UserGarden>(`/gardens/${id}/reminder`, { care_reminder_id: careReminderId })
}

export function removeGarden(id: number) {
  return request.delete<never, { removed: boolean }>(`/gardens/${id}`)
}
