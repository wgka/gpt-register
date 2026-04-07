import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
const rows = ref([]);
const total = ref(0);
const loading = ref(false);
const detailLoading = ref(false);
const detailVisible = ref(false);
const linkRegenerating = ref(false);
const selectedAccount = ref(null);
const selectedTokens = ref({});
const selectedIds = ref([]);
const batchAction = ref('');
const actionLoading = reactive({});
const stats = reactive({
    total: 0,
    by_status: {},
    by_email_service: {},
});
const filters = reactive({
    search: '',
    status: '',
    emailService: '',
    refreshTokenStatus: '',
    page: 1,
    pageSize: 10,
});
const statusOptions = [
    { label: '有效', value: 'active' },
    { label: '过期', value: 'expired' },
    { label: '封禁', value: 'banned' },
    { label: '失败', value: 'failed' },
];
const serviceOptions = [{ label: '临时邮箱', value: 'tempmail' }];
const refreshTokenOptions = [
    { label: '有 Refresh Token', value: 'has' },
    { label: '无 Refresh Token', value: 'none' },
];
async function refreshAll() {
    loading.value = true;
    try {
        await Promise.all([loadStats(), loadAccounts()]);
    }
    catch {
        ElMessage.error('加载账号数据失败');
    }
    finally {
        loading.value = false;
    }
}
async function loadAccounts() {
    const params = new URLSearchParams({
        page: String(filters.page),
        page_size: String(filters.pageSize),
    });
    if (filters.search) {
        params.set('search', filters.search);
    }
    if (filters.status) {
        params.set('status', filters.status);
    }
    if (filters.emailService) {
        params.set('email_service', filters.emailService);
    }
    if (filters.refreshTokenStatus) {
        params.set('refresh_token_status', filters.refreshTokenStatus);
    }
    const response = await fetch(`/api/accounts?${params.toString()}`);
    if (!response.ok) {
        throw new Error('load accounts failed');
    }
    const payload = (await response.json());
    rows.value = payload.accounts ?? [];
    total.value = payload.total ?? 0;
}
async function loadStats() {
    const response = await fetch('/api/accounts/stats/summary');
    if (!response.ok) {
        throw new Error('load account stats failed');
    }
    const payload = (await response.json());
    stats.total = payload.total ?? 0;
    stats.by_status = payload.by_status ?? {};
    stats.by_email_service = payload.by_email_service ?? {};
}
async function openDetail(id) {
    detailVisible.value = true;
    detailLoading.value = true;
    selectedAccount.value = null;
    selectedTokens.value = {};
    try {
        const [accountResponse, tokenResponse] = await Promise.all([fetch(`/api/accounts/${id}`), fetchAccountTokens(id)]);
        if (!accountResponse.ok || !tokenResponse.ok) {
            throw new Error('load account detail failed');
        }
        selectedAccount.value = (await accountResponse.json());
        selectedTokens.value = (await tokenResponse.json());
    }
    catch {
        selectedAccount.value = null;
        selectedTokens.value = {};
        ElMessage.error('加载账号详情失败');
    }
    finally {
        detailLoading.value = false;
    }
}
async function fetchAccountTokens(id) {
    return fetch(`/api/accounts/${id}/tokens`);
}
async function regenerateBindCardLinks() {
    const accountID = selectedAccount.value?.id;
    if (!accountID) {
        return;
    }
    linkRegenerating.value = true;
    try {
        const response = await fetch(`/api/accounts/${accountID}/tokens/regenerate-links`, {
            method: 'POST',
        });
        if (!response.ok) {
            const payload = (await response.json().catch(() => ({})));
            throw new Error(payload.error || 'regenerate bind card links failed');
        }
        selectedTokens.value = (await response.json());
        ElMessage.success('绑卡链接已重新生成');
    }
    catch (error) {
        const message = error instanceof Error ? error.message : '重新生成绑卡链接失败';
        ElMessage.error(message);
    }
    finally {
        linkRegenerating.value = false;
    }
}
async function runRowAction(account, action, reloadDetail = false) {
    actionLoading[account.id] = action;
    try {
        const response = await fetch(`/api/accounts/${account.id}/${action}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({}),
        });
        if (!response.ok) {
            throw new Error('request failed');
        }
        const payload = (await response.json());
        if (action === 'validate') {
            if (payload.valid) {
                ElMessage.success(`账号 ${account.email} Token 有效`);
            }
            else if (payload.deleted) {
                ElMessage.warning(payload.error || `账号 ${account.email} Token 无效，已删除`);
            }
            else {
                ElMessage.warning(payload.error || `账号 ${account.email} Token 无效`);
            }
        }
        else if (payload.success) {
            ElMessage.success(payload.message || actionSuccessText(action));
        }
        else {
            ElMessage.error(payload.error || `${actionSuccessText(action)}失败`);
        }
        await refreshAll();
        if (reloadDetail && selectedAccount.value?.id === account.id) {
            if (action === 'validate' && payload.deleted) {
                detailVisible.value = false;
                selectedAccount.value = null;
                selectedTokens.value = {};
                return;
            }
            await openDetail(account.id);
        }
    }
    catch {
        ElMessage.error(`${actionSuccessText(action)}失败`);
    }
    finally {
        actionLoading[account.id] = '';
    }
}
async function runCodexReauthorize(account, reloadDetail = false) {
    try {
        await ElMessageBox.confirm(`将使用保存的密码为 ${account.email} 手动执行 Codex/CLI 授权，并在成功后自动上报 CPA，是否继续？`, 'Codex/CLI 授权确认', { type: 'warning' });
    }
    catch {
        return;
    }
    actionLoading[account.id] = 'reauthorize-codex';
    try {
        const response = await fetch(`/api/accounts/${account.id}/reauthorize-codex`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ upload_cpa: true }),
        });
        if (!response.ok) {
            throw new Error('request failed');
        }
        const payload = (await response.json());
        if (payload.success) {
            ElMessage.success(payload.message || 'Codex/CLI 授权已更新');
        }
        else if (payload.auth_updated) {
            ElMessage.warning(payload.error || 'Codex/CLI 授权已更新，但 CPA 上报失败');
        }
        else {
            ElMessage.error(payload.error || 'Codex/CLI 授权失败');
        }
        await refreshAll();
        if (reloadDetail && selectedAccount.value?.id === account.id) {
            await openDetail(account.id);
        }
    }
    catch {
        ElMessage.error('Codex/CLI 授权失败');
    }
    finally {
        actionLoading[account.id] = '';
    }
}
async function runBatchAction(action) {
    if (selectedIds.value.length === 0) {
        return;
    }
    try {
        await ElMessageBox.confirm(`确定对选中的 ${selectedIds.value.length} 个账号执行${actionConfirmText(action)}吗？`, '批量操作确认', { type: 'warning' });
    }
    catch {
        return;
    }
    batchAction.value = action;
    try {
        const response = await fetch(`/api/accounts/batch-${action}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ ids: selectedIds.value }),
        });
        if (!response.ok) {
            throw new Error('request failed');
        }
        const payload = (await response.json());
        ElMessage.success(buildBatchMessage(action, payload));
        await refreshAll();
    }
    catch {
        ElMessage.error(`${actionConfirmText(action)}失败`);
    }
    finally {
        batchAction.value = '';
    }
}
async function copyValue(value, label) {
    const trimmed = value?.trim();
    if (!trimmed) {
        ElMessage.warning(`${label} 不可复制`);
        return;
    }
    try {
        await writeClipboard(trimmed);
        ElMessage.success(`${label} 已复制`);
    }
    catch {
        ElMessage.error(`${label} 复制失败`);
    }
}
async function writeClipboard(value) {
    if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(value);
        return;
    }
    const textarea = document.createElement('textarea');
    textarea.value = value;
    textarea.setAttribute('readonly', 'true');
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand('copy');
    document.body.removeChild(textarea);
}
function applyFilters() {
    filters.page = 1;
    refreshAll();
}
function resetFilters() {
    filters.search = '';
    filters.status = '';
    filters.emailService = '';
    filters.refreshTokenStatus = '';
    filters.page = 1;
    refreshAll();
}
function handlePageChange(page) {
    filters.page = page;
    refreshAll();
}
function handlePageSizeChange(pageSize) {
    filters.pageSize = pageSize;
    filters.page = 1;
    refreshAll();
}
function handleSelectionChange(selection) {
    selectedIds.value = selection.map((item) => item.id);
}
function formatDate(value) {
    if (!value) {
        return '-';
    }
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) {
        return value;
    }
    return parsed.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
    });
}
function statusTagType(status) {
    switch (status) {
        case 'active':
            return 'success';
        case 'expired':
            return 'warning';
        case 'banned':
            return 'danger';
        case 'failed':
            return 'info';
        default:
            return 'info';
    }
}
function serviceLabel(service) {
    if (service === 'tempmail' || service === 'temp-email' || service === 'meteormail') {
        return '临时邮箱';
    }
    return service || '-';
}
function actionSuccessText(action) {
    switch (action) {
        case 'refresh':
            return '刷新 Token';
        case 'validate':
            return '校验 Token';
        case 'upload-cpa':
            return '上传 CPA';
    }
}
function actionConfirmText(action) {
    switch (action) {
        case 'refresh':
            return '刷新 Token';
        case 'validate':
            return '校验 Token';
        case 'upload-cpa':
            return '上传到 CPA';
    }
}
function buildBatchMessage(action, payload) {
    if (action === 'refresh') {
        return `刷新完成，成功 ${Number(payload.success_count ?? 0)}，失败 ${Number(payload.failed_count ?? 0)}`;
    }
    if (action === 'validate') {
        return `校验完成，有效 ${Number(payload.valid_count ?? 0)}，无效 ${Number(payload.invalid_count ?? 0)}，已删除 ${Number(payload.deleted_count ?? 0)}`;
    }
    return `上传完成，成功 ${Number(payload.success_count ?? 0)}，失败 ${Number(payload.failed_count ?? 0)}，跳过 ${Number(payload.skipped_count ?? 0)}`;
}
onMounted(() => {
    refreshAll();
});
const __VLS_ctx = {
    ...{},
    ...{},
};
let __VLS_components;
let __VLS_intrinsics;
let __VLS_directives;
/** @type {__VLS_StyleScopedClasses['token-card']} */ ;
/** @type {__VLS_StyleScopedClasses['summary-grid']} */ ;
/** @type {__VLS_StyleScopedClasses['toolbar']} */ ;
/** @type {__VLS_StyleScopedClasses['filters']} */ ;
/** @type {__VLS_StyleScopedClasses['pagination']} */ ;
/** @type {__VLS_StyleScopedClasses['detail-grid']} */ ;
/** @type {__VLS_StyleScopedClasses['summary-grid']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "accounts-page" },
});
/** @type {__VLS_StyleScopedClasses['accounts-page']} */ ;
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
(__VLS_ctx.stats.total);
// @ts-ignore
[stats,];
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
(__VLS_ctx.stats.by_status.active ?? 0);
// @ts-ignore
[stats,];
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
(__VLS_ctx.stats.by_status.expired ?? 0);
// @ts-ignore
[stats,];
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
(__VLS_ctx.stats.by_status.failed ?? 0);
// @ts-ignore
[stats,];
var __VLS_21;
let __VLS_24;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_25 = __VLS_asFunctionalComponent1(__VLS_24, new __VLS_24({
    ...{ class: "page-card" },
    shadow: "never",
}));
const __VLS_26 = __VLS_25({
    ...{ class: "page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_25));
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_29 } = __VLS_27.slots;
{
    const { header: __VLS_30 } = __VLS_27.slots;
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
    let __VLS_31;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_32 = __VLS_asFunctionalComponent1(__VLS_31, new __VLS_31({
        ...{ 'onClick': {} },
        loading: (__VLS_ctx.loading),
    }));
    const __VLS_33 = __VLS_32({
        ...{ 'onClick': {} },
        loading: (__VLS_ctx.loading),
    }, ...__VLS_functionalComponentArgsRest(__VLS_32));
    let __VLS_36;
    const __VLS_37 = ({ click: {} },
        { onClick: (__VLS_ctx.refreshAll) });
    const { default: __VLS_38 } = __VLS_34.slots;
    // @ts-ignore
    [loading, refreshAll,];
    var __VLS_34;
    var __VLS_35;
    // @ts-ignore
    [];
}
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "filters" },
});
/** @type {__VLS_StyleScopedClasses['filters']} */ ;
let __VLS_39;
/** @ts-ignore @type {typeof __VLS_components.elInput | typeof __VLS_components.ElInput} */
elInput;
// @ts-ignore
const __VLS_40 = __VLS_asFunctionalComponent1(__VLS_39, new __VLS_39({
    ...{ 'onKeyup': {} },
    modelValue: (__VLS_ctx.filters.search),
    clearable: true,
    placeholder: "搜索邮箱 / 账号 ID / 工作区 ID",
}));
const __VLS_41 = __VLS_40({
    ...{ 'onKeyup': {} },
    modelValue: (__VLS_ctx.filters.search),
    clearable: true,
    placeholder: "搜索邮箱 / 账号 ID / 工作区 ID",
}, ...__VLS_functionalComponentArgsRest(__VLS_40));
let __VLS_44;
const __VLS_45 = ({ keyup: {} },
    { onKeyup: (__VLS_ctx.applyFilters) });
