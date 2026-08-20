<template>
  <span class="cat-icon" :style="{ background: bg, color: color }">
    <el-icon :size="size"><component :is="icon || 'More'" /></el-icon>
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  icon: { type: String, default: 'More' },
  color: { type: String, default: '#909399' },
  size: { type: Number, default: 20 },
})

const bg = computed(() => {
  const c = props.color || '#909399'
  return c + '22' // 10% 透明度背景
})
</script>

<style scoped>
.cat-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  width: 44px;
  height: 44px;
  flex-shrink: 0;
}
</style>
