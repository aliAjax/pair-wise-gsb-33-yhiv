<template>
  <div class="page">
    <h1>我的花园</h1>
    <el-row :gutter="16">
      <el-col :xs="24" :md="16">
        <el-card>
          <template #header>
            <div class="card-head">
              <span>花盆清单（同品种可有多盆，分盆独立登记）</span>
              <el-button size="small" type="primary" @click="openAdd">登记新花盆</el-button>
            </div>
          </template>
          <el-table :data="gardenItems" empty-text="花园还是空的，先登记一盆吧" :row-class-name="rowClass">
            <el-table-column label="花盆 / 植物" min-width="180">
              <template #default="{ row }">
                <div class="pot-line">
                  <el-tag size="small" type="success">{{ row.pot_no }}</el-tag>
                  <el-tag v-if="row.status === 'repotted'" size="small" type="info">已换盆</el-tag>
                </div>
                <span class="garden-name">{{ row.nickname || row.plant_name || ('品种#' + row.plant_species_id) }}</span>
                <div v-if="row.plant_name" class="pot-species">品种：{{ row.plant_name }}</div>
              </template>
            </el-table-column>
            <el-table-column label="位置" prop="location" min-width="110" />
            <el-table-column label="接管时间" width="110">
              <template #default="{ row }">{{ formatDate(row.owned_since) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="210">
              <template #default="{ row }">
                <el-button size="small" :disabled="row.status !== 'active'" @click="openEdit(row)">编辑</el-button>
                <el-button size="small" type="warning" :disabled="row.status !== 'active'" @click="openRepot(row)">换盆</el-button>
                <el-button size="small" type="danger" @click="remove(row.id)">移除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
        <el-card class="block">
          <template #header>我的养护提醒（绑定具体花盆）</template>
          <el-form inline>
            <el-form-item label="花盆">
              <el-select v-model="remForm.garden_id" placeholder="选择花盆" style="width: 230px">
                <el-option
                  v-for="g in activePots"
                  :key="g.id"
                  :label="`${g.pot_no} ${g.plant_name || g.nickname}（${g.location || '未填位置'}）`"
                  :value="g.id"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="任务"><el-input v-model="remForm.task_title" placeholder="如：浇水" /></el-form-item>
            <el-form-item label="日期"><el-date-picker v-model="remForm.remind_date" type="date" value-format="YYYY-MM-DD" /></el-form-item>
            <el-form-item label="频率">
              <el-select v-model="remForm.frequency" placeholder="频率" clearable style="width: 100px">
                <el-option label="每日" value="daily" />
                <el-option label="每周" value="weekly" />
                <el-option label="每月" value="monthly" />
                <el-option label="每年" value="yearly" />
              </el-select>
            </el-form-item>
            <el-form-item><el-button type="primary" @click="submitReminder">创建提醒</el-button></el-form-item>
          </el-form>
          <ReminderList :reminders="reminders" @done="markDone" @remove="removeReminder" />
        </el-card>
      </el-col>
      <el-col :xs="24" :md="8">
        <el-card>
          <template #header>我的收藏</template>
          <el-table :data="favorites" empty-text="暂无收藏">
            <el-table-column prop="target_type" label="类型" width="80">
              <template #default="{ row }">{{ FavoriteTargetTypeMap[row.target_type as FavoriteTargetType] }}</template>
            </el-table-column>
            <el-table-column prop="target_id" label="目标 ID" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <!-- 登记 / 编辑花盆 -->
    <el-dialog v-model="potDialog" :title="editPot ? '编辑花盆' : '登记新花盆'" width="460px">
      <el-form label-width="90px">
        <el-form-item label="品种" required>
          <el-select v-model="potForm.plant_species_id" :disabled="!!editPot" filterable placeholder="选择品种" style="width: 100%">
            <el-option v-for="p in species" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="花盆编号"><el-input :model-value="potNoPreview" disabled /></el-form-item>
        <el-form-item label="昵称"><el-input v-model="potForm.nickname" placeholder="如：阳台月季" /></el-form-item>
        <el-form-item label="位置"><el-input v-model="potForm.location" placeholder="如：南阳台" /></el-form-item>
        <el-form-item label="接管时间" required>
          <el-date-picker v-model="potForm.owned_since" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="potDialog = false">取消</el-button>
        <el-button type="primary" @click="savePot">保存</el-button>
      </template>
    </el-dialog>

    <!-- 换盆 -->
    <el-dialog v-model="repotDialog" title="换盆（登记新盆并重排未完成提醒）" width="480px">
      <el-form label-width="110px" v-if="repotTarget">
        <el-form-item label="原花盆"><el-input :model-value="`${repotTarget.pot_no} ${repotTarget.plant_name || repotTarget.nickname}`" disabled /></el-form-item>
        <el-form-item label="新花盆编号"><el-input :model-value="newPotNoPreview" disabled /></el-form-item>
        <el-form-item label="新位置"><el-input v-model="repotForm.location" placeholder="如：东阳台" /></el-form-item>
        <el-form-item label="新接管时间" required>
          <el-date-picker v-model="repotForm.owned_since" type="date" value-format="YYYY-MM-DD" :disabled-date="disableBefore(repotTarget.owned_since)" style="width: 100%" />
        </el-form-item>
      </el-form>
      <el-alert type="warning" :closable="false" show-icon>
        <template #title>未完成提醒会按新接管日整体顺延；已完成记录保留在原盆不动。</template>
      </el-alert>
      <template #footer>
        <el-button @click="repotDialog = false">取消</el-button>
        <el-button type="warning" @click="confirmRepot">确认换盆</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import ReminderList from '@/components/common/ReminderList.vue'
