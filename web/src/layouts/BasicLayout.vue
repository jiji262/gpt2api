<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { useUserStore } from '@/stores/user'
import { useSiteStore } from '@/stores/site'
import type { MenuItem } from '@/api/auth'

const store = useUserStore()
const site = useSiteStore()
const router = useRouter()
const route = useRoute()

const siteName = computed(() => site.get('site.name', 'GPT2API'))
const siteLogo = computed(() => site.get('site.logo_url', ''))

const { menu, user, role } = storeToRefs(store)
const collapsed = ref(false)
const loadingMenu = ref(false)

const activePath = computed(() => route.path)

const titleMap = computed(() => {
  const m = new Map<string, string>()
  function walk(items: MenuItem[]) {
    for (const it of items) {
      if (it.path) m.set(it.path, it.title)
      if (it.children) walk(it.children)
    }
  }
  walk(menu.value)
  return m
})

const currentTitle = computed(
  () => titleMap.value.get(activePath.value) || (route.meta.title as string) || '',
)

const crumb = computed(() => {
  if (activePath.value.startsWith('/admin')) return '管理员'
  if (activePath.value.startsWith('/personal')) return '个人中心'
  return ''
})

async function loadMenu() {
  if (menu.value.length > 0) return
  loadingMenu.value = true
  try { await store.fetchMenu() } finally { loadingMenu.value = false }
}

async function logout() {
  await store.logout()
  router.replace('/login')
}

function goto(path?: string) { if (path) router.push(path) }

onMounted(loadMenu)
watch(() => store.isLoggedIn, (v) => { if (v) loadMenu() })
</script>

<template>
  <el-container class="layout-root">
    <el-aside :width="collapsed ? '64px' : '240px'" class="sidebar dark-area">
      <div class="logo">
        <img v-if="siteLogo" :src="siteLogo" class="logo-img" alt="logo" />
        <span v-else class="logo-mark">{{ (siteName[0] || 'G').toUpperCase() }}</span>
        <span v-if="!collapsed" class="logo-name">{{ siteName }}</span>
      </div>
      <el-menu
        :default-active="activePath"
        :collapse="collapsed"
        class="side-menu"
        router
      >
        <template v-for="group in menu" :key="group.key">
          <el-menu-item v-if="!group.children?.length && group.path" :index="group.path">
            <el-icon v-if="group.icon"><component :is="group.icon" /></el-icon>
            <template #title>{{ group.title }}</template>
          </el-menu-item>
          <el-sub-menu v-else-if="group.children?.length" :index="group.key">
            <template #title>
              <el-icon v-if="group.icon"><component :is="group.icon" /></el-icon>
              <span>{{ group.title }}</span>
            </template>
            <el-menu-item
              v-for="child in group.children"
              :key="child.key"
              :index="child.path!"
            >
              <el-icon v-if="child.icon"><component :is="child.icon" /></el-icon>
              <template #title>{{ child.title }}</template>
            </el-menu-item>
          </el-sub-menu>
        </template>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="topbar">
        <div class="topbar__left">
          <el-button link @click="collapsed = !collapsed" class="collapse-btn">
            <el-icon :size="18"><component :is="collapsed ? 'Expand' : 'Fold'" /></el-icon>
          </el-button>
          <div class="topbar__title-block">
            <div v-if="crumb" class="topbar__crumb">{{ crumb }}</div>
            <span class="topbar__title">{{ currentTitle }}</span>
          </div>
        </div>
        <div class="topbar__right">
          <el-dropdown trigger="click" @command="(c: string) => c === 'logout' ? logout() : goto(c)">
            <span class="user-entry">
              <UserAvatar :name="user?.nickname || user?.email || 'U'" size="sm" />
              <span class="nick">{{ user?.nickname || user?.email }}</span>
              <StatusTag v-if="role === 'admin'" variant="admin">管理员</StatusTag>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="/personal/dashboard">
                  <el-icon><User /></el-icon> 个人中心
                </el-dropdown-item>
                <el-dropdown-item command="/personal/billing">
                  <el-icon><Wallet /></el-icon> 账单
                </el-dropdown-item>
                <el-dropdown-item divided command="logout">
                  <el-icon><SwitchButton /></el-icon> 退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="main" v-loading="loadingMenu">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.layout-root { height: 100vh; }

.sidebar {
  background: $n-ink-p;
  transition: width 0.2s;
  overflow-x: hidden;
  border-right: 1px solid $dark-border;
}

.logo {
  height: 64px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  color: $dark-text-1;
  font-weight: 800;
  letter-spacing: -0.01em;

  .logo-img {
    width: 32px; height: 32px; border-radius: $r-md; object-fit: contain; background: white;
  }
  .logo-mark {
    display: inline-flex;
    width: 32px; height: 32px;
    border-radius: $r-md;
    background: $c-pink;
    color: white;
    align-items: center;
    justify-content: center;
    font-size: 16px;
    font-weight: 900;
  }
  .logo-name { font-size: 17px; }
}

.side-menu {
  border-right: none;
  padding: 8px;

  :deep(.el-menu-item),
  :deep(.el-sub-menu__title) {
    height: 44px;
    line-height: 44px;
    color: $dark-text-3;
    font-weight: 500;
    font-size: 14px;

    &:hover {
      background-color: rgba(255, 255, 255, 0.04) !important;
      color: $dark-text-1;
    }
  }

  :deep(.el-menu-item.is-active) {
    background-color: rgba(255, 61, 148, 0.12) !important;
    color: $dark-text-1;
    font-weight: 700;
    border-left: 3px solid $c-pink;
    padding-left: 17px !important;
  }

  :deep(.el-sub-menu .el-menu-item) {
    padding-left: 50px !important;
    &.is-active { padding-left: 47px !important; }
  }
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
  background: $n-paper;
  color: $n-ink;
  border-bottom: 1px solid $gray-100;
  padding: 0 24px;

  &__left { display: flex; align-items: center; gap: 14px; }
  &__right { display: inline-flex; align-items: center; gap: 12px; }

  &__title-block { display: flex; flex-direction: column; line-height: 1.1; }
  &__crumb {
    font-size: var(--fs-xs);
    letter-spacing: 0.14em;
    text-transform: uppercase;
    font-weight: 700;
    color: $gray-500;
  }
  &__title { font-size: 16px; font-weight: 700; color: $n-ink; letter-spacing: -0.01em; }

  .collapse-btn { color: $gray-600; }

  .user-entry {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    color: $n-ink;
    font-size: 14px;
    .nick { font-weight: 500; }
  }
}

.main {
  background: $n-cloud;
  padding: 0;
}

.fade-enter-active, .fade-leave-active { transition: opacity 0.15s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