var __VLS_42;
var __VLS_43;
let __VLS_46;
/** @ts-ignore @type {typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect | typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect} */
elSelect;
// @ts-ignore
const __VLS_47 = __VLS_asFunctionalComponent1(__VLS_46, new __VLS_46({
    modelValue: (__VLS_ctx.filters.status),
    clearable: true,
    placeholder: "全部状态",
}));
const __VLS_48 = __VLS_47({
    modelValue: (__VLS_ctx.filters.status),
    clearable: true,
    placeholder: "全部状态",
}, ...__VLS_functionalComponentArgsRest(__VLS_47));
const { default: __VLS_51 } = __VLS_49.slots;
for (const [option] of __VLS_vFor((__VLS_ctx.statusOptions))) {
    let __VLS_52;
    /** @ts-ignore @type {typeof __VLS_components.elOption | typeof __VLS_components.ElOption} */
    elOption;
    // @ts-ignore
    const __VLS_53 = __VLS_asFunctionalComponent1(__VLS_52, new __VLS_52({
        key: (option.value),
        label: (option.label),
        value: (option.value),
    }));
    const __VLS_54 = __VLS_53({
        key: (option.value),
        label: (option.label),
        value: (option.value),
    }, ...__VLS_functionalComponentArgsRest(__VLS_53));
    // @ts-ignore
    [filters, filters, applyFilters, statusOptions,];
}
// @ts-ignore
[];
var __VLS_49;
let __VLS_57;
/** @ts-ignore @type {typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect | typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect} */
elSelect;
// @ts-ignore
const __VLS_58 = __VLS_asFunctionalComponent1(__VLS_57, new __VLS_57({
    modelValue: (__VLS_ctx.filters.emailService),
    clearable: true,
    placeholder: "全部邮箱服务",
}));
const __VLS_59 = __VLS_58({
    modelValue: (__VLS_ctx.filters.emailService),
    clearable: true,
    placeholder: "全部邮箱服务",
}, ...__VLS_functionalComponentArgsRest(__VLS_58));
const { default: __VLS_62 } = __VLS_60.slots;
for (const [option] of __VLS_vFor((__VLS_ctx.serviceOptions))) {
    let __VLS_63;
    /** @ts-ignore @type {typeof __VLS_components.elOption | typeof __VLS_components.ElOption} */
    elOption;
    // @ts-ignore
    const __VLS_64 = __VLS_asFunctionalComponent1(__VLS_63, new __VLS_63({
        key: (option.value),
        label: (option.label),
        value: (option.value),
    }));
    const __VLS_65 = __VLS_64({
        key: (option.value),
        label: (option.label),
        value: (option.value),
    }, ...__VLS_functionalComponentArgsRest(__VLS_64));
    // @ts-ignore
    [filters, serviceOptions,];
}
// @ts-ignore
[];
var __VLS_60;
let __VLS_68;
/** @ts-ignore @type {typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect | typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect} */
elSelect;
// @ts-ignore
const __VLS_69 = __VLS_asFunctionalComponent1(__VLS_68, new __VLS_68({
    modelValue: (__VLS_ctx.filters.refreshTokenStatus),
    clearable: true,
    placeholder: "Refresh Token",
}));
const __VLS_70 = __VLS_69({
    modelValue: (__VLS_ctx.filters.refreshTokenStatus),
    clearable: true,
    placeholder: "Refresh Token",
}, ...__VLS_functionalComponentArgsRest(__VLS_69));
const { default: __VLS_73 } = __VLS_71.slots;
for (const [option] of __VLS_vFor((__VLS_ctx.refreshTokenOptions))) {
    let __VLS_74;
    /** @ts-ignore @type {typeof __VLS_components.elOption | typeof __VLS_components.ElOption} */
    elOption;
    // @ts-ignore
    const __VLS_75 = __VLS_asFunctionalComponent1(__VLS_74, new __VLS_74({
        key: (option.value),
        label: (option.label),
        value: (option.value),
    }));
    const __VLS_76 = __VLS_75({
        key: (option.value),
        label: (option.label),
        value: (option.value),
    }, ...__VLS_functionalComponentArgsRest(__VLS_75));
    // @ts-ignore
    [filters, refreshTokenOptions,];
}
// @ts-ignore
[];
var __VLS_71;
let __VLS_79;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_80 = __VLS_asFunctionalComponent1(__VLS_79, new __VLS_79({
    ...{ 'onClick': {} },
    type: "primary",
}));
const __VLS_81 = __VLS_80({
    ...{ 'onClick': {} },
    type: "primary",
}, ...__VLS_functionalComponentArgsRest(__VLS_80));
let __VLS_84;
const __VLS_85 = ({ click: {} },
    { onClick: (__VLS_ctx.applyFilters) });
