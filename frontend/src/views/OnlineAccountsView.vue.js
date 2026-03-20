import { computed, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
const MANAGEMENT_BASE = (() => {
    const url = import.meta.env.VITE_CPA_API_URL;
    if (!url)
        return '';
    try {
        return new URL(url).origin;
    }
    catch {
        return '';
    }
})();
const MANAGEMENT_TOKEN = import.meta.env.VITE_CPA_API_TOKEN;
const INVALID_STATUS_MESSAGE = {
    error: {
        message: 'Your authentication token has been invalidated. Please try signing in again.',
        type: 'invalid_request_error',
        code: 'token_invalidated',
    },
};
const files = ref([]);
const loading = ref(false);
const deletingId = ref(null);
const cleaningInvalid = ref(false);
const searchText = ref('');
const filterStatus = ref('');
function isTokenInvalid(file) {
    if (!file.status_message)
        return false;
    try {
        const parsed = JSON.parse(file.status_message);
        return parsed?.error?.code === 'token_invalidated';
    }
    catch {
        return false;
    }
}
const invalidFiles = computed(() => files.value.filter(isTokenInvalid));
const filteredFiles = computed(() => {
    let result = files.value;
    if (searchText.value) {
        const q = searchText.value.toLowerCase();
        result = result.filter((f) => f.account.toLowerCase().includes(q) || f.email.toLowerCase().includes(q));
    }
    if (filterStatus.value === 'token_invalid') {
        result = result.filter(isTokenInvalid);
    }
    else if (filterStatus.value) {
        result = result.filter((f) => f.status === filterStatus.value);
    }
    return result;
});
function countByStatus(status) {
    return files.value.filter((f) => f.status === status).length;
}
const invalidTokenCount = computed(() => invalidFiles.value.length);
async function loadFiles() {
    loading.value = true;
    try {
        const response = await fetch(`${MANAGEMENT_BASE}/v0/management/auth-files`, {
            headers: { Authorization: `Bearer ${MANAGEMENT_TOKEN}` },
        });
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        const data = (await response.json());
        files.value = data.files ?? [];
    }
    catch (e) {
        ElMessage.error('加载线上账号失败: ' + (e instanceof Error ? e.message : String(e)));
    }
    finally {
        loading.value = false;
    }
}
async function deleteFile(file) {
    try {
        await ElMessageBox.confirm(`确定要删除账号 ${file.account} 吗？此操作不可撤销。`, '删除确认', { type: 'warning' });
    }
    catch {
        return;
    }
    deletingId.value = file.id;
    try {
        const response = await fetch(`${MANAGEMENT_BASE}/v0/management/auth-files?name=${encodeURIComponent(file.name)}`, { method: 'DELETE', headers: { Authorization: `Bearer ${MANAGEMENT_TOKEN}` } });
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
        for (const file of targets) {
            try {
                const response = await fetch(`${MANAGEMENT_BASE}/v0/management/auth-files?name=${encodeURIComponent(file.name)}`, { method: 'DELETE', headers: { Authorization: `Bearer ${MANAGEMENT_TOKEN}` } });
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
function statusTagType(status) {
    switch (status) {
        case 'active':
            return 'success';
        case 'disabled':
            return 'info';
        default:
            return 'warning';
    }
}
onMounted(() => {
    loadFiles();
});
const __VLS_ctx = {
    ...{},
    ...{},
};
let __VLS_components;
let __VLS_intrinsics;
let __VLS_directives;
/** @type {__VLS_StyleScopedClasses['summary-grid']} */ ;
/** @type {__VLS_StyleScopedClasses['toolbar']} */ ;
/** @type {__VLS_StyleScopedClasses['filters']} */ ;
/** @type {__VLS_StyleScopedClasses['summary-grid']} */ ;
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
(__VLS_ctx.invalidTokenCount);
// @ts-ignore
[invalidTokenCount,];
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
        type: "danger",
        plain: true,
        disabled: (__VLS_ctx.invalidFiles.length === 0),
        loading: (__VLS_ctx.cleaningInvalid),
    }));
    const __VLS_33 = __VLS_32({
        ...{ 'onClick': {} },
        type: "danger",
        plain: true,
        disabled: (__VLS_ctx.invalidFiles.length === 0),
        loading: (__VLS_ctx.cleaningInvalid),
    }, ...__VLS_functionalComponentArgsRest(__VLS_32));
    let __VLS_36;
    const __VLS_37 = ({ click: {} },
        { onClick: (__VLS_ctx.cleanAllInvalid) });
    const { default: __VLS_38 } = __VLS_34.slots;
    (__VLS_ctx.invalidFiles.length);
    // @ts-ignore
    [invalidFiles, invalidFiles, cleaningInvalid, cleanAllInvalid,];
    var __VLS_34;
    var __VLS_35;
    let __VLS_39;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_40 = __VLS_asFunctionalComponent1(__VLS_39, new __VLS_39({
        ...{ 'onClick': {} },
        loading: (__VLS_ctx.loading),
    }));
    const __VLS_41 = __VLS_40({
        ...{ 'onClick': {} },
        loading: (__VLS_ctx.loading),
    }, ...__VLS_functionalComponentArgsRest(__VLS_40));
    let __VLS_44;
    const __VLS_45 = ({ click: {} },
        { onClick: (__VLS_ctx.loadFiles) });
    const { default: __VLS_46 } = __VLS_42.slots;
    // @ts-ignore
    [loading, loadFiles,];
    var __VLS_42;
    var __VLS_43;
    // @ts-ignore
    [];
}
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "filters" },
});
/** @type {__VLS_StyleScopedClasses['filters']} */ ;
let __VLS_47;
/** @ts-ignore @type {typeof __VLS_components.elInput | typeof __VLS_components.ElInput} */
elInput;
// @ts-ignore
const __VLS_48 = __VLS_asFunctionalComponent1(__VLS_47, new __VLS_47({
    modelValue: (__VLS_ctx.searchText),
    clearable: true,
    placeholder: "搜索邮箱 / 账号",
}));
const __VLS_49 = __VLS_48({
    modelValue: (__VLS_ctx.searchText),
    clearable: true,
    placeholder: "搜索邮箱 / 账号",
}, ...__VLS_functionalComponentArgsRest(__VLS_48));
let __VLS_52;
/** @ts-ignore @type {typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect | typeof __VLS_components.elSelect | typeof __VLS_components.ElSelect} */
elSelect;
// @ts-ignore
const __VLS_53 = __VLS_asFunctionalComponent1(__VLS_52, new __VLS_52({
    modelValue: (__VLS_ctx.filterStatus),
    clearable: true,
    placeholder: "全部状态",
}));
const __VLS_54 = __VLS_53({
    modelValue: (__VLS_ctx.filterStatus),
    clearable: true,
    placeholder: "全部状态",
}, ...__VLS_functionalComponentArgsRest(__VLS_53));
const { default: __VLS_57 } = __VLS_55.slots;
let __VLS_58;
/** @ts-ignore @type {typeof __VLS_components.elOption | typeof __VLS_components.ElOption} */
elOption;
// @ts-ignore
const __VLS_59 = __VLS_asFunctionalComponent1(__VLS_58, new __VLS_58({
    label: "有效",
    value: "active",
}));
const __VLS_60 = __VLS_59({
    label: "有效",
    value: "active",
}, ...__VLS_functionalComponentArgsRest(__VLS_59));
let __VLS_63;
/** @ts-ignore @type {typeof __VLS_components.elOption | typeof __VLS_components.ElOption} */
elOption;
// @ts-ignore
const __VLS_64 = __VLS_asFunctionalComponent1(__VLS_63, new __VLS_63({
    label: "禁用",
    value: "disabled",
}));
const __VLS_65 = __VLS_64({
    label: "禁用",
    value: "disabled",
}, ...__VLS_functionalComponentArgsRest(__VLS_64));
let __VLS_68;
/** @ts-ignore @type {typeof __VLS_components.elOption | typeof __VLS_components.ElOption} */
elOption;
// @ts-ignore
const __VLS_69 = __VLS_asFunctionalComponent1(__VLS_68, new __VLS_68({
    label: "Token 失效",
    value: "token_invalid",
}));
const __VLS_70 = __VLS_69({
    label: "Token 失效",
    value: "token_invalid",
}, ...__VLS_functionalComponentArgsRest(__VLS_69));
// @ts-ignore
[searchText, filterStatus,];
var __VLS_55;
let __VLS_73;
/** @ts-ignore @type {typeof __VLS_components.elTable | typeof __VLS_components.ElTable | typeof __VLS_components.elTable | typeof __VLS_components.ElTable} */
elTable;
// @ts-ignore
const __VLS_74 = __VLS_asFunctionalComponent1(__VLS_73, new __VLS_73({
    data: (__VLS_ctx.filteredFiles),
    rowKey: "id",
    stripe: true,
    emptyText: "暂无线上账号数据",
}));
const __VLS_75 = __VLS_74({
    data: (__VLS_ctx.filteredFiles),
    rowKey: "id",
    stripe: true,
    emptyText: "暂无线上账号数据",
}, ...__VLS_functionalComponentArgsRest(__VLS_74));
__VLS_asFunctionalDirective(__VLS_directives.vLoading, {})(null, { ...__VLS_directiveBindingRestFields, value: (__VLS_ctx.loading) }, null, null);
const { default: __VLS_78 } = __VLS_76.slots;
let __VLS_79;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_80 = __VLS_asFunctionalComponent1(__VLS_79, new __VLS_79({
    prop: "account",
    label: "邮箱",
    minWidth: "240",
}));
const __VLS_81 = __VLS_80({
    prop: "account",
    label: "邮箱",
    minWidth: "240",
}, ...__VLS_functionalComponentArgsRest(__VLS_80));
let __VLS_84;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_85 = __VLS_asFunctionalComponent1(__VLS_84, new __VLS_84({
    label: "状态",
    width: "120",
}));
const __VLS_86 = __VLS_85({
    label: "状态",
    width: "120",
}, ...__VLS_functionalComponentArgsRest(__VLS_85));
const { default: __VLS_89 } = __VLS_87.slots;
{
    const { default: __VLS_90 } = __VLS_87.slots;
    const [{ row }] = __VLS_vSlot(__VLS_90);
    let __VLS_91;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_92 = __VLS_asFunctionalComponent1(__VLS_91, new __VLS_91({
        type: (__VLS_ctx.statusTagType(row.status)),
        effect: "light",
    }));
    const __VLS_93 = __VLS_92({
        type: (__VLS_ctx.statusTagType(row.status)),
        effect: "light",
    }, ...__VLS_functionalComponentArgsRest(__VLS_92));
    const { default: __VLS_96 } = __VLS_94.slots;
    (row.status);
    // @ts-ignore
    [loading, filteredFiles, vLoading, statusTagType,];
    var __VLS_94;
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_87;
let __VLS_97;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_98 = __VLS_asFunctionalComponent1(__VLS_97, new __VLS_97({
    label: "Token 状态",
    width: "140",
}));
const __VLS_99 = __VLS_98({
    label: "Token 状态",
    width: "140",
}, ...__VLS_functionalComponentArgsRest(__VLS_98));
const { default: __VLS_102 } = __VLS_100.slots;
{
    const { default: __VLS_103 } = __VLS_100.slots;
    const [{ row }] = __VLS_vSlot(__VLS_103);
    if (__VLS_ctx.isTokenInvalid(row)) {
        let __VLS_104;
        /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
        elTag;
        // @ts-ignore
        const __VLS_105 = __VLS_asFunctionalComponent1(__VLS_104, new __VLS_104({
            type: "danger",
            effect: "plain",
        }));
        const __VLS_106 = __VLS_105({
            type: "danger",
            effect: "plain",
        }, ...__VLS_functionalComponentArgsRest(__VLS_105));
        const { default: __VLS_109 } = __VLS_107.slots;
        // @ts-ignore
        [isTokenInvalid,];
        var __VLS_107;
    }
    else {
        let __VLS_110;
        /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
        elTag;
        // @ts-ignore
        const __VLS_111 = __VLS_asFunctionalComponent1(__VLS_110, new __VLS_110({
            type: "success",
            effect: "plain",
        }));
        const __VLS_112 = __VLS_111({
            type: "success",
            effect: "plain",
        }, ...__VLS_functionalComponentArgsRest(__VLS_111));
        const { default: __VLS_115 } = __VLS_113.slots;
        // @ts-ignore
        [];
        var __VLS_113;
    }
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_100;
let __VLS_116;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_117 = __VLS_asFunctionalComponent1(__VLS_116, new __VLS_116({
    label: "套餐",
    width: "100",
}));
const __VLS_118 = __VLS_117({
    label: "套餐",
    width: "100",
}, ...__VLS_functionalComponentArgsRest(__VLS_117));
const { default: __VLS_121 } = __VLS_119.slots;
{
    const { default: __VLS_122 } = __VLS_119.slots;
    const [{ row }] = __VLS_vSlot(__VLS_122);
    (row.id_token?.plan_type || '-');
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_119;
let __VLS_123;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_124 = __VLS_asFunctionalComponent1(__VLS_123, new __VLS_123({
    label: "创建时间",
    minWidth: "180",
}));
const __VLS_125 = __VLS_124({
    label: "创建时间",
    minWidth: "180",
}, ...__VLS_functionalComponentArgsRest(__VLS_124));
const { default: __VLS_128 } = __VLS_126.slots;
{
    const { default: __VLS_129 } = __VLS_126.slots;
    const [{ row }] = __VLS_vSlot(__VLS_129);
    (__VLS_ctx.formatDate(row.created_at));
    // @ts-ignore
    [formatDate,];
}
// @ts-ignore
[];
var __VLS_126;
let __VLS_130;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_131 = __VLS_asFunctionalComponent1(__VLS_130, new __VLS_130({
    label: "最后刷新",
    minWidth: "180",
}));
const __VLS_132 = __VLS_131({
    label: "最后刷新",
    minWidth: "180",
}, ...__VLS_functionalComponentArgsRest(__VLS_131));
const { default: __VLS_135 } = __VLS_133.slots;
{
    const { default: __VLS_136 } = __VLS_133.slots;
    const [{ row }] = __VLS_vSlot(__VLS_136);
    (__VLS_ctx.formatDate(row.last_refresh));
    // @ts-ignore
    [formatDate,];
}
// @ts-ignore
[];
var __VLS_133;
let __VLS_137;
/** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
elTableColumn;
// @ts-ignore
const __VLS_138 = __VLS_asFunctionalComponent1(__VLS_137, new __VLS_137({
    label: "操作",
    width: "120",
    fixed: "right",
}));
const __VLS_139 = __VLS_138({
    label: "操作",
    width: "120",
    fixed: "right",
}, ...__VLS_functionalComponentArgsRest(__VLS_138));
const { default: __VLS_142 } = __VLS_140.slots;
{
    const { default: __VLS_143 } = __VLS_140.slots;
    const [{ row }] = __VLS_vSlot(__VLS_143);
    let __VLS_144;
    /** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
    elButton;
    // @ts-ignore
    const __VLS_145 = __VLS_asFunctionalComponent1(__VLS_144, new __VLS_144({
        ...{ 'onClick': {} },
        link: true,
        type: "danger",
        loading: (__VLS_ctx.deletingId === row.id),
    }));
    const __VLS_146 = __VLS_145({
        ...{ 'onClick': {} },
        link: true,
        type: "danger",
        loading: (__VLS_ctx.deletingId === row.id),
    }, ...__VLS_functionalComponentArgsRest(__VLS_145));
    let __VLS_149;
    const __VLS_150 = ({ click: {} },
        { onClick: (...[$event]) => {
                __VLS_ctx.deleteFile(row);
                // @ts-ignore
                [deletingId, deleteFile,];
            } });
    const { default: __VLS_151 } = __VLS_147.slots;
    // @ts-ignore
    [];
    var __VLS_147;
    var __VLS_148;
    // @ts-ignore
    [];
}
// @ts-ignore
[];
var __VLS_140;
// @ts-ignore
[];
var __VLS_76;
// @ts-ignore
[];
var __VLS_27;
// @ts-ignore
[];
const __VLS_export = (await import('vue')).defineComponent({});
export default {};
