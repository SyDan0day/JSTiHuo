<script setup lang="ts">
import { flowError } from '../../api/flow-error';
import { onMounted, ref } from 'vue';
import { request, RequestError } from '../../api/client';
import type { components } from '../../api/generated/schema';
const props = defineProps<{ adminOrderId?: string }>();
const shipments = ref<components['schemas']['Shipment'][]>([]);
const events = ref<Record<string, components['schemas']['TrackingEvent'][]>>({});
const eventNext = ref<Record<string, string>>({});
const next = ref(''),
  busy = ref(false),
  error = ref('');
const prefix = props.adminOrderId ? '/api/v1/admin' : '/api/v1';
/**
 * load 用途：读取本次授权订单的实际运单与轨迹。
 * 参数：more 表示沿固定游标加载下一页。
 * 返回值：完成操作的 Promise，失败显示错误。
 */
async function load(more = false) {
  busy.value = true;
  error.value = '';
  try {
    const path = props.adminOrderId
      ? prefix + '/orders/' + props.adminOrderId + '/shipments'
      : '/api/v1/order/shipments';
    const page = await request<components['schemas']['ShipmentList']>(
      path + '?limit=20' + (more ? '&after_id=' + next.value : ''),
    );
    shipments.value = more ? [...shipments.value, ...page.items] : page.items;
    next.value = page.next_after_id ?? '';
    for (const item of page.items) {
      const pageEvents = await request<components['schemas']['TrackingEventList']>(
        prefix + '/shipments/' + item.id + '/events?limit=100',
      );
      events.value[item.id] = pageEvents.items;
      eventNext.value[item.id] = pageEvents.next_after_id ?? '';
    }
  } catch (e) {
    error.value = e instanceof RequestError ? flowError(e) : '物流读取失败，请重试';
  } finally {
    busy.value = false;
  }
}
/**
 * moreEvents 用途：分页读取指定运单的较早轨迹。
 * 参数：id 已显示的运单标识。
 * 返回值：完成操作的 Promise，失败显示错误并保留游标。
 */
async function moreEvents(id: string) {
  busy.value = true;
  error.value = '';
  try {
    const page = await request<components['schemas']['TrackingEventList']>(
      prefix + '/shipments/' + id + '/events?limit=100&after_id=' + eventNext.value[id],
    );
    events.value[id] = [...(events.value[id] ?? []), ...page.items];
    eventNext.value[id] = page.next_after_id ?? '';
  } catch (e) {
    error.value = e instanceof RequestError ? flowError(e) : '轨迹读取失败，请重试';
  } finally {
    busy.value = false;
  }
}
onMounted(() => load());
</script>
<template>
  <section class="flow-page">
    <h1 v-if="!adminOrderId">订单物流</h1>
    <h2>物流轨迹</h2>
    <p role="alert">{{ error }}</p>
    <button :disabled="busy" @click="load()">刷新物流</button>
    <p v-if="!busy && !shipments.length && !error">暂无运单</p>
    <article v-for="shipment in shipments" :key="shipment.id">
      <h3>{{ shipment.tracking_no }}</h3>
      <p>{{ shipment.kind }} · {{ shipment.status }} · {{ shipment.shipped_at }}</p>
      <p v-if="events[shipment.id]?.length === 0">尚未收到物流轨迹</p>
      <ol>
        <li v-for="event in events[shipment.id]" :key="event.id">
          {{ event.occurred_at }} · {{ event.event_type }} · {{ event.description }}
        </li>
      </ol>
      <button v-if="eventNext[shipment.id]" :disabled="busy" @click="moreEvents(shipment.id)">
        更早轨迹
      </button>
    </article>
    <button v-if="next" :disabled="busy" @click="load(true)">更多运单</button>
  </section>
</template>