const { default: __VLS_86 } = __VLS_82.slots;
// @ts-ignore
[applyFilters,];
var __VLS_82;
var __VLS_83;
let __VLS_87;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_88 = __VLS_asFunctionalComponent1(__VLS_87, new __VLS_87({
    ...{ 'onClick': {} },
}));
const __VLS_89 = __VLS_88({
    ...{ 'onClick': {} },
}, ...__VLS_functionalComponentArgsRest(__VLS_88));
let __VLS_92;
const __VLS_93 = ({ click: {} },
    { onClick: (__VLS_ctx.resetFilters) });
const { default: __VLS_94 } = __VLS_90.slots;
// @ts-ignore
[resetFilters,];
var __VLS_90;
var __VLS_91;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "batch-actions" },
});
/** @type {__VLS_StyleScopedClasses['batch-actions']} */ ;
let __VLS_95;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_96 = __VLS_asFunctionalComponent1(__VLS_95, new __VLS_95({
    ...{ 'onClick': {} },
    type: "primary",
    plain: true,
    disabled: (__VLS_ctx.selectedIds.length === 0),
    loading: (__VLS_ctx.batchAction === 'refresh'),
}));
const __VLS_97 = __VLS_96({
    ...{ 'onClick': {} },
    type: "primary",
    plain: true,
    disabled: (__VLS_ctx.selectedIds.length === 0),
    loading: (__VLS_ctx.batchAction === 'refresh'),
}, ...__VLS_functionalComponentArgsRest(__VLS_96));
let __VLS_100;
const __VLS_101 = ({ click: {} },
    { onClick: (...[$event]) => {
            __VLS_ctx.runBatchAction('refresh');
            // @ts-ignore
            [selectedIds, batchAction, runBatchAction,];
        } });
