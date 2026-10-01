<template>
  <el-table :data="reminders" stripe empty-text="暂无养护提醒">
    <el-table-column label="花盆" width="150">
      <template #default="{ row }">
        <span v-if="row.user_garden_id" class="pot-cell">
          <el-tag size="small" type="info">#{{ row.pot_number || row.user_garden_id }}</el-tag>
          <span class="pot-name">{{ row.pot_nickname || '未命名' }}</span>
        </span>
        <span v-else class="pot-unbound">未绑定花盆</span>
      </template>
    </el-table-column>
    <el-table-column prop="task_title" label="任务" min-width="180" />
    <el-table-column label="提醒日期" width="120">
      <template #default="{ row }">{{ formatDate(row.remind_date) }}</template>
    </el-table-column>
    <el-table-column label="频率" width="100">
      <template #default="{ row }">{{ frequencyText(row.frequency) }}</template>
    </el-table-column>
    <el-table-column label="状态" width="110">
      <template #default="{ row }">
        <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="操作" width="160">
      <template #default="{ row }">
        <el-button v-if="row.status !== 'done'" size="small" type="success" @click="$emit('done', row.id)">完成</el-button>
        <el-button size="small" type="danger" @click="$emit('remove', row.id)">删除</el-button>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup lang="ts">
import type { CareReminder } from '@/types/api'
import { formatDate } from '@/utils/dateFormat'

defineProps<{ reminders: CareReminder[] }>()
defineEmits<{ (e: 'done', id: number): void; (e: 'remove', id: number): void }>()

function statusText(s: string): string {
  return s === 'pending' ? '待处理' : s === 'done' ? '已完成' : '已逾期'
}
function statusType(s: string): 'warning' | 'success' | 'danger' {
  return s === 'pending' ? 'warning' : s === 'done' ? 'success' : 'danger'
}
function frequencyText(f: string): string {
  const map: Record<string, string> = { daily: '每日', weekly: '每周', monthly: '每月', yearly: '每年' }
  return map[f] || f || '-'
}
</script>

<style scoped>
.pot-cell { display: inline-flex; align-items: center; gap: 6px; }
.pot-name { color: #606266; font-size: 12px; }
.pot-unbound { color: #c0c4cc; font-size: 12px; }
</style>