import { listGardens, addGarden, updateGarden, repotGarden, removeGarden } from '@/api/garden'
import { listFavorites } from '@/api/favorite'
import { listReminders, deleteReminder, updateReminderStatus, createReminder } from '@/api/reminder'
import { listPlants } from '@/api/plant'
import { FavoriteTargetTypeMap, type Favorite, type FavoriteTargetType } from '@/constants/favorite'
import type { PlantSpecies } from '@/constants/plant'
import { formatDate } from '@/utils/dateFormat'
import type { CareReminder, UserGarden } from '@/types/api'

const gardenItems = ref<UserGarden[]>([])
const favorites = ref<Favorite[]>([])
const reminders = ref<CareReminder[]>([])
const species = ref<PlantSpecies[]>([])

const route = useRoute()

const activePots = computed(() => gardenItems.value.filter((g) => g.status === 'active'))

async function refreshAll() {
  gardenItems.value = await listGardens()
  reminders.value = await listReminders()
}

onMounted(async () => {
  await refreshAll()
  favorites.value = await listFavorites()
  species.value = (await listPlants({ page_size: 200 })).list
  const addPlantId = Number(route.query.add_plant)
  if (addPlantId && species.value.some((p) => p.id === addPlantId)) {
    openAdd()
    potForm.plant_species_id = addPlantId
    potForm.nickname = species.value.find((p) => p.id === addPlantId)?.name || ''
  }
})

// ---------- pot registration / edit ----------
const potDialog = ref(false)
const editPot = ref<UserGarden | null>(null)
const potForm = reactive({ plant_species_id: undefined as number | undefined, nickname: '', location: '', owned_since: '' })
const potNoPreview = computed(() => editPot.value ? editPot.value.pot_no : `P${String(gardenItems.value.length + 1).padStart(4, '0')}`)