const { default: __VLS_102 } = __VLS_98.slots;
// @ts-ignore
[];
var __VLS_98;
var __VLS_99;
let __VLS_103;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_104 = __VLS_asFunctionalComponent1(__VLS_103, new __VLS_103({
    ...{ 'onClick': {} },
    type: "success",
    plain: true,
    disabled: (__VLS_ctx.selectedIds.length === 0),
    loading: (__VLS_ctx.batchAction === 'validate'),
}));
const __VLS_105 = __VLS_104({
    ...{ 'onClick': {} },
    type: "success",
    plain: true,
    disabled: (__VLS_ctx.selectedIds.length === 0),
    loading: (__VLS_ctx.batchAction === 'validate'),
}, ...__VLS_functionalComponentArgsRest(__VLS_104));
let __VLS_108;
const __VLS_109 = ({ click: {} },
    { onClick: (...[$event]) => {
            __VLS_ctx.runBatchAction('validate');
            // @ts-ignore
            [selectedIds, batchAction, runBatchAction,];
        } });
const { default: __VLS_110 } = __VLS_106.slots;
// @ts-ignore
[];
var __VLS_106;
var __VLS_107;
let __VLS_111;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_112 = __VLS_asFunctionalComponent1(__VLS_111, new __VLS_111({
    ...{ 'onClick': {} },
    type: "warning",
    plain: true,
    disabled: (__VLS_ctx.selectedIds.length === 0),
    loading: (__VLS_ctx.batchAction === 'upload-cpa'),
}));
const __VLS_113 = __VLS_112({
    ...{ 'onClick': {} },
    type: "warning",
    plain: true,
    disabled: (__VLS_ctx.selectedIds.length === 0),
    loading: (__VLS_ctx.batchAction === 'upload-cpa'),
}, ...__VLS_functionalComponentArgsRest(__VLS_112));
let __VLS_116;
const __VLS_117 = ({ click: {} },
    { onClick: (...[$event]) => {
            __VLS_ctx.runBatchAction('upload-cpa');
            // @ts-ignore
            [selectedIds, batchAction, runBatchAction,];
        } });
const { default: __VLS_118 } = __VLS_114.slots;
// @ts-ignore
[];
var __VLS_114;
var __VLS_115;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "batch-actions__hint" },
});
/** @type {__VLS_StyleScopedClasses['batch-actions__hint']} */ ;
(__VLS_ctx.selectedIds.length);
let __VLS_119;
/** @ts-ignore @type {typeof __VLS_components.elTable | typeof __VLS_components.ElTable | typeof __VLS_components.elTable | typeof __VLS_components.ElTable} */
elTable;
// @ts-ignore
const __VLS_120 = __VLS_asFunctionalComponent1(__VLS_119, new __VLS_119({
    ...{ 'onSelectionChange': {} },
    data: (__VLS_ctx.rows),
    rowKey: "id",
    stripe: true,
    emptyText: "暂无账号数据",
}));
const __VLS_121 = __VLS_120({
    ...{ 'onSelectionChange': {} },
    data: (__VLS_ctx.rows),
    rowKey: "id",
    stripe: true,
    emptyText: "暂无账号数据",
}, ...__VLS_functionalComponentArgsRest(__VLS_120));
let __VLS_124;
const __VLS_125 = ({ selectionChange: {} },
    { onSelectionChange: (__VLS_ctx.handleSelectionChange) });
