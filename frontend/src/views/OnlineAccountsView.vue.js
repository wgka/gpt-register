import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
function normalizeManagementEndpoint(url) {
    const raw = (url || '').trim();
    if (!raw)
        return '';
    try {
        const parsed = new URL(raw);
        const pathname = parsed.pathname.replace(/\/+$/, '');
        if (!pathname) {
            return `${parsed.origin}/v0/management/auth-files`;
        }
        return `${parsed.origin}${pathname}`;
    }
    catch {
        return '';
    }
}
function normalizeFixedTimeValue(value) {
    const match = value.trim().match(/^(\d{1,2}):(\d{1,2})$/);
    if (!match)
        return null;
    const hour = Number(match[1]);
    const minute = Number(match[2]);
    if (!Number.isInteger(hour) || !Number.isInteger(minute))
        return null;
    if (hour < 0 || hour > 23 || minute < 0 || minute > 59)
        return null;
    return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`;
}
function normalizeFixedTimes(values) {
    const unique = new Set();
    for (const value of values) {
        const normalized = normalizeFixedTimeValue(value);
        if (normalized) {
            unique.add(normalized);
        }
    }
    return Array.from(unique).sort();
}
const schedulerTimeOptions = Array.from({ length: 48 }, (_, index) => {
    const hour = Math.floor(index / 2);
    const minute = index % 2 === 0 ? '00' : '30';
    return `${String(hour).padStart(2, '0')}:${minute}`;
});
const managementEndpoint = ref(normalizeManagementEndpoint(import.meta.env.VITE_CPA_API_URL));
const managementToken = ref((import.meta.env.VITE_CPA_API_TOKEN || '').trim());
function defaultSchedulerConfig() {
    return {
        enabled: false,
        mode: 'interval',
        interval_minutes: 30,
        fixed_times: ['09:00'],
        disable_invalid: false,
        delete_invalid: true,
        retry_count: 2,
        retry_delay_minutes: 5,
    };
}
function defaultSchedulerState() {
    return {
        config: defaultSchedulerConfig(),
        running: false,
        retry_pending: false,
        retry_remaining: 0,
        last_result: null,
    };
}
const files = ref([]);
const loading = ref(false);
const deletingId = ref(null);
const cleaningInvalid = ref(false);
const searchText = ref('');
const filterStatus = ref('');
const tokenInvalidCodes = new Set(['token_invalidated', 'deactivated_workspace']);
const schedulerLoading = ref(false);
const schedulerSaving = ref(false);
const schedulerRunning = ref(false);
const schedulerState = ref(defaultSchedulerState());
const schedulerForm = reactive(defaultSchedulerConfig());
const schedulerLogs = ref([]);
const schedulerLogsLoading = ref(false);
const schedulerLogTotal = ref(0);
const schedulerLogQuery = reactive({
    page: 1,
    pageSize: 10,
});
let schedulerPollingTimer = null;
function syncSchedulerForm(config) {
    schedulerForm.enabled = !!config.enabled;
    schedulerForm.mode = config.mode || 'interval';
    schedulerForm.interval_minutes = config.interval_minutes || 30;
    schedulerForm.fixed_times = normalizeFixedTimes(config.fixed_times || []);
    schedulerForm.disable_invalid = false;
    schedulerForm.delete_invalid = !!config.delete_invalid;
    schedulerForm.retry_count = Math.max(config.retry_count ?? 2, 0);
    schedulerForm.retry_delay_minutes = config.retry_delay_minutes || 5;
}
function applySchedulerState(state, syncForm = false) {
    const mergedConfig = {
        ...defaultSchedulerConfig(),
        ...(state?.config || {}),
        fixed_times: normalizeFixedTimes(state?.config?.fixed_times || []),
    };
    schedulerState.value = {
        ...defaultSchedulerState(),
        ...state,
        config: mergedConfig,
        last_result: state?.last_result || null,
    };
    if (syncForm) {
        syncSchedulerForm(mergedConfig);
    }
}
function isTokenInvalid(file) {
    if (containsTokenInvalidCode(file.status_message)) {
        return true;
    }
    const payload = parseStatusMessage(file);
    if (!payload)
        return false;
    return tokenInvalidCodes.has(payload.error?.code || payload.detail?.code || '');
}
function containsTokenInvalidCode(raw) {
    if (!raw)
        return false;
    return Array.from(tokenInvalidCodes).some((code) => raw.includes(code));
}
function parseStatusMessage(file) {
    if (!file.status_message)
        return null;
    let payload = file.status_message;
    for (let i = 0; i < 2; i++) {
        if (payload && typeof payload === 'object') {
            return payload;
        }
        if (typeof payload !== 'string') {
            return null;
        }
        try {
            payload = JSON.parse(payload);
        }
        catch {
            return null;
        }
    }
    return payload && typeof payload === 'object' ? payload : null;
}
const invalidFiles = computed(() => files.value.filter(isTokenInvalid));
function effectiveStatus(file) {
    return file.disabled ? 'disabled' : file.status;
}
function usageLimitState(file) {
    if (!isUsageLimited(file)) {
        return 'none';
    }
    const resetAt = usageLimitResetAt(file);
    if (resetAt === null || resetAt > Date.now()) {
        return 'limited';
    }
    if (file.disabled) {
        return 'recoverable';
    }
    return 'recovered';
}
const filteredFiles = computed(() => {
    let result = files.value;
    if (searchText.value) {
        const q = searchText.value.toLowerCase();
        result = result.filter((f) => f.account.toLowerCase().includes(q) || f.email.toLowerCase().includes(q));
    }
    if (filterStatus.value === 'token_invalid') {
        result = result.filter(isTokenInvalid);
    }
    else if (filterStatus.value === 'usage_limited') {
        result = result.filter((f) => usageLimitState(f) === 'limited');
    }
    else if (filterStatus.value) {
        result = result.filter((f) => effectiveStatus(f) === filterStatus.value);
    }
    return result;
});
function countByStatus(status) {
    return files.value.filter((f) => effectiveStatus(f) === status).length;
}
const invalidTokenCount = computed(() => invalidFiles.value.length);
const usageLimitedCount = computed(() => files.value.filter((f) => usageLimitState(f) === 'limited').length);
const schedulerStatusLabel = computed(() => {
    if (schedulerState.value.running)
        return '执行中';
    return schedulerState.value.config.enabled ? '已启用' : '未启用';
});
const schedulerStatusTagType = computed(() => {
    if (schedulerState.value.running)
        return 'warning';
    return schedulerState.value.config.enabled ? 'success' : 'info';
});
const normalizedFormFixedTimes = computed(() => normalizeFixedTimes(schedulerForm.fixed_times));
const schedulerConfiguredActionsText = computed(() => {
    return schedulerForm.delete_invalid ? '删除失效账号' : '未选择动作';
});
const schedulerModeDescription = computed(() => {
    if (schedulerForm.mode === 'fixed_times') {
        return normalizedFormFixedTimes.value.length > 0
            ? `固定时间 ${normalizedFormFixedTimes.value.join('、')}`
            : '固定时间未配置';
    }
    return `每 ${schedulerForm.interval_minutes} 分钟执行一次`;
});
const schedulerHandledSummary = computed(() => {
    const result = schedulerState.value.last_result;
    if (!result)
        return '-';
    return `禁用 ${result.disabled_count} / 删除 ${result.deleted_count} / 失败 ${result.failed_count}`;
});
const schedulerRetrySummary = computed(() => {
    if (!schedulerState.value.retry_pending)
        return '无';
    return `剩余 ${schedulerState.value.retry_remaining} 次`;
});
const schedulerNextReasonLabel = computed(() => {
    switch (schedulerState.value.next_run_reason) {
        case 'interval':
            return '按间隔';
        case 'fixed_times':
            return '固定时间';
        case 'retry':
            return '失败重试';
        default:
            return '-';
    }
});
const schedulerResultPreview = computed(() => {
    const result = schedulerState.value.last_result;
    if (!result)
        return [];
    const messages = result.messages || [];
    const preview = messages.slice(0, 3);
    if (result.error && !preview.includes(result.error)) {
        preview.unshift(result.error);
    }
    return preview.slice(0, 3);
});
const isSchedulerFormDirty = computed(() => {
    const config = schedulerState.value.config;
    const configFixedTimes = normalizeFixedTimes(config.fixed_times || []);
    return (schedulerForm.enabled !== config.enabled ||
        schedulerForm.mode !== config.mode ||
        schedulerForm.interval_minutes !== config.interval_minutes ||
        schedulerForm.delete_invalid !== config.delete_invalid ||
        schedulerForm.retry_count !== config.retry_count ||
        schedulerForm.retry_delay_minutes !== config.retry_delay_minutes ||
        JSON.stringify(normalizedFormFixedTimes.value) !== JSON.stringify(configFixedTimes));
});
async function readJSONResponse(response) {
    const raw = await response.text();
    try {
        return JSON.parse(raw);
    }
    catch {
        const contentType = response.headers.get('content-type') || 'unknown';
        const snippet = raw.replace(/\s+/g, ' ').trim().slice(0, 120);
        throw new Error(`期望 JSON，实际返回 ${contentType}${snippet ? `: ${snippet}` : ''}`);
    }
}
async function refreshManagementConfig() {
    const response = await fetch('/api/settings');
    if (!response.ok) {
        throw new Error(`加载配置失败: HTTP ${response.status}`);
    }
    const data = await readJSONResponse(response);
    managementEndpoint.value = normalizeManagementEndpoint(data.editable?.cpa?.api_url);
    managementToken.value = (data.editable?.cpa?.api_token || '').trim();
}
async function ensureManagementConfig() {
    try {
        await refreshManagementConfig();
    }
    catch (e) {
        console.warn('load management config failed', e);
    }
    if (!managementEndpoint.value) {
        throw new Error('CPA API URL 未配置');
    }
    if (!managementToken.value) {
        throw new Error('CPA API Token 未配置');
    }
}
function numberFromUnknown(value) {
    if (typeof value === 'number') {
        return Number.isFinite(value) ? value : null;
    }
    if (typeof value === 'string' && value.trim() !== '') {
        const parsed = Number(value);
        return Number.isFinite(parsed) ? parsed : null;
    }
    return null;
}
function usageLimitResetAt(file) {
    const payload = parseStatusMessage(file);
    if (payload?.error?.type !== 'usage_limit_reached') {
        return null;
    }
    const resetsAt = numberFromUnknown(payload.error.resets_at);
    if (resetsAt !== null && resetsAt > 0) {
        return resetsAt * 1000;
    }
    const resetsInSeconds = numberFromUnknown(payload.error.resets_in_seconds);
    if (resetsInSeconds !== null) {
        return Date.now() + Math.max(resetsInSeconds, 0) * 1000;
    }
    return null;
}
function isUsageLimited(file) {
    return parseStatusMessage(file)?.error?.type === 'usage_limit_reached';
}
function shouldAutoDisableForLimit(file) {
    if (!isUsageLimited(file) || file.disabled) {
        return false;
    }
    const resetAt = usageLimitResetAt(file);
    return resetAt === null || resetAt > Date.now();
}
function shouldAutoEnableAfterLimit(file) {
    if (!file.disabled || !isUsageLimited(file)) {
        return false;
    }
    const resetAt = usageLimitResetAt(file);
    return resetAt !== null && resetAt <= Date.now();
}
async function updateFileDisabledStatus(file, disabled) {
    const response = await fetch(`${managementEndpoint.value}/status`, {
        method: 'PATCH',
        headers: {
            Authorization: `Bearer ${managementToken.value}`,
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            name: file.name,
            disabled,
        }),
    });
    if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
    }
}
async function syncUsageLimitAccountStates(nextFiles) {
    let changed = false;
    const failedAccounts = [];
    for (const file of nextFiles) {
        const shouldDisable = shouldAutoDisableForLimit(file);
        const shouldEnable = !shouldDisable && shouldAutoEnableAfterLimit(file);
        if (!shouldDisable && !shouldEnable) {
            continue;
        }
        try {
            await updateFileDisabledStatus(file, shouldDisable);
            changed = true;
        }
        catch (e) {
            console.error('sync usage limit state failed', file.name, e);
            failedAccounts.push(file.account || file.email || file.name);
        }
    }
    if (failedAccounts.length > 0) {
        ElMessage.warning(`部分限额账号状态同步失败: ${failedAccounts.join('、')}`);
    }
    return changed;
}
async function loadFiles(syncUsageLimit = true) {
    loading.value = true;
    try {
        await ensureManagementConfig();
        const response = await fetch(managementEndpoint.value, {
            headers: { Authorization: `Bearer ${managementToken.value}` },
        });
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        const data = await readJSONResponse(response);
        const nextFiles = data.files ?? [];
        if (syncUsageLimit) {
            const changed = await syncUsageLimitAccountStates(nextFiles);
            if (changed) {
                await loadFiles(false);
                return;
            }
        }
        files.value = nextFiles;
    }
    catch (e) {
        ElMessage.error('加载线上账号失败: ' + (e instanceof Error ? e.message : String(e)));
    }
    finally {
        loading.value = false;
    }
}
async function loadSchedulerState(syncForm = true, silent = false) {
    if (!silent) {
        schedulerLoading.value = true;
    }
    try {
        const response = await fetch('/api/online-accounts/scheduler');
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        const data = await readJSONResponse(response);
        applySchedulerState(data.state, syncForm);
    }
    catch (e) {
        if (!silent) {
            ElMessage.error('加载定时任务状态失败: ' + (e instanceof Error ? e.message : String(e)));
        }
    }
    finally {
        if (!silent) {
            schedulerLoading.value = false;
        }
    }
}
async function loadSchedulerLogs(silent = false) {
    if (!silent) {
        schedulerLogsLoading.value = true;
    }
    try {
        const response = await fetch(`/api/online-accounts/scheduler/logs?page=${schedulerLogQuery.page}&page_size=${schedulerLogQuery.pageSize}`);
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        const data = await readJSONResponse(response);
        schedulerLogs.value = data.logs || [];
        schedulerLogTotal.value = data.total || 0;
    }
    catch (e) {
        if (!silent) {
            ElMessage.error('加载执行日志失败: ' + (e instanceof Error ? e.message : String(e)));
        }
    }
    finally {
        if (!silent) {
            schedulerLogsLoading.value = false;
        }
    }
}
async function persistScheduler(showSuccess = true) {
    const normalizedTimes = normalizedFormFixedTimes.value;
    if (schedulerForm.enabled && !schedulerForm.delete_invalid) {
        ElMessage.warning('启用定时任务时至少选择一个动作');
        return false;
    }
    if (schedulerForm.mode === 'fixed_times' && normalizedTimes.length === 0) {
        ElMessage.warning('固定时间模式下至少配置一个执行时间，格式如 09:00');
        return false;
    }
    schedulerSaving.value = true;
    try {
        const response = await fetch('/api/online-accounts/scheduler', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                config: {
                    enabled: schedulerForm.enabled,
                    mode: schedulerForm.mode,
                    interval_minutes: schedulerForm.interval_minutes,
                    fixed_times: normalizedTimes,
                    disable_invalid: false,
                    delete_invalid: schedulerForm.delete_invalid,
                    retry_count: schedulerForm.retry_count,
                    retry_delay_minutes: schedulerForm.retry_delay_minutes,
                },
            }),
        });
        const data = await readJSONResponse(response);
        if (!response.ok) {
            throw new Error(data.error || `HTTP ${response.status}`);
        }
        applySchedulerState(data.state, true);
        if (showSuccess) {
            ElMessage.success(data.message || '定时任务配置已保存');
        }
        return true;
    }
    catch (e) {
        ElMessage.error('保存定时任务失败: ' + (e instanceof Error ? e.message : String(e)));
        return false;
    }
    finally {
        schedulerSaving.value = false;
    }
}
async function saveScheduler() {
    await persistScheduler(true);
}
async function runSchedulerNow() {
    if (schedulerForm.mode === 'fixed_times' && normalizedFormFixedTimes.value.length === 0) {
        ElMessage.warning('固定时间模式下至少配置一个执行时间后再执行');
        return;
    }
    if (!schedulerForm.delete_invalid) {
        ElMessage.warning('至少选择一个动作后再执行');
        return;
    }
    if (isSchedulerFormDirty.value) {
        const saved = await persistScheduler(false);
        if (!saved)
            return;
    }
    schedulerRunning.value = true;
    try {
        const response = await fetch('/api/online-accounts/scheduler/run', { method: 'POST' });
        const data = await readJSONResponse(response);
        if (data.state) {
            applySchedulerState(data.state, false);
        }
        if (!response.ok || data.success === false) {
            throw new Error(data.error || `HTTP ${response.status}`);
        }
        ElMessage.success(data.message || '定时任务执行完成');
        await Promise.all([loadFiles(), loadSchedulerState(false, true), loadSchedulerLogs(true)]);
    }
    catch (e) {
        ElMessage.error('执行定时任务失败: ' + (e instanceof Error ? e.message : String(e)));
    }
    finally {
        schedulerRunning.value = false;
    }
}
async function deleteFile(file) {
    try {
        await ElMessageBox.confirm(`确定要删除账号 ${file.account} 吗？此操作不可撤销。`, '删除确认', {
            type: 'warning',
        });
    }
    catch {
        return;
    }
    deletingId.value = file.id;
    try {
        await ensureManagementConfig();
        const response = await fetch(`${managementEndpoint.value}?name=${encodeURIComponent(file.name)}`, {
            method: 'DELETE',
            headers: { Authorization: `Bearer ${managementToken.value}` },
        });
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        ElMessage.success(`账号 ${file.account} 已删除`);
        await loadFiles();
    }
    catch (e) {
        ElMessage.error('删除失败: ' + (e instanceof Error ? e.message : String(e)));
    }
    finally {
        deletingId.value = null;
    }
}
async function cleanAllInvalid() {
    const targets = invalidFiles.value;
    if (targets.length === 0)
        return;
    try {
        await ElMessageBox.confirm(`确定要删除全部 ${targets.length} 个 Token 失效账号吗？此操作不可撤销。`, '批量清理确认', { type: 'warning' });
    }
    catch {
        return;
    }
    cleaningInvalid.value = true;
    let successCount = 0;
    let failCount = 0;
    try {
        await ensureManagementConfig();
        for (const file of targets) {
            try {
                const response = await fetch(`${managementEndpoint.value}?name=${encodeURIComponent(file.name)}`, {
                    method: 'DELETE',
                    headers: { Authorization: `Bearer ${managementToken.value}` },
                });
                if (response.ok) {
                    successCount++;
                }
                else {
                    failCount++;
                }
            }
            catch {
                failCount++;
            }
        }
        ElMessage.success(`清理完成，成功 ${successCount}，失败 ${failCount}`);
        await loadFiles();
    }
    finally {
        cleaningInvalid.value = false;
    }
}
function handleSchedulerLogPageChange(page) {
    schedulerLogQuery.page = page;
    void loadSchedulerLogs();
}
function handleSchedulerLogPageSizeChange(pageSize) {
    schedulerLogQuery.pageSize = pageSize;
    schedulerLogQuery.page = 1;
    void loadSchedulerLogs();
}
function formatDate(value) {
    if (!value)
        return '-';
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime()))
        return value;
    return parsed.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
    });
}
function formatRecoveryTime(file) {
    const resetAt = usageLimitResetAt(file);
    if (resetAt === null)
        return '-';
    return formatDate(new Date(resetAt).toISOString());
}
function statusTagType(file) {
    if (usageLimitState(file) === 'limited') {
        return 'warning';
    }
    switch (effectiveStatus(file)) {
        case 'active':
            return 'success';
        case 'disabled':
            return 'info';
        default:
            return 'warning';
    }
}
function statusLabel(file) {
    if (usageLimitState(file) === 'limited' && file.disabled) {
        return '限额禁用';
    }
    if (usageLimitState(file) === 'limited') {
        return '限额中';
    }
    return effectiveStatus(file);
}
function schedulerActionLabel(action) {
    switch (action) {
        case 'disable_invalid':
            return '禁用失效账号';
        case 'delete_invalid':
            return '删除失效账号';
        default:
            return action || '-';
    }
}
function schedulerTriggerLabel(triggerType) {
    switch (triggerType) {
        case 'manual':
            return '手动执行';
        case 'retry':
            return '失败重试';
        case 'scheduled':
            return '定时执行';
        default:
            return triggerType || '-';
    }
}
function schedulerRunStatusLabel(status) {
    switch (status) {
        case 'success':
            return '成功';
        case 'partial_failed':
            return '部分失败';
        case 'failed':
            return '失败';
        default:
            return status || '-';
    }
}
function schedulerRunStatusTagType(status) {
    switch (status) {
        case 'success':
            return 'success';
        case 'partial_failed':
            return 'warning';
        case 'failed':
            return 'danger';
        default:
            return 'info';
    }
}
function schedulerLogMessage(log) {
    if (log.error_message)
        return log.error_message;
    if (log.messages && log.messages.length > 0)
        return log.messages[0];
    return '-';
}
onMounted(() => {
    void Promise.all([loadFiles(), loadSchedulerState(true), loadSchedulerLogs()]);
    schedulerPollingTimer = setInterval(() => {
        void Promise.all([loadSchedulerState(false, true), loadSchedulerLogs(true)]);
    }, 30000);
});
onBeforeUnmount(() => {
    if (schedulerPollingTimer) {
        clearInterval(schedulerPollingTimer);
        schedulerPollingTimer = null;
    }
});
const __VLS_ctx = {
    ...{},
    ...{},
};
let __VLS_components;
let __VLS_intrinsics;
let __VLS_directives;
/** @type {__VLS_StyleScopedClasses['scheduler-toggle']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-toggle']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-metric']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-metric']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-result__messages']} */ ;
/** @type {__VLS_StyleScopedClasses['summary-grid']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-panel']} */ ;
/** @type {__VLS_StyleScopedClasses['summary-grid']} */ ;
/** @type {__VLS_StyleScopedClasses['toolbar']} */ ;
/** @type {__VLS_StyleScopedClasses['filters']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-config-grid']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-metrics']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-config-card--times']} */ ;
/** @type {__VLS_StyleScopedClasses['summary-grid']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-toggle']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-actions-row']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-actions']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "online-accounts-page" },
});
/** @type {__VLS_StyleScopedClasses['online-accounts-page']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.section, __VLS_intrinsics.section)({
    ...{ class: "summary-grid" },
});
/** @type {__VLS_StyleScopedClasses['summary-grid']} */ ;
let __VLS_0;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_1 = __VLS_asFunctionalComponent1(__VLS_0, new __VLS_0({
    ...{ class: "summary-card page-card" },
    shadow: "never",
}));
const __VLS_2 = __VLS_1({
    ...{ class: "summary-card page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_1));
/** @type {__VLS_StyleScopedClasses['summary-card']} */ ;
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_5 } = __VLS_3.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "summary-card__label" },
});
/** @type {__VLS_StyleScopedClasses['summary-card__label']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({
    ...{ class: "summary-card__value" },
});
/** @type {__VLS_StyleScopedClasses['summary-card__value']} */ ;
(__VLS_ctx.files.length);
// @ts-ignore
[files,];
var __VLS_3;
let __VLS_6;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_7 = __VLS_asFunctionalComponent1(__VLS_6, new __VLS_6({
    ...{ class: "summary-card page-card" },
    shadow: "never",
}));
const __VLS_8 = __VLS_7({
    ...{ class: "summary-card page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_7));
/** @type {__VLS_StyleScopedClasses['summary-card']} */ ;
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_11 } = __VLS_9.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "summary-card__label" },
});
/** @type {__VLS_StyleScopedClasses['summary-card__label']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({
    ...{ class: "summary-card__value" },
});
/** @type {__VLS_StyleScopedClasses['summary-card__value']} */ ;
(__VLS_ctx.countByStatus('active'));
// @ts-ignore
[countByStatus,];
var __VLS_9;
let __VLS_12;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_13 = __VLS_asFunctionalComponent1(__VLS_12, new __VLS_12({
    ...{ class: "summary-card page-card" },
    shadow: "never",
}));
const __VLS_14 = __VLS_13({
    ...{ class: "summary-card page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_13));
/** @type {__VLS_StyleScopedClasses['summary-card']} */ ;
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_17 } = __VLS_15.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "summary-card__label" },
});
/** @type {__VLS_StyleScopedClasses['summary-card__label']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({
    ...{ class: "summary-card__value" },
});
/** @type {__VLS_StyleScopedClasses['summary-card__value']} */ ;
(__VLS_ctx.countByStatus('disabled'));
// @ts-ignore
[countByStatus,];
var __VLS_15;
let __VLS_18;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_19 = __VLS_asFunctionalComponent1(__VLS_18, new __VLS_18({
    ...{ class: "summary-card page-card" },
    shadow: "never",
}));
const __VLS_20 = __VLS_19({
    ...{ class: "summary-card page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_19));
/** @type {__VLS_StyleScopedClasses['summary-card']} */ ;
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_23 } = __VLS_21.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "summary-card__label" },
});
/** @type {__VLS_StyleScopedClasses['summary-card__label']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({
    ...{ class: "summary-card__value" },
});
/** @type {__VLS_StyleScopedClasses['summary-card__value']} */ ;
(__VLS_ctx.usageLimitedCount);
// @ts-ignore
[usageLimitedCount,];
var __VLS_21;
let __VLS_24;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_25 = __VLS_asFunctionalComponent1(__VLS_24, new __VLS_24({
    ...{ class: "summary-card page-card" },
    shadow: "never",
}));
const __VLS_26 = __VLS_25({
    ...{ class: "summary-card page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_25));
/** @type {__VLS_StyleScopedClasses['summary-card']} */ ;
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_29 } = __VLS_27.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "summary-card__label" },
});
/** @type {__VLS_StyleScopedClasses['summary-card__label']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({
    ...{ class: "summary-card__value" },
});
/** @type {__VLS_StyleScopedClasses['summary-card__value']} */ ;
(__VLS_ctx.invalidTokenCount);
// @ts-ignore
[invalidTokenCount,];
var __VLS_27;
let __VLS_30;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_31 = __VLS_asFunctionalComponent1(__VLS_30, new __VLS_30({
    ...{ class: "page-card scheduler-card" },
    shadow: "never",
}));
const __VLS_32 = __VLS_31({
    ...{ class: "page-card scheduler-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_31));
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
/** @type {__VLS_StyleScopedClasses['scheduler-card']} */ ;
const { default: __VLS_35 } = __VLS_33.slots;
{
    const { header: __VLS_36 } = __VLS_33.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "toolbar" },
    });
    /** @type {__VLS_StyleScopedClasses['toolbar']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({});
    __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
        ...{ class: "page-title" },
    });
    /** @type {__VLS_StyleScopedClasses['page-title']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({
        ...{ class: "page-subtitle" },
    });
    /** @type {__VLS_StyleScopedClasses['page-subtitle']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "toolbar__actions" },
    });
    /** @type {__VLS_StyleScopedClasses['toolbar__actions']} */ ;
    let __VLS_37;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_38 = __VLS_asFunctionalComponent1(__VLS_37, new __VLS_37({
        type: (__VLS_ctx.schedulerStatusTagType),
        effect: "light",
    }));
    const __VLS_39 = __VLS_38({
        type: (__VLS_ctx.schedulerStatusTagType),
        effect: "light",
    }, ...__VLS_functionalComponentArgsRest(__VLS_38));
    const { default: __VLS_42 } = __VLS_40.slots;
    (__VLS_ctx.schedulerStatusLabel);
    // @ts-ignore
    [schedulerStatusTagType, schedulerStatusLabel,];
    var __VLS_40;
    // @ts-ignore
    [];
}
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-panel" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-panel']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-form" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-form']} */ ;
let __VLS_43;
/** @ts-ignore @type {typeof __VLS_components.elAlert | typeof __VLS_components.ElAlert} */
elAlert;
// @ts-ignore
const __VLS_44 = __VLS_asFunctionalComponent1(__VLS_43, new __VLS_43({
    type: "info",
    showIcon: true,
    closable: (false),
    title: "定时任务会复用设置页里的 CPA API URL 和 Token，仅处理 token_invalidated / deactivated_workspace 账号。",
}));
const __VLS_45 = __VLS_44({
    type: "info",
    showIcon: true,
    closable: (false),
    title: "定时任务会复用设置页里的 CPA API URL 和 Token，仅处理 token_invalidated / deactivated_workspace 账号。",
}, ...__VLS_functionalComponentArgsRest(__VLS_44));
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-switches" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-switches']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-toggle" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-toggle']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({});
let __VLS_48;
/** @ts-ignore @type {typeof __VLS_components.elSwitch | typeof __VLS_components.ElSwitch} */
elSwitch;
// @ts-ignore
const __VLS_49 = __VLS_asFunctionalComponent1(__VLS_48, new __VLS_48({
    modelValue: (__VLS_ctx.schedulerForm.enabled),
    inlinePrompt: true,
    activeText: "开",
    inactiveText: "关",
}));
const __VLS_50 = __VLS_49({
    modelValue: (__VLS_ctx.schedulerForm.enabled),
    inlinePrompt: true,
    activeText: "开",
    inactiveText: "关",
}, ...__VLS_functionalComponentArgsRest(__VLS_49));
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-toggle" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-toggle']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({});
let __VLS_53;
/** @ts-ignore @type {typeof __VLS_components.elSwitch | typeof __VLS_components.ElSwitch} */
elSwitch;
// @ts-ignore
const __VLS_54 = __VLS_asFunctionalComponent1(__VLS_53, new __VLS_53({
    modelValue: (__VLS_ctx.schedulerForm.delete_invalid),
    inlinePrompt: true,
    activeText: "开",
    inactiveText: "关",
}));
const __VLS_55 = __VLS_54({
    modelValue: (__VLS_ctx.schedulerForm.delete_invalid),
    inlinePrompt: true,
    activeText: "开",
    inactiveText: "关",
}, ...__VLS_functionalComponentArgsRest(__VLS_54));
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-config-grid" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-config-grid']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-config-card" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-config-card']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "scheduler-config-card__label" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-config-card__label']} */ ;
let __VLS_58;
/** @ts-ignore @type {typeof __VLS_components.elRadioGroup | typeof __VLS_components.ElRadioGroup | typeof __VLS_components.elRadioGroup | typeof __VLS_components.ElRadioGroup} */
elRadioGroup;
// @ts-ignore
const __VLS_59 = __VLS_asFunctionalComponent1(__VLS_58, new __VLS_58({
    modelValue: (__VLS_ctx.schedulerForm.mode),
}));
const __VLS_60 = __VLS_59({
    modelValue: (__VLS_ctx.schedulerForm.mode),
}, ...__VLS_functionalComponentArgsRest(__VLS_59));
const { default: __VLS_63 } = __VLS_61.slots;
let __VLS_64;
/** @ts-ignore @type {typeof __VLS_components.elRadioButton | typeof __VLS_components.ElRadioButton | typeof __VLS_components.elRadioButton | typeof __VLS_components.ElRadioButton} */
elRadioButton;
// @ts-ignore
const __VLS_65 = __VLS_asFunctionalComponent1(__VLS_64, new __VLS_64({
    label: "interval",
}));
const __VLS_66 = __VLS_65({
    label: "interval",
}, ...__VLS_functionalComponentArgsRest(__VLS_65));
const { default: __VLS_69 } = __VLS_67.slots;
// @ts-ignore
[schedulerForm, schedulerForm, schedulerForm,];
var __VLS_67;
let __VLS_70;
/** @ts-ignore @type {typeof __VLS_components.elRadioButton | typeof __VLS_components.ElRadioButton | typeof __VLS_components.elRadioButton | typeof __VLS_components.ElRadioButton} */
elRadioButton;
// @ts-ignore
const __VLS_71 = __VLS_asFunctionalComponent1(__VLS_70, new __VLS_70({
    label: "fixed_times",
}));
const __VLS_72 = __VLS_71({
    label: "fixed_times",
}, ...__VLS_functionalComponentArgsRest(__VLS_71));
const { default: __VLS_75 } = __VLS_73.slots;
// @ts-ignore
[];
var __VLS_73;
// @ts-ignore
[];
var __VLS_61;
if (__VLS_ctx.schedulerForm.mode === 'interval') {
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "scheduler-config-card" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-config-card']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "scheduler-config-card__label" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-config-card__label']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "scheduler-inline-fields" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-inline-fields']} */ ;
    let __VLS_76;
    /** @ts-ignore @type {typeof __VLS_components.elInputNumber | typeof __VLS_components.ElInputNumber} */
    elInputNumber;
    // @ts-ignore
    const __VLS_77 = __VLS_asFunctionalComponent1(__VLS_76, new __VLS_76({
        modelValue: (__VLS_ctx.schedulerForm.interval_minutes),
        min: (1),
        max: (1440),
        controlsPosition: "right",
    }));
    const __VLS_78 = __VLS_77({
        modelValue: (__VLS_ctx.schedulerForm.interval_minutes),
        min: (1),
        max: (1440),
        controlsPosition: "right",
    }, ...__VLS_functionalComponentArgsRest(__VLS_77));
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "scheduler-inline-fields__suffix" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-inline-fields__suffix']} */ ;
}
else {
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "scheduler-config-card scheduler-config-card--times" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-config-card']} */ ;
    /** @type {__VLS_StyleScopedClasses['scheduler-config-card--times']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "scheduler-config-card__label" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-config-card__label']} */ ;
    let __VLS_81;
    /** @ts-ignore @type {typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect | typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect} */
    elSelect;
    // @ts-ignore
    const __VLS_82 = __VLS_asFunctionalComponent1(__VLS_81, new __VLS_81({
        modelValue: (__VLS_ctx.schedulerForm.fixed_times),
        multiple: true,
        filterable: true,
        allowCreate: true,
        defaultFirstOption: true,
        clearable: true,
        collapseTags: true,
        collapseTagsTooltip: true,
        placeholder: "选择或输入 HH:mm，例如 09:00",
    }));
    const __VLS_83 = __VLS_82({
        modelValue: (__VLS_ctx.schedulerForm.fixed_times),
        multiple: true,
        filterable: true,
        allowCreate: true,
        defaultFirstOption: true,
        clearable: true,
        collapseTags: true,
        collapseTagsTooltip: true,
        placeholder: "选择或输入 HH:mm，例如 09:00",
    }, ...__VLS_functionalComponentArgsRest(__VLS_82));
    const { default: __VLS_86 } = __VLS_84.slots;
    for (const [option] of __VLS_vFor((__VLS_ctx.schedulerTimeOptions))) {
        let __VLS_87;
        /** @ts-ignore @type {typeof __VLS_components.elOption | typeof __VLS_components.ElOption} */
        elOption;
        // @ts-ignore
        const __VLS_88 = __VLS_asFunctionalComponent1(__VLS_87, new __VLS_87({
            key: (option),
            label: (option),
            value: (option),
        }));
        const __VLS_89 = __VLS_88({
            key: (option),
            label: (option),
            value: (option),
        }, ...__VLS_functionalComponentArgsRest(__VLS_88));
        // @ts-ignore
        [schedulerForm, schedulerForm, schedulerForm, schedulerTimeOptions,];
    }
    // @ts-ignore
    [];
    var __VLS_84;
    __VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({
        ...{ class: "scheduler-config-card__hint" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-config-card__hint']} */ ;
}
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-config-card" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-config-card']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "scheduler-config-card__label" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-config-card__label']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-retry-grid" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-retry-grid']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-inline-fields" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-inline-fields']} */ ;
let __VLS_92;
/** @ts-ignore @type {typeof __VLS_components.elInputNumber | typeof __VLS_components.ElInputNumber} */
elInputNumber;
// @ts-ignore
const __VLS_93 = __VLS_asFunctionalComponent1(__VLS_92, new __VLS_92({
    modelValue: (__VLS_ctx.schedulerForm.retry_count),
    min: (0),
    max: (10),
    controlsPosition: "right",
}));
const __VLS_94 = __VLS_93({
    modelValue: (__VLS_ctx.schedulerForm.retry_count),
    min: (0),
    max: (10),
    controlsPosition: "right",
}, ...__VLS_functionalComponentArgsRest(__VLS_93));
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "scheduler-inline-fields__suffix" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-inline-fields__suffix']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-inline-fields" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-inline-fields']} */ ;
let __VLS_97;
/** @ts-ignore @type {typeof __VLS_components.elInputNumber | typeof __VLS_components.ElInputNumber} */
elInputNumber;
// @ts-ignore
const __VLS_98 = __VLS_asFunctionalComponent1(__VLS_97, new __VLS_97({
    modelValue: (__VLS_ctx.schedulerForm.retry_delay_minutes),
    min: (1),
    max: (1440),
    controlsPosition: "right",
    disabled: (__VLS_ctx.schedulerForm.retry_count === 0),
}));
const __VLS_99 = __VLS_98({
    modelValue: (__VLS_ctx.schedulerForm.retry_delay_minutes),
    min: (1),
    max: (1440),
    controlsPosition: "right",
    disabled: (__VLS_ctx.schedulerForm.retry_count === 0),
}, ...__VLS_functionalComponentArgsRest(__VLS_98));
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "scheduler-inline-fields__suffix" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-inline-fields__suffix']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-actions-row" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-actions-row']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({
    ...{ class: "scheduler-hint" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-hint']} */ ;
(__VLS_ctx.schedulerConfiguredActionsText);
(__VLS_ctx.schedulerModeDescription);
if (__VLS_ctx.isSchedulerFormDirty) {
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
}
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-actions" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-actions']} */ ;
let __VLS_102;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_103 = __VLS_asFunctionalComponent1(__VLS_102, new __VLS_102({
    ...{ 'onClick': {} },
    type: "primary",
    loading: (__VLS_ctx.schedulerSaving),
}));
const __VLS_104 = __VLS_103({
    ...{ 'onClick': {} },
    type: "primary",
    loading: (__VLS_ctx.schedulerSaving),
}, ...__VLS_functionalComponentArgsRest(__VLS_103));
let __VLS_107;
const __VLS_108 = ({ click: {} },
    { onClick: (__VLS_ctx.saveScheduler) });
const { default: __VLS_109 } = __VLS_105.slots;
// @ts-ignore
[schedulerForm, schedulerForm, schedulerForm, schedulerConfiguredActionsText, schedulerModeDescription, isSchedulerFormDirty, schedulerSaving, saveScheduler,];
var __VLS_105;
var __VLS_106;
let __VLS_110;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_111 = __VLS_asFunctionalComponent1(__VLS_110, new __VLS_110({
    ...{ 'onClick': {} },
    type: "success",
    plain: true,
    loading: (__VLS_ctx.schedulerRunning),
}));
const __VLS_112 = __VLS_111({
    ...{ 'onClick': {} },
    type: "success",
    plain: true,
    loading: (__VLS_ctx.schedulerRunning),
}, ...__VLS_functionalComponentArgsRest(__VLS_111));
let __VLS_115;
const __VLS_116 = ({ click: {} },
    { onClick: (__VLS_ctx.runSchedulerNow) });
const { default: __VLS_117 } = __VLS_113.slots;
// @ts-ignore
[schedulerRunning, runSchedulerNow,];
var __VLS_113;
var __VLS_114;
let __VLS_118;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_119 = __VLS_asFunctionalComponent1(__VLS_118, new __VLS_118({
    ...{ 'onClick': {} },
    plain: true,
    loading: (__VLS_ctx.schedulerLoading),
}));
const __VLS_120 = __VLS_119({
    ...{ 'onClick': {} },
    plain: true,
    loading: (__VLS_ctx.schedulerLoading),
}, ...__VLS_functionalComponentArgsRest(__VLS_119));
let __VLS_123;
const __VLS_124 = ({ click: {} },
    { onClick: (...[$event]) => {
            __VLS_ctx.loadSchedulerState();
            // @ts-ignore
            [schedulerLoading, loadSchedulerState,];
        } });
const { default: __VLS_125 } = __VLS_121.slots;
// @ts-ignore
[];
var __VLS_121;
var __VLS_122;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-status" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-status']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-status__header" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-status__header']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
if (__VLS_ctx.schedulerState.running) {
    let __VLS_126;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_127 = __VLS_asFunctionalComponent1(__VLS_126, new __VLS_126({
        type: "warning",
        effect: "plain",
    }));
    const __VLS_128 = __VLS_127({
        type: "warning",
        effect: "plain",
    }, ...__VLS_functionalComponentArgsRest(__VLS_127));
    const { default: __VLS_131 } = __VLS_129.slots;
    // @ts-ignore
    [schedulerState,];
    var __VLS_129;
}
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-metrics" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-metrics']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-metric" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-metric']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
(__VLS_ctx.formatDate(__VLS_ctx.schedulerState.next_run_at));
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-metric" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-metric']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
(__VLS_ctx.schedulerNextReasonLabel);
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-metric" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-metric']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
(__VLS_ctx.formatDate(__VLS_ctx.schedulerState.last_run_at));
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-metric" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-metric']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
(__VLS_ctx.schedulerRetrySummary);
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-metric" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-metric']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
(__VLS_ctx.schedulerState.last_result?.invalid_found ?? 0);
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "scheduler-metric" },
});
/** @type {__VLS_StyleScopedClasses['scheduler-metric']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
(__VLS_ctx.schedulerHandledSummary);
if (__VLS_ctx.schedulerState.last_result) {
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "scheduler-result" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-result']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "scheduler-result__actions" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-result__actions']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "scheduler-result__label" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-result__label']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "scheduler-result__tags" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-result__tags']} */ ;
    for (const [action] of __VLS_vFor((__VLS_ctx.schedulerState.last_result.actions))) {
        let __VLS_132;
        /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
        elTag;
        // @ts-ignore
        const __VLS_133 = __VLS_asFunctionalComponent1(__VLS_132, new __VLS_132({
            key: (action),
            size: "small",
            effect: "plain",
        }));
        const __VLS_134 = __VLS_133({
            key: (action),
            size: "small",
            effect: "plain",
        }, ...__VLS_functionalComponentArgsRest(__VLS_133));
        const { default: __VLS_137 } = __VLS_135.slots;
        (__VLS_ctx.schedulerActionLabel(action));
        // @ts-ignore
        [schedulerState, schedulerState, schedulerState, schedulerState, schedulerState, formatDate, formatDate, schedulerNextReasonLabel, schedulerRetrySummary, schedulerHandledSummary, schedulerActionLabel,];
        var __VLS_135;
        // @ts-ignore
        [];
    }
    __VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({
        ...{ class: "scheduler-result__status" },
    });
    /** @type {__VLS_StyleScopedClasses['scheduler-result__status']} */ ;
    (__VLS_ctx.schedulerRunStatusLabel(__VLS_ctx.schedulerState.last_result.status));
    (__VLS_ctx.schedulerState.last_result.attempt);
    (__VLS_ctx.schedulerState.last_result.max_attempts);
    if (__VLS_ctx.schedulerResultPreview.length > 0) {
        __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
            ...{ class: "scheduler-result__messages" },
        });
        /** @type {__VLS_StyleScopedClasses['scheduler-result__messages']} */ ;
        for (const [message, index] of __VLS_vFor((__VLS_ctx.schedulerResultPreview))) {
            __VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({
                key: (`${index}-${message}`),
            });
            (message);
            // @ts-ignore
            [schedulerState, schedulerState, schedulerState, schedulerRunStatusLabel, schedulerResultPreview, schedulerResultPreview,];
        }
    }
}
// @ts-ignore
[];
var __VLS_33;
let __VLS_138;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_139 = __VLS_asFunctionalComponent1(__VLS_138, new __VLS_138({
    ...{ class: "page-card" },
    shadow: "never",
}));
const __VLS_140 = __VLS_139({
    ...{ class: "page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_139));
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_143 } = __VLS_141.slots;
{
    const { header: __VLS_144 } = __VLS_141.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "toolbar" },
    });
    /** @type {__VLS_StyleScopedClasses['toolbar']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({});
    __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
        ...{ class: "page-title" },
    });
    /** @type {__VLS_StyleScopedClasses['page-title']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({
        ...{ class: "page-subtitle" },
    });
    /** @type {__VLS_StyleScopedClasses['page-subtitle']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "toolbar__actions" },
    });
    /** @type {__VLS_StyleScopedClasses['toolbar__actions']} */ ;
    let __VLS_145;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_146 = __VLS_asFunctionalComponent1(__VLS_145, new __VLS_145({
        ...{ 'onClick': {} },
        plain: true,
        loading: (__VLS_ctx.schedulerLogsLoading),
    }));
    const __VLS_147 = __VLS_146({
        ...{ 'onClick': {} },
        plain: true,
        loading: (__VLS_ctx.schedulerLogsLoading),
    }, ...__VLS_functionalComponentArgsRest(__VLS_146));
    let __VLS_150;
    const __VLS_151 = ({ click: {} },
        { onClick: (...[$event]) => {
                __VLS_ctx.loadSchedulerLogs();
                // @ts-ignore
                [schedulerLogsLoading, loadSchedulerLogs,];
            } });
    const { default: __VLS_152 } = __VLS_148.slots;
    // @ts-ignore
    [];
    var __VLS_148;
    var __VLS_149;
    // @ts-ignore
    [];
}
let __VLS_153;
/** @ts-ignore @type {typeof __VLS_components.elTable | typeof __VLS_components.ElTable | typeof __VLS_components.elTable | typeof __VLS_components.ElTable} */
elTable;
// @ts-ignore
const __VLS_154 = __VLS_asFunctionalComponent1(__VLS_153, new __VLS_153({
    data: (__VLS_ctx.schedulerLogs),
    stripe: true,
    emptyText: "暂无执行日志",
}));
const __VLS_155 = __VLS_154({
    data: (__VLS_ctx.schedulerLogs),
    stripe: true,
    emptyText: "暂无执行日志",
}, ...__VLS_functionalComponentArgsRest(__VLS_154));
__VLS_asFunctionalDirective(__VLS_directives.vLoading, {})(null, { ...__VLS_directiveBindingRestFields, value: (__VLS_ctx.schedulerLogsLoading) }, null, null);
const { default: __VLS_158 } = __VLS_156.slots;
let __VLS_159;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_160 = __VLS_asFunctionalComponent1(__VLS_159, new __VLS_159({
    label: "执行时间",
    minWidth: "180",
}));
const __VLS_161 = __VLS_160({
    label: "执行时间",
    minWidth: "180",
}, ...__VLS_functionalComponentArgsRest(__VLS_160));
const { default: __VLS_164 } = __VLS_162.slots;
{
    const { default: __VLS_165 } = __VLS_162.slots;
    const [{ row }] = __VLS_vSlot(__VLS_165);
    (__VLS_ctx.formatDate(row.created_at || row.finished_at || row.started_at));
    // @ts-ignore
    [formatDate, schedulerLogsLoading, schedulerLogs, vLoading,];
}
// @ts-ignore
[];
var __VLS_162;
let __VLS_166;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_167 = __VLS_asFunctionalComponent1(__VLS_166, new __VLS_166({
    label: "触发方式",
    width: "120",
}));
const __VLS_168 = __VLS_167({
    label: "触发方式",
    width: "120",
}, ...__VLS_functionalComponentArgsRest(__VLS_167));
const { default: __VLS_171 } = __VLS_169.slots;
{
    const { default: __VLS_172 } = __VLS_169.slots;
    const [{ row }] = __VLS_vSlot(__VLS_172);
    let __VLS_173;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_174 = __VLS_asFunctionalComponent1(__VLS_173, new __VLS_173({
        size: "small",
        effect: "plain",
    }));
    const __VLS_175 = __VLS_174({
        size: "small",
        effect: "plain",
    }, ...__VLS_functionalComponentArgsRest(__VLS_174));
    const { default: __VLS_178 } = __VLS_176.slots;
    (__VLS_ctx.schedulerTriggerLabel(row.trigger_type));
    // @ts-ignore
    [schedulerTriggerLabel,];
    var __VLS_176;
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_169;
let __VLS_179;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_180 = __VLS_asFunctionalComponent1(__VLS_179, new __VLS_179({
    label: "状态",
    width: "120",
}));
const __VLS_181 = __VLS_180({
    label: "状态",
    width: "120",
}, ...__VLS_functionalComponentArgsRest(__VLS_180));
const { default: __VLS_184 } = __VLS_182.slots;
{
    const { default: __VLS_185 } = __VLS_182.slots;
    const [{ row }] = __VLS_vSlot(__VLS_185);
    let __VLS_186;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_187 = __VLS_asFunctionalComponent1(__VLS_186, new __VLS_186({
        type: (__VLS_ctx.schedulerRunStatusTagType(row.status)),
        size: "small",
        effect: "light",
    }));
    const __VLS_188 = __VLS_187({
        type: (__VLS_ctx.schedulerRunStatusTagType(row.status)),
        size: "small",
        effect: "light",
    }, ...__VLS_functionalComponentArgsRest(__VLS_187));
    const { default: __VLS_191 } = __VLS_189.slots;
    (__VLS_ctx.schedulerRunStatusLabel(row.status));
    // @ts-ignore
    [schedulerRunStatusLabel, schedulerRunStatusTagType,];
    var __VLS_189;
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_182;
let __VLS_192;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_193 = __VLS_asFunctionalComponent1(__VLS_192, new __VLS_192({
    label: "尝试",
    width: "100",
}));
const __VLS_194 = __VLS_193({
    label: "尝试",
    width: "100",
}, ...__VLS_functionalComponentArgsRest(__VLS_193));
const { default: __VLS_197 } = __VLS_195.slots;
{
    const { default: __VLS_198 } = __VLS_195.slots;
    const [{ row }] = __VLS_vSlot(__VLS_198);
    (row.attempt);
    (row.max_attempts);
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_195;
let __VLS_199;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_200 = __VLS_asFunctionalComponent1(__VLS_199, new __VLS_199({
    label: "动作",
    minWidth: "180",
}));
const __VLS_201 = __VLS_200({
    label: "动作",
    minWidth: "180",
}, ...__VLS_functionalComponentArgsRest(__VLS_200));
const { default: __VLS_204 } = __VLS_202.slots;
{
    const { default: __VLS_205 } = __VLS_202.slots;
    const [{ row }] = __VLS_vSlot(__VLS_205);
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "log-tags" },
    });
    /** @type {__VLS_StyleScopedClasses['log-tags']} */ ;
    for (const [action] of __VLS_vFor((row.actions))) {
        let __VLS_206;
        /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
        elTag;
        // @ts-ignore
        const __VLS_207 = __VLS_asFunctionalComponent1(__VLS_206, new __VLS_206({
            key: (`${row.id}-${action}`),
            size: "small",
            effect: "plain",
        }));
        const __VLS_208 = __VLS_207({
            key: (`${row.id}-${action}`),
            size: "small",
            effect: "plain",
        }, ...__VLS_functionalComponentArgsRest(__VLS_207));
        const { default: __VLS_211 } = __VLS_209.slots;
        (__VLS_ctx.schedulerActionLabel(action));
        // @ts-ignore
        [schedulerActionLabel,];
        var __VLS_209;
        // @ts-ignore
        [];
    }
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_202;
let __VLS_212;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_213 = __VLS_asFunctionalComponent1(__VLS_212, new __VLS_212({
    label: "结果",
    minWidth: "180",
}));
const __VLS_214 = __VLS_213({
    label: "结果",
    minWidth: "180",
}, ...__VLS_functionalComponentArgsRest(__VLS_213));
const { default: __VLS_217 } = __VLS_215.slots;
{
    const { default: __VLS_218 } = __VLS_215.slots;
    const [{ row }] = __VLS_vSlot(__VLS_218);
    (row.invalid_found);
    (row.disabled_count);
    (row.deleted_count);
    (row.failed_count);
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_215;
let __VLS_219;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_220 = __VLS_asFunctionalComponent1(__VLS_219, new __VLS_219({
    label: "说明",
    minWidth: "260",
}));
const __VLS_221 = __VLS_220({
    label: "说明",
    minWidth: "260",
}, ...__VLS_functionalComponentArgsRest(__VLS_220));
const { default: __VLS_224 } = __VLS_222.slots;
{
    const { default: __VLS_225 } = __VLS_222.slots;
    const [{ row }] = __VLS_vSlot(__VLS_225);
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    (__VLS_ctx.schedulerLogMessage(row));
    // @ts-ignore
    [schedulerLogMessage,];
}
// @ts-ignore
[];
var __VLS_222;
// @ts-ignore
[];
var __VLS_156;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "pagination pagination--logs" },
});
/** @type {__VLS_StyleScopedClasses['pagination']} */ ;
/** @type {__VLS_StyleScopedClasses['pagination--logs']} */ ;
let __VLS_226;
/** @ts-ignore @type {typeof __VLS_components.elPagination | typeof __VLS_components.ElPagination} */
elPagination;
// @ts-ignore
const __VLS_227 = __VLS_asFunctionalComponent1(__VLS_226, new __VLS_226({
    ...{ 'onCurrentChange': {} },
    ...{ 'onSizeChange': {} },
    currentPage: (__VLS_ctx.schedulerLogQuery.page),
    pageSize: (__VLS_ctx.schedulerLogQuery.pageSize),
    pageSizes: ([10, 20, 50]),
    total: (__VLS_ctx.schedulerLogTotal),
    background: true,
    layout: "total, sizes, prev, pager, next",
}));
const __VLS_228 = __VLS_227({
    ...{ 'onCurrentChange': {} },
    ...{ 'onSizeChange': {} },
    currentPage: (__VLS_ctx.schedulerLogQuery.page),
    pageSize: (__VLS_ctx.schedulerLogQuery.pageSize),
    pageSizes: ([10, 20, 50]),
    total: (__VLS_ctx.schedulerLogTotal),
    background: true,
    layout: "total, sizes, prev, pager, next",
}, ...__VLS_functionalComponentArgsRest(__VLS_227));
let __VLS_231;
const __VLS_232 = ({ currentChange: {} },
    { onCurrentChange: (__VLS_ctx.handleSchedulerLogPageChange) });
const __VLS_233 = ({ sizeChange: {} },
    { onSizeChange: (__VLS_ctx.handleSchedulerLogPageSizeChange) });
var __VLS_229;
var __VLS_230;
// @ts-ignore
[schedulerLogQuery, schedulerLogQuery, schedulerLogTotal, handleSchedulerLogPageChange, handleSchedulerLogPageSizeChange,];
var __VLS_141;
let __VLS_234;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_235 = __VLS_asFunctionalComponent1(__VLS_234, new __VLS_234({
    ...{ class: "page-card" },
    shadow: "never",
}));
const __VLS_236 = __VLS_235({
    ...{ class: "page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_235));
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_239 } = __VLS_237.slots;
{
    const { header: __VLS_240 } = __VLS_237.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "toolbar" },
    });
    /** @type {__VLS_StyleScopedClasses['toolbar']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({});
    __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
        ...{ class: "page-title" },
    });
    /** @type {__VLS_StyleScopedClasses['page-title']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({
        ...{ class: "page-subtitle" },
    });
    /** @type {__VLS_StyleScopedClasses['page-subtitle']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "toolbar__actions" },
    });
    /** @type {__VLS_StyleScopedClasses['toolbar__actions']} */ ;
    let __VLS_241;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_242 = __VLS_asFunctionalComponent1(__VLS_241, new __VLS_241({
        ...{ 'onClick': {} },
        type: "danger",
        plain: true,
        disabled: (__VLS_ctx.invalidFiles.length === 0),
        loading: (__VLS_ctx.cleaningInvalid),
    }));
    const __VLS_243 = __VLS_242({
        ...{ 'onClick': {} },
        type: "danger",
        plain: true,
        disabled: (__VLS_ctx.invalidFiles.length === 0),
        loading: (__VLS_ctx.cleaningInvalid),
    }, ...__VLS_functionalComponentArgsRest(__VLS_242));
    let __VLS_246;
    const __VLS_247 = ({ click: {} },
        { onClick: (__VLS_ctx.cleanAllInvalid) });
    const { default: __VLS_248 } = __VLS_244.slots;
    (__VLS_ctx.invalidFiles.length);
    // @ts-ignore
    [invalidFiles, invalidFiles, cleaningInvalid, cleanAllInvalid,];
    var __VLS_244;
    var __VLS_245;
    let __VLS_249;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_250 = __VLS_asFunctionalComponent1(__VLS_249, new __VLS_249({
        ...{ 'onClick': {} },
        loading: (__VLS_ctx.loading),
    }));
    const __VLS_251 = __VLS_250({
        ...{ 'onClick': {} },
        loading: (__VLS_ctx.loading),
    }, ...__VLS_functionalComponentArgsRest(__VLS_250));
    let __VLS_254;
    const __VLS_255 = ({ click: {} },
        { onClick: (__VLS_ctx.loadFiles) });
    const { default: __VLS_256 } = __VLS_252.slots;
    // @ts-ignore
    [loading, loadFiles,];
    var __VLS_252;
    var __VLS_253;
    // @ts-ignore
    [];
}
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "filters" },
});
/** @type {__VLS_StyleScopedClasses['filters']} */ ;
let __VLS_257;
/** @ts-ignore @type {typeof __VLS_components.elInput | typeof __VLS_components.ElInput} */
elInput;
// @ts-ignore
const __VLS_258 = __VLS_asFunctionalComponent1(__VLS_257, new __VLS_257({
    modelValue: (__VLS_ctx.searchText),
    clearable: true,
    placeholder: "搜索邮箱 / 账号",
}));
const __VLS_259 = __VLS_258({
    modelValue: (__VLS_ctx.searchText),
    clearable: true,
    placeholder: "搜索邮箱 / 账号",
}, ...__VLS_functionalComponentArgsRest(__VLS_258));
let __VLS_262;
/** @ts-ignore @type {typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect | typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect} */
elSelect;
// @ts-ignore
const __VLS_263 = __VLS_asFunctionalComponent1(__VLS_262, new __VLS_262({
    modelValue: (__VLS_ctx.filterStatus),
    clearable: true,
    placeholder: "全部状态",
}));
const __VLS_264 = __VLS_263({
    modelValue: (__VLS_ctx.filterStatus),
    clearable: true,
    placeholder: "全部状态",
}, ...__VLS_functionalComponentArgsRest(__VLS_263));
const { default: __VLS_267 } = __VLS_265.slots;
let __VLS_268;
/** @ts-ignore @type {typeof __VLS_components.elOption | typeof __VLS_components.ElOption} */
elOption;
// @ts-ignore
const __VLS_269 = __VLS_asFunctionalComponent1(__VLS_268, new __VLS_268({
    label: "有效",
    value: "active",
}));
const __VLS_270 = __VLS_269({
    label: "有效",
    value: "active",
}, ...__VLS_functionalComponentArgsRest(__VLS_269));
let __VLS_273;
/** @ts-ignore @type {typeof __VLS_components.elOption | typeof __VLS_components.ElOption} */
elOption;
// @ts-ignore
const __VLS_274 = __VLS_asFunctionalComponent1(__VLS_273, new __VLS_273({
    label: "禁用",
    value: "disabled",
}));
const __VLS_275 = __VLS_274({
    label: "禁用",
    value: "disabled",
}, ...__VLS_functionalComponentArgsRest(__VLS_274));
let __VLS_278;
/** @ts-ignore @type {typeof __VLS_components.elOption | typeof __VLS_components.ElOption} */
elOption;
// @ts-ignore
const __VLS_279 = __VLS_asFunctionalComponent1(__VLS_278, new __VLS_278({
    label: "限额中",
    value: "usage_limited",
}));
const __VLS_280 = __VLS_279({
    label: "限额中",
    value: "usage_limited",
}, ...__VLS_functionalComponentArgsRest(__VLS_279));
let __VLS_283;
/** @ts-ignore @type {typeof __VLS_components.elOption | typeof __VLS_components.ElOption} */
elOption;
// @ts-ignore
const __VLS_284 = __VLS_asFunctionalComponent1(__VLS_283, new __VLS_283({
    label: "Token 失效",
    value: "token_invalid",
}));
const __VLS_285 = __VLS_284({
    label: "Token 失效",
    value: "token_invalid",
}, ...__VLS_functionalComponentArgsRest(__VLS_284));
// @ts-ignore
[searchText, filterStatus,];
var __VLS_265;
let __VLS_288;
/** @ts-ignore @type {typeof __VLS_components.elTable | typeof __VLS_components.ElTable | typeof __VLS_components.elTable | typeof __VLS_components.ElTable} */
elTable;
// @ts-ignore
const __VLS_289 = __VLS_asFunctionalComponent1(__VLS_288, new __VLS_288({
    data: (__VLS_ctx.filteredFiles),
    rowKey: "id",
    stripe: true,
    emptyText: "暂无线上账号数据",
}));
const __VLS_290 = __VLS_289({
    data: (__VLS_ctx.filteredFiles),
    rowKey: "id",
    stripe: true,
    emptyText: "暂无线上账号数据",
}, ...__VLS_functionalComponentArgsRest(__VLS_289));
__VLS_asFunctionalDirective(__VLS_directives.vLoading, {})(null, { ...__VLS_directiveBindingRestFields, value: (__VLS_ctx.loading) }, null, null);
const { default: __VLS_293 } = __VLS_291.slots;
let __VLS_294;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_295 = __VLS_asFunctionalComponent1(__VLS_294, new __VLS_294({
    prop: "account",
    label: "邮箱",
    minWidth: "240",
}));
const __VLS_296 = __VLS_295({
    prop: "account",
    label: "邮箱",
    minWidth: "240",
}, ...__VLS_functionalComponentArgsRest(__VLS_295));
let __VLS_299;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_300 = __VLS_asFunctionalComponent1(__VLS_299, new __VLS_299({
    label: "状态",
    width: "120",
}));
const __VLS_301 = __VLS_300({
    label: "状态",
    width: "120",
}, ...__VLS_functionalComponentArgsRest(__VLS_300));
const { default: __VLS_304 } = __VLS_302.slots;
{
    const { default: __VLS_305 } = __VLS_302.slots;
    const [{ row }] = __VLS_vSlot(__VLS_305);
    let __VLS_306;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_307 = __VLS_asFunctionalComponent1(__VLS_306, new __VLS_306({
        type: (__VLS_ctx.statusTagType(row)),
        effect: "light",
    }));
    const __VLS_308 = __VLS_307({
        type: (__VLS_ctx.statusTagType(row)),
        effect: "light",
    }, ...__VLS_functionalComponentArgsRest(__VLS_307));
    const { default: __VLS_311 } = __VLS_309.slots;
    (__VLS_ctx.statusLabel(row));
    // @ts-ignore
    [vLoading, loading, filteredFiles, statusTagType, statusLabel,];
    var __VLS_309;
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_302;
let __VLS_312;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_313 = __VLS_asFunctionalComponent1(__VLS_312, new __VLS_312({
    label: "限额状态",
    width: "120",
}));
const __VLS_314 = __VLS_313({
    label: "限额状态",
    width: "120",
}, ...__VLS_functionalComponentArgsRest(__VLS_313));
const { default: __VLS_317 } = __VLS_315.slots;
{
    const { default: __VLS_318 } = __VLS_315.slots;
    const [{ row }] = __VLS_vSlot(__VLS_318);
    if (__VLS_ctx.usageLimitState(row) === 'limited') {
        let __VLS_319;
        /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
        elTag;
        // @ts-ignore
        const __VLS_320 = __VLS_asFunctionalComponent1(__VLS_319, new __VLS_319({
            type: "warning",
            effect: "plain",
        }));
        const __VLS_321 = __VLS_320({
            type: "warning",
            effect: "plain",
        }, ...__VLS_functionalComponentArgsRest(__VLS_320));
        const { default: __VLS_324 } = __VLS_322.slots;
        // @ts-ignore
        [usageLimitState,];
        var __VLS_322;
    }
    else if (__VLS_ctx.usageLimitState(row) === 'recoverable') {
        let __VLS_325;
        /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
        elTag;
        // @ts-ignore
        const __VLS_326 = __VLS_asFunctionalComponent1(__VLS_325, new __VLS_325({
            type: "info",
            effect: "plain",
        }));
        const __VLS_327 = __VLS_326({
            type: "info",
            effect: "plain",
        }, ...__VLS_functionalComponentArgsRest(__VLS_326));
        const { default: __VLS_330 } = __VLS_328.slots;
        // @ts-ignore
        [usageLimitState,];
        var __VLS_328;
    }
    else if (__VLS_ctx.usageLimitState(row) === 'recovered') {
        let __VLS_331;
        /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
        elTag;
        // @ts-ignore
        const __VLS_332 = __VLS_asFunctionalComponent1(__VLS_331, new __VLS_331({
            type: "success",
            effect: "plain",
        }));
        const __VLS_333 = __VLS_332({
            type: "success",
            effect: "plain",
        }, ...__VLS_functionalComponentArgsRest(__VLS_332));
        const { default: __VLS_336 } = __VLS_334.slots;
        // @ts-ignore
        [usageLimitState,];
        var __VLS_334;
    }
    else {
        __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    }
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_315;
let __VLS_337;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_338 = __VLS_asFunctionalComponent1(__VLS_337, new __VLS_337({
    label: "Token 状态",
    width: "140",
}));
const __VLS_339 = __VLS_338({
    label: "Token 状态",
    width: "140",
}, ...__VLS_functionalComponentArgsRest(__VLS_338));
const { default: __VLS_342 } = __VLS_340.slots;
{
    const { default: __VLS_343 } = __VLS_340.slots;
    const [{ row }] = __VLS_vSlot(__VLS_343);
    if (__VLS_ctx.isTokenInvalid(row)) {
        let __VLS_344;
        /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
        elTag;
        // @ts-ignore
        const __VLS_345 = __VLS_asFunctionalComponent1(__VLS_344, new __VLS_344({
            type: "danger",
            effect: "plain",
        }));
        const __VLS_346 = __VLS_345({
            type: "danger",
            effect: "plain",
        }, ...__VLS_functionalComponentArgsRest(__VLS_345));
        const { default: __VLS_349 } = __VLS_347.slots;
        // @ts-ignore
        [isTokenInvalid,];
        var __VLS_347;
    }
    else {
        let __VLS_350;
        /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
        elTag;
        // @ts-ignore
        const __VLS_351 = __VLS_asFunctionalComponent1(__VLS_350, new __VLS_350({
            type: "success",
            effect: "plain",
        }));
        const __VLS_352 = __VLS_351({
            type: "success",
            effect: "plain",
        }, ...__VLS_functionalComponentArgsRest(__VLS_351));
        const { default: __VLS_355 } = __VLS_353.slots;
        // @ts-ignore
        [];
        var __VLS_353;
    }
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_340;
let __VLS_356;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_357 = __VLS_asFunctionalComponent1(__VLS_356, new __VLS_356({
    label: "套餐",
    width: "100",
}));
const __VLS_358 = __VLS_357({
    label: "套餐",
    width: "100",
}, ...__VLS_functionalComponentArgsRest(__VLS_357));
const { default: __VLS_361 } = __VLS_359.slots;
{
    const { default: __VLS_362 } = __VLS_359.slots;
    const [{ row }] = __VLS_vSlot(__VLS_362);
    (row.id_token?.plan_type || '-');
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_359;
let __VLS_363;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_364 = __VLS_asFunctionalComponent1(__VLS_363, new __VLS_363({
    label: "恢复时间",
    minWidth: "180",
}));
const __VLS_365 = __VLS_364({
    label: "恢复时间",
    minWidth: "180",
}, ...__VLS_functionalComponentArgsRest(__VLS_364));
const { default: __VLS_368 } = __VLS_366.slots;
{
    const { default: __VLS_369 } = __VLS_366.slots;
    const [{ row }] = __VLS_vSlot(__VLS_369);
    (__VLS_ctx.formatRecoveryTime(row));
    // @ts-ignore
    [formatRecoveryTime,];
}
// @ts-ignore
[];
var __VLS_366;
let __VLS_370;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_371 = __VLS_asFunctionalComponent1(__VLS_370, new __VLS_370({
    label: "创建时间",
    minWidth: "180",
}));
const __VLS_372 = __VLS_371({
    label: "创建时间",
    minWidth: "180",
}, ...__VLS_functionalComponentArgsRest(__VLS_371));
const { default: __VLS_375 } = __VLS_373.slots;
{
    const { default: __VLS_376 } = __VLS_373.slots;
    const [{ row }] = __VLS_vSlot(__VLS_376);
    (__VLS_ctx.formatDate(row.created_at));
    // @ts-ignore
    [formatDate,];
}
// @ts-ignore
[];
var __VLS_373;
let __VLS_377;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_378 = __VLS_asFunctionalComponent1(__VLS_377, new __VLS_377({
    label: "最后刷新",
    minWidth: "180",
}));
const __VLS_379 = __VLS_378({
    label: "最后刷新",
    minWidth: "180",
}, ...__VLS_functionalComponentArgsRest(__VLS_378));
const { default: __VLS_382 } = __VLS_380.slots;
{
    const { default: __VLS_383 } = __VLS_380.slots;
    const [{ row }] = __VLS_vSlot(__VLS_383);
    (__VLS_ctx.formatDate(row.last_refresh));
    // @ts-ignore
    [formatDate,];
}
// @ts-ignore
[];
var __VLS_380;
let __VLS_384;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_385 = __VLS_asFunctionalComponent1(__VLS_384, new __VLS_384({
    label: "操作",
    width: "120",
    fixed: "right",
}));
const __VLS_386 = __VLS_385({
    label: "操作",
    width: "120",
    fixed: "right",
}, ...__VLS_functionalComponentArgsRest(__VLS_385));
const { default: __VLS_389 } = __VLS_387.slots;
{
    const { default: __VLS_390 } = __VLS_387.slots;
    const [{ row }] = __VLS_vSlot(__VLS_390);
    let __VLS_391;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_392 = __VLS_asFunctionalComponent1(__VLS_391, new __VLS_391({
        ...{ 'onClick': {} },
        link: true,
        type: "danger",
        loading: (__VLS_ctx.deletingId === row.id),
    }));
    const __VLS_393 = __VLS_392({
        ...{ 'onClick': {} },
        link: true,
        type: "danger",
        loading: (__VLS_ctx.deletingId === row.id),
    }, ...__VLS_functionalComponentArgsRest(__VLS_392));
    let __VLS_396;
    const __VLS_397 = ({ click: {} },
        { onClick: (...[$event]) => {
                __VLS_ctx.deleteFile(row);
                // @ts-ignore
                [deletingId, deleteFile,];
            } });
    const { default: __VLS_398 } = __VLS_394.slots;
    // @ts-ignore
    [];
    var __VLS_394;
    var __VLS_395;
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_387;
// @ts-ignore
[];
var __VLS_291;
// @ts-ignore
[];
var __VLS_237;
// @ts-ignore
[];
const __VLS_export = (await import('vue')).defineComponent({});
export default {};
