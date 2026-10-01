import request from '@/utils/request'
import type { CareReminder, GardenView, UserGarden } from '@/types/api'

export function listGardens() {
  return request.get<never, GardenView[]>('/gardens')
}

export function addGarden(payload: { plant_species_id: number; nickname?: string; owned_since?: string; location?: string }) {
  return request.post<never, UserGarden>('/gardens', payload)
}

export function updateGarden(id: number, payload: { nickname?: string; location?: string }) {
  return request.put<never, UserGarden>(`/gardens/${id}`, payload)
}

// 换盆：更新接管时间，未完成提醒按新起算日重排，返回更新后的花盆（含提醒）
export function repotGarden(id: number, payload: { owned_since: string; nickname?: string; location?: string }) {
  return request.post<never, GardenView>(`/gardens/${id}/repot`, payload)
}

export function bindReminder(id: number, careReminderId: number) {
  return request.put<never, CareReminder>(`/gardens/${id}/reminder`, { care_reminder_id: careReminderId })
}

export function removeGarden(id: number) {
  return request.delete<never, { removed: boolean }>(`/gardens/${id}`)
}