__VLS_asFunctionalDirective(__VLS_directives.vLoading, {})(null, { ...__VLS_directiveBindingRestFields, value: (__VLS_ctx.loading) }, null, null);
const { default: __VLS_126 } = __VLS_122.slots;
let __VLS_127;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_128 = __VLS_asFunctionalComponent1(__VLS_127, new __VLS_127({
    type: "selection",
    width: "48",
    reserveSelection: true,
}));
const __VLS_129 = __VLS_128({
    type: "selection",
    width: "48",
    reserveSelection: true,
}, ...__VLS_functionalComponentArgsRest(__VLS_128));
let __VLS_132;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_133 = __VLS_asFunctionalComponent1(__VLS_132, new __VLS_132({
    prop: "email",
    label: "邮箱",
    minWidth: "240",
}));
const __VLS_134 = __VLS_133({
    prop: "email",
    label: "邮箱",
    minWidth: "240",
}, ...__VLS_functionalComponentArgsRest(__VLS_133));
let __VLS_137;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_138 = __VLS_asFunctionalComponent1(__VLS_137, new __VLS_137({
    label: "邮箱服务",
    minWidth: "140",
}));
const __VLS_139 = __VLS_138({
    label: "邮箱服务",
    minWidth: "140",
}, ...__VLS_functionalComponentArgsRest(__VLS_138));
const { default: __VLS_142 } = __VLS_140.slots;
{
    const { default: __VLS_143 } = __VLS_140.slots;
    const [{ row }] = __VLS_vSlot(__VLS_143);
    (__VLS_ctx.serviceLabel(row.email_service));
    // @ts-ignore
    [loading, selectedIds, rows, handleSelectionChange, vLoading, serviceLabel,];
}
// @ts-ignore
[];
var __VLS_140;
let __VLS_144;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_145 = __VLS_asFunctionalComponent1(__VLS_144, new __VLS_144({
    label: "状态",
    width: "120",
}));
const __VLS_146 = __VLS_145({
    label: "状态",
    width: "120",
}, ...__VLS_functionalComponentArgsRest(__VLS_145));
const { default: __VLS_149 } = __VLS_147.slots;
{
    const { default: __VLS_150 } = __VLS_147.slots;
    const [{ row }] = __VLS_vSlot(__VLS_150);
    let __VLS_151;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_152 = __VLS_asFunctionalComponent1(__VLS_151, new __VLS_151({
        type: (__VLS_ctx.statusTagType(row.status)),
        effect: "light",
    }));
    const __VLS_153 = __VLS_152({
        type: (__VLS_ctx.statusTagType(row.status)),
        effect: "light",
    }, ...__VLS_functionalComponentArgsRest(__VLS_152));
    const { default: __VLS_156 } = __VLS_154.slots;
    (row.status);
    // @ts-ignore
    [statusTagType,];
    var __VLS_154;
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_147;
let __VLS_157;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_158 = __VLS_asFunctionalComponent1(__VLS_157, new __VLS_157({
    label: "注册时间",
    minWidth: "180",
}));
const __VLS_159 = __VLS_158({
    label: "注册时间",
    minWidth: "180",
}, ...__VLS_functionalComponentArgsRest(__VLS_158));
const { default: __VLS_162 } = __VLS_160.slots;
{
    const { default: __VLS_163 } = __VLS_160.slots;
    const [{ row }] = __VLS_vSlot(__VLS_163);
    (__VLS_ctx.formatDate(row.registered_at));
    // @ts-ignore
    [formatDate,];
}
// @ts-ignore
[];
var __VLS_160;
let __VLS_164;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_165 = __VLS_asFunctionalComponent1(__VLS_164, new __VLS_164({
    label: "过期时间",
    minWidth: "180",
}));
const __VLS_166 = __VLS_165({
    label: "过期时间",
    minWidth: "180",
}, ...__VLS_functionalComponentArgsRest(__VLS_165));
const { default: __VLS_169 } = __VLS_167.slots;
{
    const { default: __VLS_170 } = __VLS_167.slots;
    const [{ row }] = __VLS_vSlot(__VLS_170);
    (__VLS_ctx.formatDate(row.expires_at));
    // @ts-ignore
    [formatDate,];
}
// @ts-ignore
[];
var __VLS_167;
let __VLS_171;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_172 = __VLS_asFunctionalComponent1(__VLS_171, new __VLS_171({
    label: "CPA",
    width: "100",
}));
const __VLS_173 = __VLS_172({
    label: "CPA",
    width: "100",
}, ...__VLS_functionalComponentArgsRest(__VLS_172));
const { default: __VLS_176 } = __VLS_174.slots;
{
    const { default: __VLS_177 } = __VLS_174.slots;
    const [{ row }] = __VLS_vSlot(__VLS_177);
    let __VLS_178;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_179 = __VLS_asFunctionalComponent1(__VLS_178, new __VLS_178({
        type: (row.cpa_uploaded ? 'success' : 'info'),
        effect: "plain",
    }));
    const __VLS_180 = __VLS_179({
        type: (row.cpa_uploaded ? 'success' : 'info'),
        effect: "plain",
    }, ...__VLS_functionalComponentArgsRest(__VLS_179));
    const { default: __VLS_183 } = __VLS_181.slots;
    (row.cpa_uploaded ? '已上传' : '未上传');
    // @ts-ignore
    [];
    var __VLS_181;
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_174;
let __VLS_184;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_185 = __VLS_asFunctionalComponent1(__VLS_184, new __VLS_184({
    label: "操作",
    width: "260",
    fixed: "right",
}));
const __VLS_186 = __VLS_185({
    label: "操作",
    width: "260",
    fixed: "right",
}, ...__VLS_functionalComponentArgsRest(__VLS_185));
const { default: __VLS_189 } = __VLS_187.slots;
{
    const { default: __VLS_190 } = __VLS_187.slots;
    const [{ row }] = __VLS_vSlot(__VLS_190);
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "row-actions" },
    });
    /** @type {__VLS_StyleScopedClasses['row-actions']} */ ;
    let __VLS_191;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_192 = __VLS_asFunctionalComponent1(__VLS_191, new __VLS_191({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        loading: (__VLS_ctx.actionLoading[row.id] === 'refresh'),
    }));
    const __VLS_193 = __VLS_192({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        loading: (__VLS_ctx.actionLoading[row.id] === 'refresh'),
    }, ...__VLS_functionalComponentArgsRest(__VLS_192));
    let __VLS_196;
    const __VLS_197 = ({ click: {} },
        { onClick: (...[$event]) => {
                __VLS_ctx.runRowAction(row, 'refresh');
                // @ts-ignore
                [actionLoading, runRowAction,];
            } });
    const { default: __VLS_198 } = __VLS_194.slots;
    // @ts-ignore
    [];
    var __VLS_194;
    var __VLS_195;
    let __VLS_199;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_200 = __VLS_asFunctionalComponent1(__VLS_199, new __VLS_199({
        ...{ 'onClick': {} },
        link: true,
        type: "success",
        loading: (__VLS_ctx.actionLoading[row.id] === 'validate'),
    }));
    const __VLS_201 = __VLS_200({
        ...{ 'onClick': {} },
        link: true,
        type: "success",
        loading: (__VLS_ctx.actionLoading[row.id] === 'validate'),
    }, ...__VLS_functionalComponentArgsRest(__VLS_200));
    let __VLS_204;
    const __VLS_205 = ({ click: {} },
        { onClick: (...[$event]) => {
                __VLS_ctx.runRowAction(row, 'validate');
                // @ts-ignore
                [actionLoading, runRowAction,];
            } });
    const { default: __VLS_206 } = __VLS_202.slots;
    // @ts-ignore
    [];
    var __VLS_202;
    var __VLS_203;
    let __VLS_207;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_208 = __VLS_asFunctionalComponent1(__VLS_207, new __VLS_207({
        ...{ 'onClick': {} },
        link: true,
        type: "warning",
        loading: (__VLS_ctx.actionLoading[row.id] === 'upload-cpa'),
    }));
    const __VLS_209 = __VLS_208({
        ...{ 'onClick': {} },
        link: true,
        type: "warning",
        loading: (__VLS_ctx.actionLoading[row.id] === 'upload-cpa'),
    }, ...__VLS_functionalComponentArgsRest(__VLS_208));
    let __VLS_212;
    const __VLS_213 = ({ click: {} },
        { onClick: (...[$event]) => {
                __VLS_ctx.runRowAction(row, 'upload-cpa');
                // @ts-ignore
                [actionLoading, runRowAction,];
            } });
    const { default: __VLS_214 } = __VLS_210.slots;
    // @ts-ignore
    [];
    var __VLS_210;
    var __VLS_211;
    let __VLS_215;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_216 = __VLS_asFunctionalComponent1(__VLS_215, new __VLS_215({
        ...{ 'onClick': {} },
        link: true,
        type: "danger",
        loading: (__VLS_ctx.actionLoading[row.id] === 'reauthorize-codex'),
    }));
    const __VLS_217 = __VLS_216({
        ...{ 'onClick': {} },
        link: true,
        type: "danger",
        loading: (__VLS_ctx.actionLoading[row.id] === 'reauthorize-codex'),
    }, ...__VLS_functionalComponentArgsRest(__VLS_216));
    let __VLS_220;
    const __VLS_221 = ({ click: {} },
        { onClick: (...[$event]) => {
                __VLS_ctx.runCodexReauthorize(row);
                // @ts-ignore
                [actionLoading, runCodexReauthorize,];
            } });
    const { default: __VLS_222 } = __VLS_218.slots;
    // @ts-ignore
    [];
    var __VLS_218;
    var __VLS_219;
    let __VLS_223;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_224 = __VLS_asFunctionalComponent1(__VLS_223, new __VLS_223({
        ...{ 'onClick': {} },
        link: true,
        type: "info",
    }));
    const __VLS_225 = __VLS_224({
        ...{ 'onClick': {} },
        link: true,
        type: "info",
    }, ...__VLS_functionalComponentArgsRest(__VLS_224));
    let __VLS_228;
    const __VLS_229 = ({ click: {} },
        { onClick: (...[$event]) => {
                __VLS_ctx.openDetail(row.id);
                // @ts-ignore
                [openDetail,];
            } });
    const { default: __VLS_230 } = __VLS_226.slots;
    // @ts-ignore
    [];
    var __VLS_226;
    var __VLS_227;
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_187;
// @ts-ignore
[];
var __VLS_122;
var __VLS_123;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "pagination" },
});
/** @type {__VLS_StyleScopedClasses['pagination']} */ ;
let __VLS_231;
/** @ts-ignore @type {typeof __VLS_components.elPagination | typeof __VLS_components.ElPagination} */
elPagination;
// @ts-ignore
const __VLS_232 = __VLS_asFunctionalComponent1(__VLS_231, new __VLS_231({
    ...{ 'onCurrentChange': {} },
    ...{ 'onSizeChange': {} },
    currentPage: (__VLS_ctx.filters.page),
    pageSize: (__VLS_ctx.filters.pageSize),
    pageSizes: ([10, 20, 50, 100]),
    total: (__VLS_ctx.total),
    background: true,
    layout: "total, sizes, prev, pager, next",
}));
const __VLS_233 = __VLS_232({
    ...{ 'onCurrentChange': {} },
    ...{ 'onSizeChange': {} },
    currentPage: (__VLS_ctx.filters.page),
    pageSize: (__VLS_ctx.filters.pageSize),
    pageSizes: ([10, 20, 50, 100]),
    total: (__VLS_ctx.total),
    background: true,
    layout: "total, sizes, prev, pager, next",
}, ...__VLS_functionalComponentArgsRest(__VLS_232));
let __VLS_236;
const __VLS_237 = ({ currentChange: {} },
    { onCurrentChange: (__VLS_ctx.handlePageChange) });
