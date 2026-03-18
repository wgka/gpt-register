import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
const loading = ref(false);
const starting = ref(false);
const socket = ref(null);
const websocketState = ref('未连接');
const logs = ref([]);
const recentResults = ref([]);
const activeBatchID = ref('');
const registrationStats = reactive({
    by_status: {},
    today_count: 0,
});
const form = reactive({
    count: 1,
    concurrency: 1,
    interval_min: 5,
    interval_max: 15,
});
const current = reactive({
    status: '',
    total: 0,
    completed: 0,
    success: 0,
    failed: 0,
});
async function refreshData() {
    loading.value = true;
    try {
        await loadStats();
    }
    catch {
        ElMessage.error('刷新失败');
    }
    finally {
        loading.value = false;
    }
}
async function loadStats() {
    const response = await fetch('/api/registration/stats');
    if (!response.ok) {
        throw new Error('load stats failed');
    }
    const payload = (await response.json());
    registrationStats.by_status = payload.by_status ?? {};
    registrationStats.today_count = payload.today_count ?? 0;
}
async function startRegistration() {
    if (form.interval_max < form.interval_min) {
        ElMessage.warning('最大间隔不能小于最小间隔');
        return;
    }
    starting.value = true;
    try {
        resetCurrent();
        logs.value = [];
        recentResults.value = [];
        const response = await fetch('/api/registration/batch', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                count: form.count,
                concurrency: form.concurrency,
                interval_min: form.interval_min,
                interval_max: form.interval_max,
                email_service_type: 'meteormail',
            }),
        });
        if (!response.ok) {
            throw new Error('start failed');
        }
        const payload = (await response.json());
        activeBatchID.value = payload.batch_id;
        current.status = 'running';
        current.total = form.count;
        connectSocket(payload.batch_id);
        ElMessage.success('已开始');
    }
    catch {
        ElMessage.error('启动失败');
    }
    finally {
        starting.value = false;
    }
}
function connectSocket(batchID) {
    closeSocket();
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws/batch/${batchID}`);
    socket.value = ws;
    websocketState.value = '连接中';
    ws.onopen = () => {
        websocketState.value = '已连接';
    };
    ws.onmessage = (event) => {
        const payload = JSON.parse(event.data);
        applyEvent(payload);
    };
    ws.onerror = () => {
        websocketState.value = '连接异常';
    };
    ws.onclose = () => {
        if (socket.value === ws) {
            socket.value = null;
        }
        websocketState.value = activeBatchID.value ? '已断开' : '未连接';
    };
}
function applyEvent(payload) {
    const extra = payload.extra ?? {};
    if (payload.type === 'status') {
        current.status = payload.status || current.status;
        current.total = asNumber(extra.total, current.total);
        current.completed = asNumber(extra.completed, current.completed);
        current.success = asNumber(extra.success, current.success);
        current.failed = asNumber(extra.failed, current.failed);
        if (payload.message) {
            appendLog(payload.message);
        }
        if (isTerminalStatus(current.status)) {
            void loadStats();
        }
        return;
    }
    if (payload.type === 'log' && payload.message) {
        appendLog(payload.message);
        return;
    }
    if (payload.type === 'result') {
        const result = normalizeRecentResult(payload);
        if (!result) {
            return;
        }
        recentResults.value = [result, ...recentResults.value.filter((item) => item.task_uuid !== result.task_uuid)].slice(0, 8);
    }
}
function appendLog(message) {
    logs.value = [...logs.value, message].slice(-400);
}
function normalizeRecentResult(payload) {
    const extra = payload.extra ?? {};
    const taskUUID = asString(extra.task_uuid) || payload.task_uuid || `${Date.now()}`;
    const email = asString(extra.email);
    if (!email) {
        return null;
    }
    return {
        task_uuid: taskUUID,
        email,
        account_id: asString(extra.account_id),
        workspace_id: asString(extra.workspace_id),
        source: asString(extra.source) || 'register',
        bind_card_url: asString(extra.bind_card_url),
        bind_card_url_summary: asString(extra.bind_card_url_summary),
    };
}
function cancelRegistration() {
    if (!socket.value) {
        return;
    }
    socket.value.send(JSON.stringify({ type: 'cancel' }));
}
function closeSocket() {
    if (socket.value) {
        socket.value.close();
        socket.value = null;
    }
    websocketState.value = '未连接';
}
function resetCurrent() {
    activeBatchID.value = '';
    current.status = '';
    current.total = 0;
    current.completed = 0;
    current.success = 0;
    current.failed = 0;
}
function statusTagType(status) {
    switch (status) {
        case 'completed':
            return 'success';
        case 'running':
        case 'cancelling':
            return 'warning';
        case 'failed':
            return 'danger';
        case 'cancelled':
            return 'info';
        default:
            return 'info';
    }
}
function isTerminalStatus(status) {
    return status === 'completed' || status === 'failed' || status === 'cancelled';
}
function asNumber(value, fallback) {
    return typeof value === 'number' ? value : fallback;
}
function asString(value) {
    return typeof value === 'string' ? value : '';
}
async function copyValue(value, label) {
    const trimmed = value.trim();
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
const canCancel = computed(() => current.status === 'running' || current.status === 'cancelling');
const websocketTagType = computed(() => {
    switch (websocketState.value) {
        case '已连接':
            return 'success';
        case '连接中':
            return 'warning';
        default:
            return 'info';
    }
});
onMounted(() => {
    refreshData().catch(() => {
        ElMessage.error('初始化失败');
    });
});
onBeforeUnmount(() => {
    closeSocket();
});
const __VLS_ctx = {
    ...{},
    ...{},
};
let __VLS_components;
let __VLS_intrinsics;
let __VLS_directives;
/** @type {__VLS_StyleScopedClasses['result-row']} */ ;
/** @type {__VLS_StyleScopedClasses['el-card__header']} */ ;
/** @type {__VLS_StyleScopedClasses['launch-card']} */ ;
/** @type {__VLS_StyleScopedClasses['log-card']} */ ;
/** @type {__VLS_StyleScopedClasses['el-card__body']} */ ;
/** @type {__VLS_StyleScopedClasses['launch-form']} */ ;
/** @type {__VLS_StyleScopedClasses['launch-form']} */ ;
/** @type {__VLS_StyleScopedClasses['launch-form']} */ ;
/** @type {__VLS_StyleScopedClasses['launch-form']} */ ;
/** @type {__VLS_StyleScopedClasses['el-input-number']} */ ;
/** @type {__VLS_StyleScopedClasses['summary-item']} */ ;
/** @type {__VLS_StyleScopedClasses['log-lines']} */ ;
/** @type {__VLS_StyleScopedClasses['console-grid']} */ ;
/** @type {__VLS_StyleScopedClasses['summary-grid']} */ ;
/** @type {__VLS_StyleScopedClasses['hero']} */ ;
/** @type {__VLS_StyleScopedClasses['section-header']} */ ;
/** @type {__VLS_StyleScopedClasses['status-bar']} */ ;
/** @type {__VLS_StyleScopedClasses['form-actions']} */ ;
/** @type {__VLS_StyleScopedClasses['result-card__header']} */ ;
/** @type {__VLS_StyleScopedClasses['hero']} */ ;
/** @type {__VLS_StyleScopedClasses['hero__title']} */ ;
/** @type {__VLS_StyleScopedClasses['result-row--meta']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "dashboard" },
});
/** @type {__VLS_StyleScopedClasses['dashboard']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.section, __VLS_intrinsics.section)({
    ...{ class: "hero page-card" },
});
/** @type {__VLS_StyleScopedClasses['hero']} */ ;
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "hero__copy" },
});
/** @type {__VLS_StyleScopedClasses['hero__copy']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "hero__eyebrow" },
});
/** @type {__VLS_StyleScopedClasses['hero__eyebrow']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.h1, __VLS_intrinsics.h1)({
    ...{ class: "hero__title" },
});
/** @type {__VLS_StyleScopedClasses['hero__title']} */ ;
let __VLS_0;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_1 = __VLS_asFunctionalComponent1(__VLS_0, new __VLS_0({
    ...{ 'onClick': {} },
    ...{ class: "hero__refresh" },
    loading: (__VLS_ctx.loading),
    type: "primary",
}));
const __VLS_2 = __VLS_1({
    ...{ 'onClick': {} },
    ...{ class: "hero__refresh" },
    loading: (__VLS_ctx.loading),
    type: "primary",
}, ...__VLS_functionalComponentArgsRest(__VLS_1));
let __VLS_5;
const __VLS_6 = ({ click: {} },
    { onClick: (__VLS_ctx.refreshData) });
/** @type {__VLS_StyleScopedClasses['hero__refresh']} */ ;
const { default: __VLS_7 } = __VLS_3.slots;
// @ts-ignore
[loading, refreshData,];
var __VLS_3;
var __VLS_4;
__VLS_asFunctionalElement1(__VLS_intrinsics.section, __VLS_intrinsics.section)({
    ...{ class: "console-grid" },
});
/** @type {__VLS_StyleScopedClasses['console-grid']} */ ;
let __VLS_8;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_9 = __VLS_asFunctionalComponent1(__VLS_8, new __VLS_8({
    ...{ class: "page-card launch-card" },
    shadow: "never",
}));
const __VLS_10 = __VLS_9({
    ...{ class: "page-card launch-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_9));
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
/** @type {__VLS_StyleScopedClasses['launch-card']} */ ;
const { default: __VLS_13 } = __VLS_11.slots;
{
    const { header: __VLS_14 } = __VLS_11.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "section-header" },
    });
    /** @type {__VLS_StyleScopedClasses['section-header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
        ...{ class: "page-title" },
    });
    /** @type {__VLS_StyleScopedClasses['page-title']} */ ;
    let __VLS_15;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_16 = __VLS_asFunctionalComponent1(__VLS_15, new __VLS_15({
        ...{ class: "section-tag" },
        type: "info",
    }));
    const __VLS_17 = __VLS_16({
        ...{ class: "section-tag" },
        type: "info",
    }, ...__VLS_functionalComponentArgsRest(__VLS_16));
    /** @type {__VLS_StyleScopedClasses['section-tag']} */ ;
    const { default: __VLS_20 } = __VLS_18.slots;
    (__VLS_ctx.registrationStats.today_count ?? 0);
    // @ts-ignore
    [registrationStats,];
    var __VLS_18;
    // @ts-ignore
    [];
}
let __VLS_21;
/** @ts-ignore @type {typeof __VLS_components.elForm | typeof __VLS_components.ElForm | typeof __VLS_components.elForm | typeof __VLS_components.ElForm} */
elForm;
// @ts-ignore
const __VLS_22 = __VLS_asFunctionalComponent1(__VLS_21, new __VLS_21({
    ...{ 'onSubmit': {} },
    ...{ class: "launch-form" },
    labelPosition: "top",
}));
const __VLS_23 = __VLS_22({
    ...{ 'onSubmit': {} },
    ...{ class: "launch-form" },
    labelPosition: "top",
}, ...__VLS_functionalComponentArgsRest(__VLS_22));
let __VLS_26;
const __VLS_27 = ({ submit: {} },
    { onSubmit: () => { } });
/** @type {__VLS_StyleScopedClasses['launch-form']} */ ;
const { default: __VLS_28 } = __VLS_24.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "launch-fields" },
});
/** @type {__VLS_StyleScopedClasses['launch-fields']} */ ;
let __VLS_29;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_30 = __VLS_asFunctionalComponent1(__VLS_29, new __VLS_29({
    label: "数量",
}));
const __VLS_31 = __VLS_30({
    label: "数量",
}, ...__VLS_functionalComponentArgsRest(__VLS_30));
const { default: __VLS_34 } = __VLS_32.slots;
let __VLS_35;
/** @ts-ignore @type {typeof __VLS_components.elInputNumber | typeof __VLS_components.ElInputNumber} */
elInputNumber;
// @ts-ignore
const __VLS_36 = __VLS_asFunctionalComponent1(__VLS_35, new __VLS_35({
    modelValue: (__VLS_ctx.form.count),
    min: (1),
    max: (100),
    controlsPosition: "right",
}));
const __VLS_37 = __VLS_36({
    modelValue: (__VLS_ctx.form.count),
    min: (1),
    max: (100),
    controlsPosition: "right",
}, ...__VLS_functionalComponentArgsRest(__VLS_36));
// @ts-ignore
[form,];
var __VLS_32;
let __VLS_40;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_41 = __VLS_asFunctionalComponent1(__VLS_40, new __VLS_40({
    label: "并发数",
}));
const __VLS_42 = __VLS_41({
    label: "并发数",
}, ...__VLS_functionalComponentArgsRest(__VLS_41));
const { default: __VLS_45 } = __VLS_43.slots;
let __VLS_46;
/** @ts-ignore @type {typeof __VLS_components.elInputNumber | typeof __VLS_components.ElInputNumber} */
elInputNumber;
// @ts-ignore
const __VLS_47 = __VLS_asFunctionalComponent1(__VLS_46, new __VLS_46({
    modelValue: (__VLS_ctx.form.concurrency),
    min: (1),
    max: (Math.min(20, __VLS_ctx.form.count)),
    controlsPosition: "right",
}));
const __VLS_48 = __VLS_47({
    modelValue: (__VLS_ctx.form.concurrency),
    min: (1),
    max: (Math.min(20, __VLS_ctx.form.count)),
    controlsPosition: "right",
}, ...__VLS_functionalComponentArgsRest(__VLS_47));
// @ts-ignore
[form, form,];
var __VLS_43;
let __VLS_51;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_52 = __VLS_asFunctionalComponent1(__VLS_51, new __VLS_51({
    label: "最小间隔（秒）",
}));
const __VLS_53 = __VLS_52({
    label: "最小间隔（秒）",
}, ...__VLS_functionalComponentArgsRest(__VLS_52));
const { default: __VLS_56 } = __VLS_54.slots;
let __VLS_57;
/** @ts-ignore @type {typeof __VLS_components.elInputNumber | typeof __VLS_components.ElInputNumber} */
elInputNumber;
// @ts-ignore
const __VLS_58 = __VLS_asFunctionalComponent1(__VLS_57, new __VLS_57({
    modelValue: (__VLS_ctx.form.interval_min),
    min: (0),
    max: (3600),
    controlsPosition: "right",
}));
const __VLS_59 = __VLS_58({
    modelValue: (__VLS_ctx.form.interval_min),
    min: (0),
    max: (3600),
    controlsPosition: "right",
}, ...__VLS_functionalComponentArgsRest(__VLS_58));
// @ts-ignore
[form,];
var __VLS_54;
let __VLS_62;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_63 = __VLS_asFunctionalComponent1(__VLS_62, new __VLS_62({
    label: "最大间隔（秒）",
}));
const __VLS_64 = __VLS_63({
    label: "最大间隔（秒）",
}, ...__VLS_functionalComponentArgsRest(__VLS_63));
const { default: __VLS_67 } = __VLS_65.slots;
let __VLS_68;
/** @ts-ignore @type {typeof __VLS_components.elInputNumber | typeof __VLS_components.ElInputNumber} */
elInputNumber;
// @ts-ignore
const __VLS_69 = __VLS_asFunctionalComponent1(__VLS_68, new __VLS_68({
    modelValue: (__VLS_ctx.form.interval_max),
    min: (0),
    max: (3600),
    controlsPosition: "right",
}));
const __VLS_70 = __VLS_69({
    modelValue: (__VLS_ctx.form.interval_max),
    min: (0),
    max: (3600),
    controlsPosition: "right",
}, ...__VLS_functionalComponentArgsRest(__VLS_69));
// @ts-ignore
[form,];
var __VLS_65;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "form-actions" },
});
/** @type {__VLS_StyleScopedClasses['form-actions']} */ ;
let __VLS_73;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_74 = __VLS_asFunctionalComponent1(__VLS_73, new __VLS_73({
    ...{ 'onClick': {} },
    ...{ class: "action-btn" },
    type: "primary",
    loading: (__VLS_ctx.starting),
}));
const __VLS_75 = __VLS_74({
    ...{ 'onClick': {} },
    ...{ class: "action-btn" },
    type: "primary",
    loading: (__VLS_ctx.starting),
}, ...__VLS_functionalComponentArgsRest(__VLS_74));
let __VLS_78;
const __VLS_79 = ({ click: {} },
    { onClick: (__VLS_ctx.startRegistration) });
/** @type {__VLS_StyleScopedClasses['action-btn']} */ ;
const { default: __VLS_80 } = __VLS_76.slots;
// @ts-ignore
[starting, startRegistration,];
var __VLS_76;
var __VLS_77;
let __VLS_81;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_82 = __VLS_asFunctionalComponent1(__VLS_81, new __VLS_81({
    ...{ 'onClick': {} },
    ...{ class: "action-btn" },
    type: "danger",
    plain: true,
    disabled: (!__VLS_ctx.socket || !__VLS_ctx.canCancel),
}));
const __VLS_83 = __VLS_82({
    ...{ 'onClick': {} },
    ...{ class: "action-btn" },
    type: "danger",
    plain: true,
    disabled: (!__VLS_ctx.socket || !__VLS_ctx.canCancel),
}, ...__VLS_functionalComponentArgsRest(__VLS_82));
let __VLS_86;
const __VLS_87 = ({ click: {} },
    { onClick: (__VLS_ctx.cancelRegistration) });
/** @type {__VLS_StyleScopedClasses['action-btn']} */ ;
const { default: __VLS_88 } = __VLS_84.slots;
// @ts-ignore
[socket, canCancel, cancelRegistration,];
var __VLS_84;
var __VLS_85;
// @ts-ignore
[];
var __VLS_24;
var __VLS_25;
let __VLS_89;
/** @ts-ignore @type {typeof __VLS_components.elDivider | typeof __VLS_components.ElDivider | typeof __VLS_components.elDivider | typeof __VLS_components.ElDivider} */
elDivider;
// @ts-ignore
const __VLS_90 = __VLS_asFunctionalComponent1(__VLS_89, new __VLS_89({
    contentPosition: "left",
}));
const __VLS_91 = __VLS_90({
    contentPosition: "left",
}, ...__VLS_functionalComponentArgsRest(__VLS_90));
const { default: __VLS_94 } = __VLS_92.slots;
// @ts-ignore
[];
var __VLS_92;
if (__VLS_ctx.recentResults.length === 0) {
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "result-empty" },
    });
    /** @type {__VLS_StyleScopedClasses['result-empty']} */ ;
}
else {
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "result-list" },
    });
    /** @type {__VLS_StyleScopedClasses['result-list']} */ ;
    for (const [item] of __VLS_vFor((__VLS_ctx.recentResults))) {
        __VLS_asFunctionalElement1(__VLS_intrinsics.article, __VLS_intrinsics.article)({
            key: (item.task_uuid),
            ...{ class: "result-card" },
        });
        /** @type {__VLS_StyleScopedClasses['result-card']} */ ;
        __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
            ...{ class: "result-card__header" },
        });
        /** @type {__VLS_StyleScopedClasses['result-card__header']} */ ;
        __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({});
        __VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
        (item.email);
        __VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({
            ...{ class: "result-card__meta" },
        });
        /** @type {__VLS_StyleScopedClasses['result-card__meta']} */ ;
        (item.source === 'login' ? '已存在账号登录' : '新注册账号');
        let __VLS_95;
        /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
        elButton;
        // @ts-ignore
        const __VLS_96 = __VLS_asFunctionalComponent1(__VLS_95, new __VLS_95({
            ...{ 'onClick': {} },
            link: true,
            type: "primary",
            disabled: (!item.bind_card_url),
        }));
        const __VLS_97 = __VLS_96({
            ...{ 'onClick': {} },
            link: true,
            type: "primary",
            disabled: (!item.bind_card_url),
        }, ...__VLS_functionalComponentArgsRest(__VLS_96));
        let __VLS_100;
        const __VLS_101 = ({ click: {} },
            { onClick: (...[$event]) => {
                    if (!!(__VLS_ctx.recentResults.length === 0))
                        return;
                    __VLS_ctx.copyValue(item.bind_card_url, '绑卡链接');
                    // @ts-ignore
                    [recentResults, recentResults, copyValue,];
                } });
        const { default: __VLS_102 } = __VLS_98.slots;
        // @ts-ignore
        [];
        var __VLS_98;
        var __VLS_99;
        __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
            ...{ class: "result-row" },
        });
        /** @type {__VLS_StyleScopedClasses['result-row']} */ ;
        __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
            ...{ class: "result-row__label" },
        });
        /** @type {__VLS_StyleScopedClasses['result-row__label']} */ ;
        __VLS_asFunctionalElement1(__VLS_intrinsics.code, __VLS_intrinsics.code)({});
        (item.bind_card_url_summary || '-');
        __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
            ...{ class: "result-row result-row--meta" },
        });
        /** @type {__VLS_StyleScopedClasses['result-row']} */ ;
        /** @type {__VLS_StyleScopedClasses['result-row--meta']} */ ;
        __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
        (item.account_id || '-');
        __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
        (item.workspace_id || '-');
        // @ts-ignore
        [];
    }
}
// @ts-ignore
[];
var __VLS_11;
let __VLS_103;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_104 = __VLS_asFunctionalComponent1(__VLS_103, new __VLS_103({
    ...{ class: "page-card log-card" },
    shadow: "never",
}));
const __VLS_105 = __VLS_104({
    ...{ class: "page-card log-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_104));
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
/** @type {__VLS_StyleScopedClasses['log-card']} */ ;
const { default: __VLS_108 } = __VLS_106.slots;
{
    const { header: __VLS_109 } = __VLS_106.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "section-header" },
    });
    /** @type {__VLS_StyleScopedClasses['section-header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
        ...{ class: "page-title" },
    });
    /** @type {__VLS_StyleScopedClasses['page-title']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "status-bar" },
    });
    /** @type {__VLS_StyleScopedClasses['status-bar']} */ ;
    let __VLS_110;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_111 = __VLS_asFunctionalComponent1(__VLS_110, new __VLS_110({
        type: (__VLS_ctx.statusTagType(__VLS_ctx.current.status)),
    }));
    const __VLS_112 = __VLS_111({
        type: (__VLS_ctx.statusTagType(__VLS_ctx.current.status)),
    }, ...__VLS_functionalComponentArgsRest(__VLS_111));
    const { default: __VLS_115 } = __VLS_113.slots;
    (__VLS_ctx.current.status || '-');
    // @ts-ignore
    [statusTagType, current, current,];
    var __VLS_113;
    let __VLS_116;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_117 = __VLS_asFunctionalComponent1(__VLS_116, new __VLS_116({
        type: (__VLS_ctx.websocketTagType),
    }));
    const __VLS_118 = __VLS_117({
        type: (__VLS_ctx.websocketTagType),
    }, ...__VLS_functionalComponentArgsRest(__VLS_117));
    const { default: __VLS_121 } = __VLS_119.slots;
    (__VLS_ctx.websocketState);
    // @ts-ignore
    [websocketTagType, websocketState,];
    var __VLS_119;
    // @ts-ignore
    [];
}
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "summary-grid" },
});
/** @type {__VLS_StyleScopedClasses['summary-grid']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "summary-item" },
});
/** @type {__VLS_StyleScopedClasses['summary-item']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "summary-item__label" },
});
/** @type {__VLS_StyleScopedClasses['summary-item__label']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
(__VLS_ctx.current.total ?? 0);
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "summary-item" },
});
/** @type {__VLS_StyleScopedClasses['summary-item']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "summary-item__label" },
});
/** @type {__VLS_StyleScopedClasses['summary-item__label']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
(__VLS_ctx.current.completed ?? 0);
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "summary-item" },
});
/** @type {__VLS_StyleScopedClasses['summary-item']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "summary-item__label" },
});
/** @type {__VLS_StyleScopedClasses['summary-item__label']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
(__VLS_ctx.current.success ?? 0);
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "summary-item" },
});
/** @type {__VLS_StyleScopedClasses['summary-item']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "summary-item__label" },
});
/** @type {__VLS_StyleScopedClasses['summary-item__label']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.strong, __VLS_intrinsics.strong)({});
(__VLS_ctx.current.failed ?? 0);
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "log-panel" },
});
/** @type {__VLS_StyleScopedClasses['log-panel']} */ ;
if (__VLS_ctx.logs.length === 0) {
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "log-empty" },
    });
    /** @type {__VLS_StyleScopedClasses['log-empty']} */ ;
}
else {
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "log-lines" },
    });
    /** @type {__VLS_StyleScopedClasses['log-lines']} */ ;
    for (const [line, index] of __VLS_vFor((__VLS_ctx.logs))) {
        __VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({
            key: (`log-${index}`),
        });
        (line);
        // @ts-ignore
        [current, current, current, current, logs, logs,];
    }
}
// @ts-ignore
[];
var __VLS_106;
// @ts-ignore
[];
const __VLS_export = (await import('vue')).defineComponent({});
export default {};
