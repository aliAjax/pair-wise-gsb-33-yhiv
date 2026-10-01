<template>
  <div class="page">
    <h1>我的花园</h1>
    <el-row :gutter="16">
      <el-col :xs="24" :md="16">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>花盆清单（同一品种可登记多盆，每盆独立位置与接管时间）</span>
              <el-button type="primary" size="small" @click="openAddPot">+ 登记花盆</el-button>
            </div>
          </template>
          <el-table :data="pots" empty-text="花园还是空的，去品种库登记第一盆吧" row-key="id">
            <el-table-column label="花盆" min-width="180">
              <template #default="{ row }">
                <el-tag size="small" type="info">#{{ row.id }}</el-tag>
                <span class="garden-name">{{ row.nickname || speciesName(row.plant_species_id) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="位置" prop="location" width="140" />
            <el-table-column label="接管时间" width="120">
              <template #default="{ row }">{{ formatDate(row.owned_since) }}</template>
            </el-table-column>
            <el-table-column label="提醒" width="80" align="center">
              <template #default="{ row }">
                <el-badge :value="row.reminders.length" :hidden="row.reminders.length === 0" type="primary" />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="280">
              <template #default="{ row }">
                <el-button size="small" type="primary" plain @click="openAddReminder(row)">加提醒</el-button>
                <el-button size="small" @click="openEditPot(row)">编辑</el-button>
                <el-button size="small" type="warning" plain @click="openRepot(row)">换盆</el-button>
                <el-button size="small" type="danger" plain @click="remove(row.id)">移除</el-button>
              </template>
            </el-table-column>
            <el-table-column type="expand">
              <template #default="{ row }">
                <div class="pot-reminders">
                  <ReminderList
                    :reminders="row.reminders"
                    @done="markDone"
                    @remove="removeReminder"
                  />
                </div>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card v-if="unboundReminders.length" class="block">
          <template #header>未绑定花盆的旧提醒</template>
          <ReminderList :reminders="unboundReminders" @done="markDone" @remove="removeReminder" />
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

    <!-- 登记新花盆 -->
    <el-dialog v-model="addPotVisible" title="登记花盆" width="460px">
      <el-form :model="potForm" label-width="92px">
        <el-form-item label="品种" required>
          <el-select v-model="potForm.plant_species_id" filterable placeholder="选择品种" style="width: 100%">
            <el-option v-for="p in species" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="花盆昵称"><el-input v-model="potForm.nickname" placeholder="如：月季·分株苗" /></el-form-item>
        <el-form-item label="位置"><el-input v-model="potForm.location" placeholder="如：南阳台 A2" /></el-form-item>
        <el-form-item label="接管时间">
          <el-date-picker v-model="potForm.owned_since" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addPotVisible = false">取消</el-button>
        <el-button type="primary" @click="submitAddPot">登记</el-button>
      </template>
    </el-dialog>

    <!-- 编辑花盆位置/昵称 -->
    <el-dialog v-model="editPotVisible" title="编辑花盆" width="460px">
      <el-form :model="potForm" label-width="92px">
        <el-form-item label="花盆昵称"><el-input v-model="potForm.nickname" /></el-form-item>
        <el-form-item label="位置"><el-input v-model="potForm.location" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editPotVisible = false">取消</el-button>
        <el-button type="primary" @click="submitEditPot">保存</el-button>
      </template>
    </el-dialog>

    <!-- 换盆：新接管时间，未完成提醒重排 -->
    <el-dialog v-model="repotVisible" title="换盆登记" width="460px">
      <el-alert type="info" :closable="false" show-icon class="repot-tip"
        title="保存后，该盆未完成的提醒会按新接管时间重新排期；已完成记录保持原样。" />
      <el-form :model="repotForm" label-width="92px">
        <el-form-item label="新接管时间" required>
          <el-date-picker v-model="repotForm.owned_since" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="花盆昵称"><el-input v-model="repotForm.nickname" /></el-form-item>
        <el-form-item label="新位置"><el-input v-model="repotForm.location" placeholder="如：北窗台 B1" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="repotVisible = false">取消</el-button>
        <el-button type="warning" @click="submitRepot">确认换盆并重排提醒</el-button>
      </template>
    </el-dialog>

    <!-- 为某盆添加养护提醒 -->
    <el-dialog v-model="reminderVisible" :title="`给花盆 #${reminderForm.user_garden_id} 添加养护提醒`" width="460px">
      <el-form :model="reminderForm" label-width="92px">
        <el-form-item label="任务" required><el-input v-model="reminderForm.task_title" placeholder="如：浇水、施肥" /></el-form-item>
        <el-form-item label="日期" required>
          <el-date-picker v-model="reminderForm.remind_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="频率">
          <el-select v-model="reminderForm.frequency" style="width: 100%">
            <el-option label="单次" value="" />
            <el-option label="每日" value="daily" />
            <el-option label="每周" value="weekly" />
            <el-option label="每月" value="monthly" />
            <el-option label="每年" value="yearly" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reminderVisible = false">取消</el-button>
        <el-button type="primary" @click="submitReminder">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import ReminderList from '@/components/common/ReminderList.vue'
import {
  listGardens, addGarden, updateGarden, repotGarden, removeGarden,
} from '@/api/garden'
import { listFavorites } from '@/api/favorite'
import {
  createReminder, deleteReminder, updateReminderStatus,
} from '@/api/reminder'
import { listPlants } from '@/api/plant'
import { FavoriteTargetTypeMap, type Favorite, type FavoriteTargetType } from '@/constants/favorite'
import type { PlantSpecies } from '@/constants/plant'
import { formatDate } from '@/utils/dateFormat'
import type { CareReminder, GardenView } from '@/types/api'

const gardenGroups = ref<GardenView[]>([])
const favorites = ref<Favorite[]>([])
const species = ref<PlantSpecies[]>([])
const speciesMap = computed(() => new Map(species.value.map((p) => [p.id, p.name])))

// 花园、日历和提醒列表共用同一后端结果；id=0 的分组是未绑定花盆的旧提醒
const pots = computed(() => gardenGroups.value.filter((g) => g.id > 0))
const unboundReminders = computed<CareReminder[]>(() => {
  const bucket = gardenGroups.value.find((g) => g.id === 0)
  return bucket ? bucket.reminders : []
})

const addPotVisible = ref(false)
const editPotVisible = ref(false)
const repotVisible = ref(false)
const reminderVisible = ref(false)
const editingPotId = ref(0)

const potForm = reactive({ plant_species_id: 0, nickname: '', location: '', owned_since: '' })
const repotForm = reactive({ owned_since: '', nickname: '', location: '' })
const reminderForm = reactive({
  user_garden_id: 0,
  task_title: '',
  remind_date: '',
  frequency: '',
})

onMounted(async () => {
  await refresh()
  species.value = (await listPlants({ page_size: 200 })).list
})

async function refresh() {
  gardenGroups.value = await listGardens()
  favorites.value = await listFavorites()
}

function speciesName(id: number): string {
  return speciesMap.value.get(id) || `品种#${id}`
}

function openAddPot() {
  Object.assign(potForm, { plant_species_id: 0, nickname: '', location: '', owned_since: '' })
  addPotVisible.value = true
}
async function submitAddPot() {
  if (!potForm.plant_species_id) {
    ElMessage.warning('请选择品种')
    return
  }
  await addGarden({ ...potForm })
  addPotVisible.value = false
  await refresh()
  ElMessage.success('花盆已登记')
}

function openEditPot(row: GardenView) {
  editingPotId.value = row.id
  Object.assign(potForm, {
    plant_species_id: row.plant_species_id,
    nickname: row.nickname,
    location: row.location,
    owned_since: row.owned_since,
  })
  editPotVisible.value = true
}
async function submitEditPot() {
  await updateGarden(editingPotId.value, { nickname: potForm.nickname, location: potForm.location })
  editPotVisible.value = false
  await refresh()
  ElMessage.success('花盆信息已更新')
}

function openRepot(row: GardenView) {
  editingPotId.value = row.id
  Object.assign(repotForm, {
    owned_since: formatDate(row.owned_since),
    nickname: row.nickname,
    location: row.location,
  })
  repotVisible.value = true
}
async function submitRepot() {
  if (!repotForm.owned_since) {
    ElMessage.warning('请选择新接管时间')
    return
  }
  const updated = await repotGarden(editingPotId.value, { ...repotForm })
  repotVisible.value = false
  await refresh()
  const moved = updated.reminders.filter((r) => r.status !== 'done').length
  ElMessage.success(`换盆完成，已重排 ${moved} 条未完成提醒；已完成记录保持原样`)
}

function openAddReminder(row: GardenView) {
  Object.assign(reminderForm, {
    user_garden_id: row.id,
    task_title: '',
    remind_date: '',
    frequency: '',
  })
  reminderVisible.value = true
}
async function submitReminder() {
  if (!reminderForm.task_title || !reminderForm.remind_date) {
    ElMessage.warning('请填写任务与日期')
    return
  }
  try {
    await createReminder({ ...reminderForm })
    ElMessage.success('养护提醒已创建，已绑定该花盆')
  } catch (e: any) {
    // 两台设备同时提交同一计划时，后到的请求返回已有计划
    if (e?.existing) {
      ElMessage.warning(`已有相同计划（花盆 #${e.existing.pot_number}），已为你打开已有计划`)
    } else {
      throw e
    }
  }
  reminderVisible.value = false
  await refresh()
}

async function remove(id: number) {
  await ElMessageBox.confirm('移除花盆后，其提醒会保留但解除花盆绑定，确认移除？', '确认', { type: 'warning' })
  await removeGarden(id)
  await refresh()
  ElMessage.success('已移除花盆')
}
async function markDone(id: number) {
  await updateReminderStatus(id, 'done')
  await refresh()
}
async function removeReminder(id: number) {
  await deleteReminder(id)
  await refresh()
}
</script>

<style scoped>
.page { max-width: 1200px; margin: 0 auto; }
.block { margin-top: 16px; }
.card-header { display: flex; align-items: center; justify-content: space-between; }
.garden-name { font-weight: 600; margin-left: 8px; }
.pot-reminders { padding: 8px 24px 16px; }
.repot-tip { margin-bottom: 12px; }
</style>