const __VLS_238 = ({ sizeChange: {} },
    { onSizeChange: (__VLS_ctx.handlePageSizeChange) });
var __VLS_234;
var __VLS_235;
// @ts-ignore
[filters, filters, total, handlePageChange, handlePageSizeChange,];
var __VLS_27;
let __VLS_239;
/** @ts-ignore @type {typeof __VLS_components.elDrawer | typeof __VLS_components.ElDrawer | typeof __VLS_components.elDrawer | typeof __VLS_components.ElDrawer} */
elDrawer;
// @ts-ignore
const __VLS_240 = __VLS_asFunctionalComponent1(__VLS_239, new __VLS_239({
    modelValue: (__VLS_ctx.detailVisible),
    title: "账号详情",
    size: "520px",
}));
const __VLS_241 = __VLS_240({
    modelValue: (__VLS_ctx.detailVisible),
    title: "账号详情",
    size: "520px",
}, ...__VLS_functionalComponentArgsRest(__VLS_240));
const { default: __VLS_244 } = __VLS_242.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "detail-panel" },
});
__VLS_asFunctionalDirective(__VLS_directives.vLoading, {})(null, { ...__VLS_directiveBindingRestFields, value: (__VLS_ctx.detailLoading) }, null, null);
/** @type {__VLS_StyleScopedClasses['detail-panel']} */ ;
if (__VLS_ctx.selectedAccount) {
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "detail-header" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({});
    __VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
    (__VLS_ctx.selectedAccount.email);
    __VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({
        ...{ class: "detail-header__subtitle" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-header__subtitle']} */ ;
    (__VLS_ctx.serviceLabel(__VLS_ctx.selectedAccount.email_service));
    let __VLS_245;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_246 = __VLS_asFunctionalComponent1(__VLS_245, new __VLS_245({
        type: (__VLS_ctx.statusTagType(__VLS_ctx.selectedAccount.status)),
    }));
    const __VLS_247 = __VLS_246({
        type: (__VLS_ctx.statusTagType(__VLS_ctx.selectedAccount.status)),
    }, ...__VLS_functionalComponentArgsRest(__VLS_246));
    const { default: __VLS_250 } = __VLS_248.slots;
    (__VLS_ctx.selectedAccount.status);
    // @ts-ignore
    [vLoading, serviceLabel, statusTagType, detailVisible, detailLoading, selectedAccount, selectedAccount, selectedAccount, selectedAccount, selectedAccount,];
    var __VLS_248;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "detail-actions" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-actions']} */ ;
    let __VLS_251;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_252 = __VLS_asFunctionalComponent1(__VLS_251, new __VLS_251({
        ...{ 'onClick': {} },
        type: "primary",
        loading: (__VLS_ctx.actionLoading[__VLS_ctx.selectedAccount.id] === 'refresh'),
    }));
    const __VLS_253 = __VLS_252({
        ...{ 'onClick': {} },
        type: "primary",
        loading: (__VLS_ctx.actionLoading[__VLS_ctx.selectedAccount.id] === 'refresh'),
    }, ...__VLS_functionalComponentArgsRest(__VLS_252));
    let __VLS_256;
    const __VLS_257 = ({ click: {} },
        { onClick: (...[$event]) => {
                if (!(__VLS_ctx.selectedAccount))
                    return;
                __VLS_ctx.runRowAction(__VLS_ctx.selectedAccount, 'refresh', true);
                // @ts-ignore
                [actionLoading, runRowAction, selectedAccount, selectedAccount,];
            } });
    const { default: __VLS_258 } = __VLS_254.slots;
    // @ts-ignore
    [];
    var __VLS_254;
    var __VLS_255;
    let __VLS_259;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_260 = __VLS_asFunctionalComponent1(__VLS_259, new __VLS_259({
        ...{ 'onClick': {} },
        type: "success",
        plain: true,
        loading: (__VLS_ctx.actionLoading[__VLS_ctx.selectedAccount.id] === 'validate'),
    }));
    const __VLS_261 = __VLS_260({
        ...{ 'onClick': {} },
        type: "success",
        plain: true,
        loading: (__VLS_ctx.actionLoading[__VLS_ctx.selectedAccount.id] === 'validate'),
    }, ...__VLS_functionalComponentArgsRest(__VLS_260));
    let __VLS_264;
    const __VLS_265 = ({ click: {} },
        { onClick: (...[$event]) => {
                if (!(__VLS_ctx.selectedAccount))
                    return;
                __VLS_ctx.runRowAction(__VLS_ctx.selectedAccount, 'validate', true);
                // @ts-ignore
                [actionLoading, runRowAction, selectedAccount, selectedAccount,];
            } });
    const { default: __VLS_266 } = __VLS_262.slots;
    // @ts-ignore
    [];
    var __VLS_262;
    var __VLS_263;
    let __VLS_267;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_268 = __VLS_asFunctionalComponent1(__VLS_267, new __VLS_267({
        ...{ 'onClick': {} },
        type: "warning",
        plain: true,
        loading: (__VLS_ctx.actionLoading[__VLS_ctx.selectedAccount.id] === 'upload-cpa'),
    }));
    const __VLS_269 = __VLS_268({
        ...{ 'onClick': {} },
        type: "warning",
        plain: true,
        loading: (__VLS_ctx.actionLoading[__VLS_ctx.selectedAccount.id] === 'upload-cpa'),
    }, ...__VLS_functionalComponentArgsRest(__VLS_268));
    let __VLS_272;
    const __VLS_273 = ({ click: {} },
        { onClick: (...[$event]) => {
                if (!(__VLS_ctx.selectedAccount))
                    return;
                __VLS_ctx.runRowAction(__VLS_ctx.selectedAccount, 'upload-cpa', true);
                // @ts-ignore
                [actionLoading, runRowAction, selectedAccount, selectedAccount,];
            } });
    const { default: __VLS_274 } = __VLS_270.slots;
    // @ts-ignore
    [];
    var __VLS_270;
    var __VLS_271;
    let __VLS_275;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_276 = __VLS_asFunctionalComponent1(__VLS_275, new __VLS_275({
        ...{ 'onClick': {} },
        type: "danger",
        plain: true,
        loading: (__VLS_ctx.actionLoading[__VLS_ctx.selectedAccount.id] === 'reauthorize-codex'),
    }));
    const __VLS_277 = __VLS_276({
        ...{ 'onClick': {} },
        type: "danger",
        plain: true,
        loading: (__VLS_ctx.actionLoading[__VLS_ctx.selectedAccount.id] === 'reauthorize-codex'),
    }, ...__VLS_functionalComponentArgsRest(__VLS_276));
    let __VLS_280;
    const __VLS_281 = ({ click: {} },
        { onClick: (...[$event]) => {
                if (!(__VLS_ctx.selectedAccount))
                    return;
                __VLS_ctx.runCodexReauthorize(__VLS_ctx.selectedAccount, true);
                // @ts-ignore
                [actionLoading, runCodexReauthorize, selectedAccount, selectedAccount,];
            } });
    const { default: __VLS_282 } = __VLS_278.slots;
    // @ts-ignore
    [];
    var __VLS_278;
    var __VLS_279;
    let __VLS_283;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_284 = __VLS_asFunctionalComponent1(__VLS_283, new __VLS_283({
        ...{ 'onClick': {} },
        type: "info",
        plain: true,
        loading: (__VLS_ctx.linkRegenerating),
        disabled: (!__VLS_ctx.selectedTokens.access_token),
    }));
    const __VLS_285 = __VLS_284({
        ...{ 'onClick': {} },
        type: "info",
        plain: true,
        loading: (__VLS_ctx.linkRegenerating),
        disabled: (!__VLS_ctx.selectedTokens.access_token),
    }, ...__VLS_functionalComponentArgsRest(__VLS_284));
    let __VLS_288;
    const __VLS_289 = ({ click: {} },
        { onClick: (__VLS_ctx.regenerateBindCardLinks) });
    const { default: __VLS_290 } = __VLS_286.slots;
    // @ts-ignore
    [linkRegenerating, selectedTokens, regenerateBindCardLinks,];
    var __VLS_286;
    var __VLS_287;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "detail-grid" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-grid']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "detail-item" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    (__VLS_ctx.selectedAccount.account_id || '-');
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "detail-item" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    (__VLS_ctx.selectedAccount.workspace_id || '-');
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "detail-item" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    (__VLS_ctx.selectedAccount.client_id || '-');
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "detail-item" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    (__VLS_ctx.selectedAccount.proxy_used || '-');
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "detail-item" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    (__VLS_ctx.formatDate(__VLS_ctx.selectedAccount.registered_at));
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "detail-item" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    (__VLS_ctx.formatDate(__VLS_ctx.selectedAccount.last_refresh));
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "detail-item" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    (__VLS_ctx.formatDate(__VLS_ctx.selectedAccount.expires_at));
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "detail-item" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    (__VLS_ctx.formatDate(__VLS_ctx.selectedAccount.cpa_uploaded_at));
    let __VLS_291;
    /** @ts-ignore @type {typeof __VLS_components.elDivider | typeof __VLS_components.ElDivider | typeof __VLS_components.elDivider | typeof __VLS_components.ElDivider} */
    elDivider;
    // @ts-ignore
    const __VLS_292 = __VLS_asFunctionalComponent1(__VLS_291, new __VLS_291({
        contentPosition: "left",
    }));
    const __VLS_293 = __VLS_292({
        contentPosition: "left",
    }, ...__VLS_functionalComponentArgsRest(__VLS_292));
    const { default: __VLS_296 } = __VLS_294.slots;
    // @ts-ignore
    [formatDate, formatDate, formatDate, formatDate, selectedAccount, selectedAccount, selectedAccount, selectedAccount, selectedAccount, selectedAccount, selectedAccount, selectedAccount,];
    var __VLS_294;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-grid" },
    });
    /** @type {__VLS_StyleScopedClasses['token-grid']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card__header" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card__header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    let __VLS_297;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_298 = __VLS_asFunctionalComponent1(__VLS_297, new __VLS_297({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.access_token),
    }));
    const __VLS_299 = __VLS_298({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.access_token),
    }, ...__VLS_functionalComponentArgsRest(__VLS_298));
    let __VLS_302;
    const __VLS_303 = ({ click: {} },
        { onClick: (...[$event]) => {
                if (!(__VLS_ctx.selectedAccount))
                    return;
                __VLS_ctx.copyValue(__VLS_ctx.selectedTokens.access_token, 'Access Token');
                // @ts-ignore
                [selectedTokens, selectedTokens, copyValue,];
            } });
    const { default: __VLS_304 } = __VLS_300.slots;
    // @ts-ignore
    [];
    var __VLS_300;
    var __VLS_301;
    __VLS_asFunctionalElement1(__VLS_intrinsics.code, __VLS_intrinsics.code)({});
    (__VLS_ctx.selectedTokens.access_token_summary || '-');
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card__header" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card__header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    let __VLS_305;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_306 = __VLS_asFunctionalComponent1(__VLS_305, new __VLS_305({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.refresh_token),
    }));
    const __VLS_307 = __VLS_306({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.refresh_token),
    }, ...__VLS_functionalComponentArgsRest(__VLS_306));
    let __VLS_310;
    const __VLS_311 = ({ click: {} },
        { onClick: (...[$event]) => {
                if (!(__VLS_ctx.selectedAccount))
                    return;
                __VLS_ctx.copyValue(__VLS_ctx.selectedTokens.refresh_token, 'Refresh Token');
                // @ts-ignore
                [selectedTokens, selectedTokens, selectedTokens, copyValue,];
            } });
    const { default: __VLS_312 } = __VLS_308.slots;
    // @ts-ignore
    [];
    var __VLS_308;
    var __VLS_309;
    __VLS_asFunctionalElement1(__VLS_intrinsics.code, __VLS_intrinsics.code)({});
    (__VLS_ctx.selectedTokens.refresh_token_summary || '-');
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card__header" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card__header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    let __VLS_313;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_314 = __VLS_asFunctionalComponent1(__VLS_313, new __VLS_313({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.id_token),
    }));
    const __VLS_315 = __VLS_314({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.id_token),
    }, ...__VLS_functionalComponentArgsRest(__VLS_314));
    let __VLS_318;
    const __VLS_319 = ({ click: {} },
        { onClick: (...[$event]) => {
                if (!(__VLS_ctx.selectedAccount))
                    return;
                __VLS_ctx.copyValue(__VLS_ctx.selectedTokens.id_token, 'ID Token');
                // @ts-ignore
                [selectedTokens, selectedTokens, selectedTokens, copyValue,];
            } });
    const { default: __VLS_320 } = __VLS_316.slots;
    // @ts-ignore
    [];
    var __VLS_316;
    var __VLS_317;
    __VLS_asFunctionalElement1(__VLS_intrinsics.code, __VLS_intrinsics.code)({});
    (__VLS_ctx.selectedTokens.id_token_summary || '-');
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card__header" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card__header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    let __VLS_321;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_322 = __VLS_asFunctionalComponent1(__VLS_321, new __VLS_321({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.session_token),
    }));
    const __VLS_323 = __VLS_322({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.session_token),
    }, ...__VLS_functionalComponentArgsRest(__VLS_322));
    let __VLS_326;
    const __VLS_327 = ({ click: {} },
        { onClick: (...[$event]) => {
                if (!(__VLS_ctx.selectedAccount))
                    return;
                __VLS_ctx.copyValue(__VLS_ctx.selectedTokens.session_token, 'Session Token');
                // @ts-ignore
                [selectedTokens, selectedTokens, selectedTokens, copyValue,];
            } });
    const { default: __VLS_328 } = __VLS_324.slots;
    // @ts-ignore
    [];
    var __VLS_324;
    var __VLS_325;
    __VLS_asFunctionalElement1(__VLS_intrinsics.code, __VLS_intrinsics.code)({});
    (__VLS_ctx.selectedTokens.session_token_summary || '-');
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card__header" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card__header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    let __VLS_329;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_330 = __VLS_asFunctionalComponent1(__VLS_329, new __VLS_329({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.bind_card_url),
    }));
    const __VLS_331 = __VLS_330({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.bind_card_url),
    }, ...__VLS_functionalComponentArgsRest(__VLS_330));
    let __VLS_334;
    const __VLS_335 = ({ click: {} },
        { onClick: (...[$event]) => {
                if (!(__VLS_ctx.selectedAccount))
                    return;
                __VLS_ctx.copyValue(__VLS_ctx.selectedTokens.bind_card_url, '绑卡短链');
                // @ts-ignore
                [selectedTokens, selectedTokens, selectedTokens, copyValue,];
            } });
    const { default: __VLS_336 } = __VLS_332.slots;
    // @ts-ignore
    [];
    var __VLS_332;
    var __VLS_333;
    __VLS_asFunctionalElement1(__VLS_intrinsics.code, __VLS_intrinsics.code)({});
    (__VLS_ctx.selectedTokens.bind_card_url_summary || '-');
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "token-card__header" },
    });
    /** @type {__VLS_StyleScopedClasses['token-card__header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
        ...{ class: "detail-item__label" },
    });
    /** @type {__VLS_StyleScopedClasses['detail-item__label']} */ ;
    let __VLS_337;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_338 = __VLS_asFunctionalComponent1(__VLS_337, new __VLS_337({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.bind_card_long_url),
    }));
    const __VLS_339 = __VLS_338({
        ...{ 'onClick': {} },
        link: true,
        type: "primary",
        disabled: (!__VLS_ctx.selectedTokens.bind_card_long_url),
    }, ...__VLS_functionalComponentArgsRest(__VLS_338));
    let __VLS_342;
    const __VLS_343 = ({ click: {} },
        { onClick: (...[$event]) => {
                if (!(__VLS_ctx.selectedAccount))
                    return;
                __VLS_ctx.copyValue(__VLS_ctx.selectedTokens.bind_card_long_url, '绑卡长链');
                // @ts-ignore
                [selectedTokens, selectedTokens, selectedTokens, copyValue,];
            } });
    const { default: __VLS_344 } = __VLS_340.slots;
    // @ts-ignore
    [];
    var __VLS_340;
    var __VLS_341;
    __VLS_asFunctionalElement1(__VLS_intrinsics.code, __VLS_intrinsics.code)({});
    (__VLS_ctx.selectedTokens.bind_card_long_url_summary || '-');
}
// @ts-ignore
[selectedTokens,];
var __VLS_242;
// @ts-ignore
[];
const __VLS_export = (await import('vue')).defineComponent({});
export default {};