function openAdd() {
  editPot.value = null
  potForm.plant_species_id = undefined
  potForm.nickname = ''
  potForm.location = ''
  potForm.owned_since = new Date().toISOString().slice(0, 10)
  potDialog.value = true
}
function openEdit(row: UserGarden) {
  editPot.value = row
  potForm.plant_species_id = row.plant_species_id
  potForm.nickname = row.nickname
  potForm.location = row.location
  potForm.owned_since = formatDate(row.owned_since)
  potDialog.value = true
}
async function savePot() {
  if (!editPot.value && !potForm.plant_species_id) {
    ElMessage.warning('请选择品种')
    return
  }
  if (!potForm.owned_since) {
    ElMessage.warning('请选择接管时间')
    return
  }
  if (editPot.value) {
    await updateGarden(editPot.value.id, { nickname: potForm.nickname, location: potForm.location, owned_since: potForm.owned_since })
    ElMessage.success('花盆信息已更新')
  } else {
    await addGarden({
      plant_species_id: potForm.plant_species_id!,
      nickname: potForm.nickname,
      location: potForm.location,
      owned_since: potForm.owned_since,
    })
    ElMessage.success('新花盆已登记')
  }
  potDialog.value = false
  await refreshAll()
}

// ---------- repotting ----------
const repotDialog = ref(false)
const repotTarget = ref<UserGarden | null>(null)
const repotForm = reactive({ location: '', owned_since: '' })
const newPotNoPreview = computed(() => `P${String(gardenItems.value.length + 1).padStart(4, '0')}`)

function disableBefore(min: string) {
  return (d: Date) => d.getTime() < new Date(min).getTime()
}
function openRepot(row: UserGarden) {
  repotTarget.value = row
  repotForm.location = row.location
  repotForm.owned_since = new Date().toISOString().slice(0, 10)
  repotDialog.value = true
}
async function confirmRepot() {
  if (!repotForm.owned_since) {
    ElMessage.warning('请选择新接管时间')
    return
  }
  const newPot = await repotGarden(repotTarget.value!.id, {
    owned_since: repotForm.owned_since,
    location: repotForm.location,
  })
  repotDialog.value = false
  ElMessage.success(`已换到新盆 ${newPot.pot_no}，未完成提醒已重排，已完成记录保留在原盆`)
  await refreshAll()
}

// ---------- reminders ----------
const remForm = reactive({ garden_id: undefined as number | undefined, task_title: '', remind_date: '', frequency: '' })

async function submitReminder() {
  if (!remForm.garden_id || !remForm.task_title || !remForm.remind_date) {
    ElMessage.warning('请选择花盆、填写任务与日期')
    return
  }
  const result = await createReminder({
    garden_id: remForm.garden_id,
    task_title: remForm.task_title,
    remind_date: remForm.remind_date,
    frequency: remForm.frequency || undefined,
  })
  reminders.value = await listReminders()
  remForm.task_title = ''
  remForm.remind_date = ''
  remForm.frequency = ''
  ElMessage.success(result.already_existed ? '该计划已存在，返回已有计划' : '养护提醒已创建')
}
async function markDone(id: number) {
  await updateReminderStatus(id, 'done')
  reminders.value = await listReminders()
}
async function removeReminder(id: number) {
  await deleteReminder(id)
  reminders.value = await listReminders()
}

async function remove(id: number) {
  try {
    await ElMessageBox.confirm('移除花盆会同时删除其全部提醒，确认？', '确认移除', { type: 'warning' })
  } catch {
    return
  }
  await removeGarden(id)
  await refreshAll()
  ElMessage.success('花盆及其提醒已移除')
}

function rowClass({ row }: { row: UserGarden }) {
  return row.status === 'repotted' ? 'repotted-row' : ''
}
</script>

<style scoped>
.page { max-width: 1200px; margin: 0 auto; }
.block { margin-top: 16px; }
.card-head { display: flex; align-items: center; justify-content: space-between; }
.garden-name { font-weight: 600; }
.pot-line { display: flex; gap: 6px; margin-bottom: 4px; }
.pot-species { font-size: 12px; color: #909399; margin-top: 2px; }
:deep(.repotted-row) { color: #909399; background-color: #fafafa; }
</style>
