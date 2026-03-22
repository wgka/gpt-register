import { onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
const loading = ref(false);
const saving = ref(false);
const state = reactive({
    app: {
        name: '',
        version: '',
        debug: false,
    },
    runtime: {
        addr: '',
        database_url: '',
        database_driver: '',
        log_file: '',
    },
    sections: [],
    editable: {
        proxy: {
            url: '',
            api_url: '',
            attempts: 4,
            preflight_timeout: 12,
        },
        cpa: {
            enabled: false,
            api_url: '',
            api_token: '',
            proxy_url: '',
        },
        telegram: {
            bot_token: '',
            allowed_chat_ids: '',
            debug: false,
            restart_hint: '',
        },
    },
});
const form = reactive({
    proxy: {
        url: '',
        api_url: '',
        attempts: 4,
        preflight_timeout: 12,
    },
    cpa: {
        enabled: false,
        api_url: '',
        api_token: '',
        proxy_url: '',
    },
    telegram: {
        bot_token: '',
        allowed_chat_ids: '',
        debug: false,
        restart_hint: '',
    },
});
function syncEditable(editable) {
    state.editable = editable;
    form.proxy.url = editable.proxy.url;
    form.proxy.api_url = editable.proxy.api_url;
    form.proxy.attempts = editable.proxy.attempts;
    form.proxy.preflight_timeout = editable.proxy.preflight_timeout;
    form.cpa.enabled = editable.cpa.enabled;
    form.cpa.api_url = editable.cpa.api_url;
    form.cpa.api_token = editable.cpa.api_token;
    form.cpa.proxy_url = editable.cpa.proxy_url;
    form.telegram.bot_token = editable.telegram.bot_token;
    form.telegram.allowed_chat_ids = editable.telegram.allowed_chat_ids;
    form.telegram.debug = editable.telegram.debug;
    form.telegram.restart_hint = editable.telegram.restart_hint;
}
async function loadSettings() {
    loading.value = true;
    try {
        const response = await fetch('/api/settings');
        if (!response.ok) {
            throw new Error('load settings failed');
        }
        const payload = (await response.json());
        state.app = payload.app;
        state.runtime = payload.runtime;
        state.sections = payload.sections;
        syncEditable(payload.editable);
    }
    catch {
        ElMessage.error('加载配置失败');
    }
    finally {
        loading.value = false;
    }
}
async function saveSettings() {
    saving.value = true;
    try {
        const response = await fetch('/api/settings', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                editable: form,
            }),
        });
        if (!response.ok) {
            throw new Error('save settings failed');
        }
        const payload = (await response.json());
        syncEditable(payload.editable);
        ElMessage.success(payload.message || '保存成功');
    }
    catch {
        ElMessage.error('保存配置失败');
    }
    finally {
        saving.value = false;
    }
}
onMounted(() => {
    void loadSettings();
});
const __VLS_ctx = {
    ...{},
    ...{},
};
let __VLS_components;
let __VLS_intrinsics;
let __VLS_directives;
/** @type {__VLS_StyleScopedClasses['hero__actions']} */ ;
/** @type {__VLS_StyleScopedClasses['hero']} */ ;
/** @type {__VLS_StyleScopedClasses['hero__actions']} */ ;
/** @type {__VLS_StyleScopedClasses['hero__actions']} */ ;
/** @type {__VLS_StyleScopedClasses['el-button']} */ ;
/** @type {__VLS_StyleScopedClasses['page-subtitle']} */ ;
/** @type {__VLS_StyleScopedClasses['inline-grid']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "settings-page" },
});
/** @type {__VLS_StyleScopedClasses['settings-page']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.section, __VLS_intrinsics.section)({
    ...{ class: "hero page-card" },
});
/** @type {__VLS_StyleScopedClasses['hero']} */ ;
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
    ...{ class: "hero__eyebrow" },
});
/** @type {__VLS_StyleScopedClasses['hero__eyebrow']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.h1, __VLS_intrinsics.h1)({
    ...{ class: "page-title" },
});
/** @type {__VLS_StyleScopedClasses['page-title']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.p, __VLS_intrinsics.p)({
    ...{ class: "page-subtitle" },
});
/** @type {__VLS_StyleScopedClasses['page-subtitle']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "hero__actions" },
});
/** @type {__VLS_StyleScopedClasses['hero__actions']} */ ;
let __VLS_0;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_1 = __VLS_asFunctionalComponent1(__VLS_0, new __VLS_0({
    ...{ 'onClick': {} },
    plain: true,
    loading: (__VLS_ctx.loading),
}));
const __VLS_2 = __VLS_1({
    ...{ 'onClick': {} },
    plain: true,
    loading: (__VLS_ctx.loading),
}, ...__VLS_functionalComponentArgsRest(__VLS_1));
let __VLS_5;
const __VLS_6 = ({ click: {} },
    { onClick: (__VLS_ctx.loadSettings) });
const { default: __VLS_7 } = __VLS_3.slots;
// @ts-ignore
[loading, loadSettings,];
var __VLS_3;
var __VLS_4;
let __VLS_8;
/** @ts-ignore @type {typeof __VLS_components.elButton | typeof __VLS_components.ElButton | typeof __VLS_components.elButton | typeof __VLS_components.ElButton} */
elButton;
// @ts-ignore
const __VLS_9 = __VLS_asFunctionalComponent1(__VLS_8, new __VLS_8({
    ...{ 'onClick': {} },
    type: "primary",
    loading: (__VLS_ctx.saving),
}));
const __VLS_10 = __VLS_9({
    ...{ 'onClick': {} },
    type: "primary",
    loading: (__VLS_ctx.saving),
}, ...__VLS_functionalComponentArgsRest(__VLS_9));
let __VLS_13;
const __VLS_14 = ({ click: {} },
    { onClick: (__VLS_ctx.saveSettings) });
const { default: __VLS_15 } = __VLS_11.slots;
// @ts-ignore
[saving, saveSettings,];
var __VLS_11;
var __VLS_12;
__VLS_asFunctionalElement1(__VLS_intrinsics.section, __VLS_intrinsics.section)({
    ...{ class: "settings-grid" },
});
/** @type {__VLS_StyleScopedClasses['settings-grid']} */ ;
let __VLS_16;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_17 = __VLS_asFunctionalComponent1(__VLS_16, new __VLS_16({
    ...{ class: "page-card" },
    shadow: "never",
}));
const __VLS_18 = __VLS_17({
    ...{ class: "page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_17));
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_21 } = __VLS_19.slots;
{
    const { header: __VLS_22 } = __VLS_19.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "section-header" },
    });
    /** @type {__VLS_StyleScopedClasses['section-header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
        ...{ class: "section-title" },
    });
    /** @type {__VLS_StyleScopedClasses['section-title']} */ ;
    let __VLS_23;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_24 = __VLS_asFunctionalComponent1(__VLS_23, new __VLS_23({
        type: "info",
    }));
    const __VLS_25 = __VLS_24({
        type: "info",
    }, ...__VLS_functionalComponentArgsRest(__VLS_24));
    const { default: __VLS_28 } = __VLS_26.slots;
    // @ts-ignore
    [];
    var __VLS_26;
    // @ts-ignore
    [];
}
let __VLS_29;
/** @ts-ignore @type {typeof __VLS_components.elForm | typeof __VLS_components.ElForm | typeof __VLS_components.elForm | typeof __VLS_components.ElForm} */
elForm;
// @ts-ignore
const __VLS_30 = __VLS_asFunctionalComponent1(__VLS_29, new __VLS_29({
    labelPosition: "top",
    ...{ class: "config-form" },
}));
const __VLS_31 = __VLS_30({
    labelPosition: "top",
    ...{ class: "config-form" },
}, ...__VLS_functionalComponentArgsRest(__VLS_30));
/** @type {__VLS_StyleScopedClasses['config-form']} */ ;
const { default: __VLS_34 } = __VLS_32.slots;
let __VLS_35;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_36 = __VLS_asFunctionalComponent1(__VLS_35, new __VLS_35({
    label: "固定代理 URL",
}));
const __VLS_37 = __VLS_36({
    label: "固定代理 URL",
}, ...__VLS_functionalComponentArgsRest(__VLS_36));
const { default: __VLS_40 } = __VLS_38.slots;
let __VLS_41;
/** @ts-ignore @type {typeof __VLS_components.elInput | typeof __VLS_components.ElInput} */
elInput;
// @ts-ignore
const __VLS_42 = __VLS_asFunctionalComponent1(__VLS_41, new __VLS_41({
    modelValue: (__VLS_ctx.form.proxy.url),
    clearable: true,
    placeholder: "http://127.0.0.1:7897 或 socks5://127.0.0.1:7890",
}));
const __VLS_43 = __VLS_42({
    modelValue: (__VLS_ctx.form.proxy.url),
    clearable: true,
    placeholder: "http://127.0.0.1:7897 或 socks5://127.0.0.1:7890",
}, ...__VLS_functionalComponentArgsRest(__VLS_42));
// @ts-ignore
[form,];
var __VLS_38;
let __VLS_46;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_47 = __VLS_asFunctionalComponent1(__VLS_46, new __VLS_46({
    label: "动态代理 API URL",
}));
const __VLS_48 = __VLS_47({
    label: "动态代理 API URL",
}, ...__VLS_functionalComponentArgsRest(__VLS_47));
const { default: __VLS_51 } = __VLS_49.slots;
let __VLS_52;
/** @ts-ignore @type {typeof __VLS_components.elInput | typeof __VLS_components.ElInput} */
elInput;
// @ts-ignore
const __VLS_53 = __VLS_asFunctionalComponent1(__VLS_52, new __VLS_52({
    modelValue: (__VLS_ctx.form.proxy.api_url),
    clearable: true,
    placeholder: "返回代理池 JSON 的接口地址",
}));
const __VLS_54 = __VLS_53({
    modelValue: (__VLS_ctx.form.proxy.api_url),
    clearable: true,
    placeholder: "返回代理池 JSON 的接口地址",
}, ...__VLS_functionalComponentArgsRest(__VLS_53));
// @ts-ignore
[form,];
var __VLS_49;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "inline-grid" },
});
/** @type {__VLS_StyleScopedClasses['inline-grid']} */ ;
let __VLS_57;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_58 = __VLS_asFunctionalComponent1(__VLS_57, new __VLS_57({
    label: "最大尝试次数",
}));
const __VLS_59 = __VLS_58({
    label: "最大尝试次数",
}, ...__VLS_functionalComponentArgsRest(__VLS_58));
const { default: __VLS_62 } = __VLS_60.slots;
let __VLS_63;
/** @ts-ignore @type {typeof __VLS_components.elInputNumber | typeof __VLS_components.ElInputNumber} */
elInputNumber;
// @ts-ignore
const __VLS_64 = __VLS_asFunctionalComponent1(__VLS_63, new __VLS_63({
    modelValue: (__VLS_ctx.form.proxy.attempts),
    min: (1),
    max: (20),
    controlsPosition: "right",
}));
const __VLS_65 = __VLS_64({
    modelValue: (__VLS_ctx.form.proxy.attempts),
    min: (1),
    max: (20),
    controlsPosition: "right",
}, ...__VLS_functionalComponentArgsRest(__VLS_64));
// @ts-ignore
[form,];
var __VLS_60;
let __VLS_68;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_69 = __VLS_asFunctionalComponent1(__VLS_68, new __VLS_68({
    label: "预检超时（秒）",
}));
const __VLS_70 = __VLS_69({
    label: "预检超时（秒）",
}, ...__VLS_functionalComponentArgsRest(__VLS_69));
const { default: __VLS_73 } = __VLS_71.slots;
let __VLS_74;
/** @ts-ignore @type {typeof __VLS_components.elInputNumber | typeof __VLS_components.ElInputNumber} */
elInputNumber;
// @ts-ignore
const __VLS_75 = __VLS_asFunctionalComponent1(__VLS_74, new __VLS_74({
    modelValue: (__VLS_ctx.form.proxy.preflight_timeout),
    min: (3),
    max: (60),
    controlsPosition: "right",
}));
const __VLS_76 = __VLS_75({
    modelValue: (__VLS_ctx.form.proxy.preflight_timeout),
    min: (3),
    max: (60),
    controlsPosition: "right",
}, ...__VLS_functionalComponentArgsRest(__VLS_75));
// @ts-ignore
[form,];
var __VLS_71;
let __VLS_79;
/** @ts-ignore @type {typeof __VLS_components.elAlert | typeof __VLS_components.ElAlert} */
elAlert;
// @ts-ignore
const __VLS_80 = __VLS_asFunctionalComponent1(__VLS_79, new __VLS_79({
    type: "info",
    showIcon: true,
    closable: (false),
    title: "如果同时填写动态代理 API 和固定代理，系统会优先尝试动态代理，失败后回退到固定代理。",
}));
const __VLS_81 = __VLS_80({
    type: "info",
    showIcon: true,
    closable: (false),
    title: "如果同时填写动态代理 API 和固定代理，系统会优先尝试动态代理，失败后回退到固定代理。",
}, ...__VLS_functionalComponentArgsRest(__VLS_80));
// @ts-ignore
[];
var __VLS_32;
// @ts-ignore
[];
var __VLS_19;
let __VLS_84;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_85 = __VLS_asFunctionalComponent1(__VLS_84, new __VLS_84({
    ...{ class: "page-card" },
    shadow: "never",
}));
const __VLS_86 = __VLS_85({
    ...{ class: "page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_85));
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_89 } = __VLS_87.slots;
{
    const { header: __VLS_90 } = __VLS_87.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "section-header" },
    });
    /** @type {__VLS_StyleScopedClasses['section-header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
        ...{ class: "section-title" },
    });
    /** @type {__VLS_StyleScopedClasses['section-title']} */ ;
    let __VLS_91;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_92 = __VLS_asFunctionalComponent1(__VLS_91, new __VLS_91({
        type: (__VLS_ctx.form.cpa.enabled ? 'success' : 'info'),
    }));
    const __VLS_93 = __VLS_92({
        type: (__VLS_ctx.form.cpa.enabled ? 'success' : 'info'),
    }, ...__VLS_functionalComponentArgsRest(__VLS_92));
    const { default: __VLS_96 } = __VLS_94.slots;
    (__VLS_ctx.form.cpa.enabled ? '已启用' : '未启用');
    // @ts-ignore
    [form, form,];
    var __VLS_94;
    // @ts-ignore
    [];
}
let __VLS_97;
/** @ts-ignore @type {typeof __VLS_components.elForm | typeof __VLS_components.ElForm | typeof __VLS_components.elForm | typeof __VLS_components.ElForm} */
elForm;
// @ts-ignore
const __VLS_98 = __VLS_asFunctionalComponent1(__VLS_97, new __VLS_97({
    labelPosition: "top",
    ...{ class: "config-form" },
}));
const __VLS_99 = __VLS_98({
    labelPosition: "top",
    ...{ class: "config-form" },
}, ...__VLS_functionalComponentArgsRest(__VLS_98));
/** @type {__VLS_StyleScopedClasses['config-form']} */ ;
const { default: __VLS_102 } = __VLS_100.slots;
let __VLS_103;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_104 = __VLS_asFunctionalComponent1(__VLS_103, new __VLS_103({}));
const __VLS_105 = __VLS_104({}, ...__VLS_functionalComponentArgsRest(__VLS_104));
const { default: __VLS_108 } = __VLS_106.slots;
{
    const { label: __VLS_109 } = __VLS_106.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    // @ts-ignore
    [];
}
let __VLS_110;
/** @ts-ignore @type {typeof __VLS_components.elSwitch | typeof __VLS_components.ElSwitch} */
elSwitch;
// @ts-ignore
const __VLS_111 = __VLS_asFunctionalComponent1(__VLS_110, new __VLS_110({
    modelValue: (__VLS_ctx.form.cpa.enabled),
    inlinePrompt: true,
    activeText: "开",
    inactiveText: "关",
}));
const __VLS_112 = __VLS_111({
    modelValue: (__VLS_ctx.form.cpa.enabled),
    inlinePrompt: true,
    activeText: "开",
    inactiveText: "关",
}, ...__VLS_functionalComponentArgsRest(__VLS_111));
// @ts-ignore
[form,];
var __VLS_106;
let __VLS_115;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_116 = __VLS_asFunctionalComponent1(__VLS_115, new __VLS_115({
    label: "CPA API URL",
}));
const __VLS_117 = __VLS_116({
    label: "CPA API URL",
}, ...__VLS_functionalComponentArgsRest(__VLS_116));
const { default: __VLS_120 } = __VLS_118.slots;
let __VLS_121;
/** @ts-ignore @type {typeof __VLS_components.elInput | typeof __VLS_components.ElInput} */
elInput;
// @ts-ignore
const __VLS_122 = __VLS_asFunctionalComponent1(__VLS_121, new __VLS_121({
    modelValue: (__VLS_ctx.form.cpa.api_url),
    clearable: true,
    placeholder: "http://host:port/v0/management/auth-files",
}));
const __VLS_123 = __VLS_122({
    modelValue: (__VLS_ctx.form.cpa.api_url),
    clearable: true,
    placeholder: "http://host:port/v0/management/auth-files",
}, ...__VLS_functionalComponentArgsRest(__VLS_122));
// @ts-ignore
[form,];
var __VLS_118;
let __VLS_126;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_127 = __VLS_asFunctionalComponent1(__VLS_126, new __VLS_126({
    label: "CPA API Token",
}));
const __VLS_128 = __VLS_127({
    label: "CPA API Token",
}, ...__VLS_functionalComponentArgsRest(__VLS_127));
const { default: __VLS_131 } = __VLS_129.slots;
let __VLS_132;
/** @ts-ignore @type {typeof __VLS_components.elInput | typeof __VLS_components.ElInput} */
elInput;
// @ts-ignore
const __VLS_133 = __VLS_asFunctionalComponent1(__VLS_132, new __VLS_132({
    modelValue: (__VLS_ctx.form.cpa.api_token),
    type: "password",
    showPassword: true,
    clearable: true,
    placeholder: "输入 CPA Token",
}));
const __VLS_134 = __VLS_133({
    modelValue: (__VLS_ctx.form.cpa.api_token),
    type: "password",
    showPassword: true,
    clearable: true,
    placeholder: "输入 CPA Token",
}, ...__VLS_functionalComponentArgsRest(__VLS_133));
// @ts-ignore
[form,];
var __VLS_129;
let __VLS_137;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_138 = __VLS_asFunctionalComponent1(__VLS_137, new __VLS_137({
    label: "CPA 专用代理（可选）",
}));
const __VLS_139 = __VLS_138({
    label: "CPA 专用代理（可选）",
}, ...__VLS_functionalComponentArgsRest(__VLS_138));
const { default: __VLS_142 } = __VLS_140.slots;
let __VLS_143;
/** @ts-ignore @type {typeof __VLS_components.elInput | typeof __VLS_components.ElInput} */
elInput;
// @ts-ignore
const __VLS_144 = __VLS_asFunctionalComponent1(__VLS_143, new __VLS_143({
    modelValue: (__VLS_ctx.form.cpa.proxy_url),
    clearable: true,
    placeholder: "留空则直连；可填写 http/socks5 代理",
}));
const __VLS_145 = __VLS_144({
    modelValue: (__VLS_ctx.form.cpa.proxy_url),
    clearable: true,
    placeholder: "留空则直连；可填写 http/socks5 代理",
}, ...__VLS_functionalComponentArgsRest(__VLS_144));
// @ts-ignore
[form,];
var __VLS_140;
// @ts-ignore
[];
var __VLS_100;
// @ts-ignore
[];
var __VLS_87;
let __VLS_148;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_149 = __VLS_asFunctionalComponent1(__VLS_148, new __VLS_148({
    ...{ class: "page-card" },
    shadow: "never",
}));
const __VLS_150 = __VLS_149({
    ...{ class: "page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_149));
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_153 } = __VLS_151.slots;
{
    const { header: __VLS_154 } = __VLS_151.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
        ...{ class: "section-header" },
    });
    /** @type {__VLS_StyleScopedClasses['section-header']} */ ;
    __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
        ...{ class: "section-title" },
    });
    /** @type {__VLS_StyleScopedClasses['section-title']} */ ;
    let __VLS_155;
    /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
    elTag;
    // @ts-ignore
    const __VLS_156 = __VLS_asFunctionalComponent1(__VLS_155, new __VLS_155({
        type: "warning",
    }));
    const __VLS_157 = __VLS_156({
        type: "warning",
    }, ...__VLS_functionalComponentArgsRest(__VLS_156));
    const { default: __VLS_160 } = __VLS_158.slots;
    // @ts-ignore
    [];
    var __VLS_158;
    // @ts-ignore
    [];
}
let __VLS_161;
/** @ts-ignore @type {typeof __VLS_components.elForm | typeof __VLS_components.ElForm | typeof __VLS_components.elForm | typeof __VLS_components.ElForm} */
elForm;
// @ts-ignore
const __VLS_162 = __VLS_asFunctionalComponent1(__VLS_161, new __VLS_161({
    labelPosition: "top",
    ...{ class: "config-form" },
}));
const __VLS_163 = __VLS_162({
    labelPosition: "top",
    ...{ class: "config-form" },
}, ...__VLS_functionalComponentArgsRest(__VLS_162));
/** @type {__VLS_StyleScopedClasses['config-form']} */ ;
const { default: __VLS_166 } = __VLS_164.slots;
let __VLS_167;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_168 = __VLS_asFunctionalComponent1(__VLS_167, new __VLS_167({
    label: "TELEGRAM_BOT_TOKEN",
}));
const __VLS_169 = __VLS_168({
    label: "TELEGRAM_BOT_TOKEN",
}, ...__VLS_functionalComponentArgsRest(__VLS_168));
const { default: __VLS_172 } = __VLS_170.slots;
let __VLS_173;
/** @ts-ignore @type {typeof __VLS_components.elInput | typeof __VLS_components.ElInput} */
elInput;
// @ts-ignore
const __VLS_174 = __VLS_asFunctionalComponent1(__VLS_173, new __VLS_173({
    modelValue: (__VLS_ctx.form.telegram.bot_token),
    type: "password",
    showPassword: true,
    clearable: true,
    placeholder: "不填则不会启动 Telegram Bot",
}));
const __VLS_175 = __VLS_174({
    modelValue: (__VLS_ctx.form.telegram.bot_token),
    type: "password",
    showPassword: true,
    clearable: true,
    placeholder: "不填则不会启动 Telegram Bot",
}, ...__VLS_functionalComponentArgsRest(__VLS_174));
// @ts-ignore
[form,];
var __VLS_170;
let __VLS_178;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_179 = __VLS_asFunctionalComponent1(__VLS_178, new __VLS_178({
    label: "允许的 chat_id 白名单",
}));
const __VLS_180 = __VLS_179({
    label: "允许的 chat_id 白名单",
}, ...__VLS_functionalComponentArgsRest(__VLS_179));
const { default: __VLS_183 } = __VLS_181.slots;
let __VLS_184;
/** @ts-ignore @type {typeof __VLS_components.elInput | typeof __VLS_components.ElInput} */
elInput;
// @ts-ignore
const __VLS_185 = __VLS_asFunctionalComponent1(__VLS_184, new __VLS_184({
    modelValue: (__VLS_ctx.form.telegram.allowed_chat_ids),
    clearable: true,
    placeholder: "多个 chat_id 用英文逗号分隔",
}));
const __VLS_186 = __VLS_185({
    modelValue: (__VLS_ctx.form.telegram.allowed_chat_ids),
    clearable: true,
    placeholder: "多个 chat_id 用英文逗号分隔",
}, ...__VLS_functionalComponentArgsRest(__VLS_185));
// @ts-ignore
[form,];
var __VLS_181;
let __VLS_189;
/** @ts-ignore @type {typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem | typeof __VLS_components.elFormItem | typeof __VLS_components.ElFormItem} */
elFormItem;
// @ts-ignore
const __VLS_190 = __VLS_asFunctionalComponent1(__VLS_189, new __VLS_189({}));
const __VLS_191 = __VLS_190({}, ...__VLS_functionalComponentArgsRest(__VLS_190));
const { default: __VLS_194 } = __VLS_192.slots;
{
    const { label: __VLS_195 } = __VLS_192.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
    // @ts-ignore
    [];
}
let __VLS_196;
/** @ts-ignore @type {typeof __VLS_components.elSwitch | typeof __VLS_components.ElSwitch} */
elSwitch;
// @ts-ignore
const __VLS_197 = __VLS_asFunctionalComponent1(__VLS_196, new __VLS_196({
    modelValue: (__VLS_ctx.form.telegram.debug),
    inlinePrompt: true,
    activeText: "开",
    inactiveText: "关",
}));
const __VLS_198 = __VLS_197({
    modelValue: (__VLS_ctx.form.telegram.debug),
    inlinePrompt: true,
    activeText: "开",
    inactiveText: "关",
}, ...__VLS_functionalComponentArgsRest(__VLS_197));
// @ts-ignore
[form,];
var __VLS_192;
let __VLS_201;
/** @ts-ignore @type {typeof __VLS_components.elAlert | typeof __VLS_components.ElAlert} */
elAlert;
// @ts-ignore
const __VLS_202 = __VLS_asFunctionalComponent1(__VLS_201, new __VLS_201({
    type: "warning",
    showIcon: true,
    closable: (false),
    title: (__VLS_ctx.state.editable.telegram.restart_hint || 'Telegram Bot Token 变更后请重启应用'),
}));
const __VLS_203 = __VLS_202({
    type: "warning",
    showIcon: true,
    closable: (false),
    title: (__VLS_ctx.state.editable.telegram.restart_hint || 'Telegram Bot Token 变更后请重启应用'),
}, ...__VLS_functionalComponentArgsRest(__VLS_202));
// @ts-ignore
[state,];
var __VLS_164;
// @ts-ignore
[];
var __VLS_151;
let __VLS_206;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_207 = __VLS_asFunctionalComponent1(__VLS_206, new __VLS_206({
    ...{ class: "page-card" },
    shadow: "never",
}));
const __VLS_208 = __VLS_207({
    ...{ class: "page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_207));
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_211 } = __VLS_209.slots;
{
    const { header: __VLS_212 } = __VLS_209.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
        ...{ class: "section-title" },
    });
    /** @type {__VLS_StyleScopedClasses['section-title']} */ ;
    // @ts-ignore
    [];
}
let __VLS_213;
/** @ts-ignore @type {typeof __VLS_components.elDescriptions | typeof __VLS_components.ElDescriptions | typeof __VLS_components.elDescriptions | typeof __VLS_components.ElDescriptions} */
elDescriptions;
// @ts-ignore
const __VLS_214 = __VLS_asFunctionalComponent1(__VLS_213, new __VLS_213({
    column: (2),
    border: true,
    ...{ class: "runtime-grid" },
}));
const __VLS_215 = __VLS_214({
    column: (2),
    border: true,
    ...{ class: "runtime-grid" },
}, ...__VLS_functionalComponentArgsRest(__VLS_214));
/** @type {__VLS_StyleScopedClasses['runtime-grid']} */ ;
const { default: __VLS_218 } = __VLS_216.slots;
let __VLS_219;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_220 = __VLS_asFunctionalComponent1(__VLS_219, new __VLS_219({
    label: "应用名称",
}));
const __VLS_221 = __VLS_220({
    label: "应用名称",
}, ...__VLS_functionalComponentArgsRest(__VLS_220));
const { default: __VLS_224 } = __VLS_222.slots;
(__VLS_ctx.state.app?.name ?? '-');
// @ts-ignore
[state,];
var __VLS_222;
let __VLS_225;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_226 = __VLS_asFunctionalComponent1(__VLS_225, new __VLS_225({
    label: "版本",
}));
const __VLS_227 = __VLS_226({
    label: "版本",
}, ...__VLS_functionalComponentArgsRest(__VLS_226));
const { default: __VLS_230 } = __VLS_228.slots;
(__VLS_ctx.state.app?.version ?? '-');
// @ts-ignore
[state,];
var __VLS_228;
let __VLS_231;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_232 = __VLS_asFunctionalComponent1(__VLS_231, new __VLS_231({
    label: "调试模式",
}));
const __VLS_233 = __VLS_232({
    label: "调试模式",
}, ...__VLS_functionalComponentArgsRest(__VLS_232));
const { default: __VLS_236 } = __VLS_234.slots;
let __VLS_237;
/** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
elTag;
// @ts-ignore
const __VLS_238 = __VLS_asFunctionalComponent1(__VLS_237, new __VLS_237({
    type: (__VLS_ctx.state.app?.debug ? 'warning' : 'success'),
}));
const __VLS_239 = __VLS_238({
    type: (__VLS_ctx.state.app?.debug ? 'warning' : 'success'),
}, ...__VLS_functionalComponentArgsRest(__VLS_238));
const { default: __VLS_242 } = __VLS_240.slots;
(__VLS_ctx.state.app?.debug ? '开启' : '关闭');
// @ts-ignore
[state, state,];
var __VLS_240;
// @ts-ignore
[];
var __VLS_234;
let __VLS_243;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_244 = __VLS_asFunctionalComponent1(__VLS_243, new __VLS_243({
    label: "监听地址",
}));
const __VLS_245 = __VLS_244({
    label: "监听地址",
}, ...__VLS_functionalComponentArgsRest(__VLS_244));
const { default: __VLS_248 } = __VLS_246.slots;
(__VLS_ctx.state.runtime?.addr ?? '-');
// @ts-ignore
[state,];
var __VLS_246;
let __VLS_249;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_250 = __VLS_asFunctionalComponent1(__VLS_249, new __VLS_249({
    label: "数据库",
}));
const __VLS_251 = __VLS_250({
    label: "数据库",
}, ...__VLS_functionalComponentArgsRest(__VLS_250));
const { default: __VLS_254 } = __VLS_252.slots;
(__VLS_ctx.state.runtime?.database_url ?? '-');
// @ts-ignore
[state,];
var __VLS_252;
let __VLS_255;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_256 = __VLS_asFunctionalComponent1(__VLS_255, new __VLS_255({
    label: "日志文件",
}));
const __VLS_257 = __VLS_256({
    label: "日志文件",
}, ...__VLS_functionalComponentArgsRest(__VLS_256));
const { default: __VLS_260 } = __VLS_258.slots;
(__VLS_ctx.state.runtime?.log_file ?? '-');
// @ts-ignore
[state,];
var __VLS_258;
// @ts-ignore
[];
var __VLS_216;
// @ts-ignore
[];
var __VLS_209;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "sections-grid" },
});
/** @type {__VLS_StyleScopedClasses['sections-grid']} */ ;
for (const [section] of __VLS_vFor((__VLS_ctx.state.sections))) {
    let __VLS_261;
    /** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
    elCard;
    // @ts-ignore
    const __VLS_262 = __VLS_asFunctionalComponent1(__VLS_261, new __VLS_261({
        key: (section.category),
        ...{ class: "page-card" },
        shadow: "never",
    }));
    const __VLS_263 = __VLS_262({
        key: (section.category),
        ...{ class: "page-card" },
        shadow: "never",
    }, ...__VLS_functionalComponentArgsRest(__VLS_262));
    /** @type {__VLS_StyleScopedClasses['page-card']} */ ;
    const { default: __VLS_266 } = __VLS_264.slots;
    {
        const { header: __VLS_267 } = __VLS_264.slots;
        __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
            ...{ class: "section-header" },
        });
        /** @type {__VLS_StyleScopedClasses['section-header']} */ ;
        __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
            ...{ class: "section-title" },
        });
        /** @type {__VLS_StyleScopedClasses['section-title']} */ ;
        (section.title);
        let __VLS_268;
        /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
        elTag;
        // @ts-ignore
        const __VLS_269 = __VLS_asFunctionalComponent1(__VLS_268, new __VLS_268({
            type: "info",
        }));
        const __VLS_270 = __VLS_269({
            type: "info",
        }, ...__VLS_functionalComponentArgsRest(__VLS_269));
        const { default: __VLS_273 } = __VLS_271.slots;
        (section.category);
        // @ts-ignore
        [state,];
        var __VLS_271;
        // @ts-ignore
        [];
    }
    let __VLS_274;
    /** @ts-ignore @type {typeof __VLS_components.elTable | typeof __VLS_components.ElTable | typeof __VLS_components.elTable | typeof __VLS_components.ElTable} */
    elTable;
    // @ts-ignore
    const __VLS_275 = __VLS_asFunctionalComponent1(__VLS_274, new __VLS_274({
        data: (section.items),
        stripe: true,
    }));
    const __VLS_276 = __VLS_275({
        data: (section.items),
        stripe: true,
    }, ...__VLS_functionalComponentArgsRest(__VLS_275));
    const { default: __VLS_279 } = __VLS_277.slots;
    let __VLS_280;
    /** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
    elTableColumn;
    // @ts-ignore
    const __VLS_281 = __VLS_asFunctionalComponent1(__VLS_280, new __VLS_280({
        prop: "name",
        label: "字段",
        minWidth: "180",
    }));
    const __VLS_282 = __VLS_281({
        prop: "name",
        label: "字段",
        minWidth: "180",
    }, ...__VLS_functionalComponentArgsRest(__VLS_281));
    let __VLS_285;
    /** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
    elTableColumn;
    // @ts-ignore
    const __VLS_286 = __VLS_asFunctionalComponent1(__VLS_285, new __VLS_285({
        prop: "db_key",
        label: "数据库键",
        minWidth: "180",
    }));
    const __VLS_287 = __VLS_286({
        prop: "db_key",
        label: "数据库键",
        minWidth: "180",
    }, ...__VLS_functionalComponentArgsRest(__VLS_286));
    let __VLS_290;
    /** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
    elTableColumn;
    // @ts-ignore
    const __VLS_291 = __VLS_asFunctionalComponent1(__VLS_290, new __VLS_290({
        prop: "description",
        label: "说明",
        minWidth: "180",
    }));
    const __VLS_292 = __VLS_291({
        prop: "description",
        label: "说明",
        minWidth: "180",
    }, ...__VLS_functionalComponentArgsRest(__VLS_291));
    let __VLS_295;
    /** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
    elTableColumn;
    // @ts-ignore
    const __VLS_296 = __VLS_asFunctionalComponent1(__VLS_295, new __VLS_295({
        prop: "type",
        label: "类型",
        width: "100",
    }));
    const __VLS_297 = __VLS_296({
        prop: "type",
        label: "类型",
        width: "100",
    }, ...__VLS_functionalComponentArgsRest(__VLS_296));
    let __VLS_300;
    /** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
    elTableColumn;
    // @ts-ignore
    const __VLS_301 = __VLS_asFunctionalComponent1(__VLS_300, new __VLS_300({
        label: "当前值",
        minWidth: "220",
    }));
    const __VLS_302 = __VLS_301({
        label: "当前值",
        minWidth: "220",
    }, ...__VLS_functionalComponentArgsRest(__VLS_301));
    const { default: __VLS_305 } = __VLS_303.slots;
    {
        const { default: __VLS_306 } = __VLS_303.slots;
        const [{ row }] = __VLS_vSlot(__VLS_306);
        if (row.secret) {
            let __VLS_307;
            /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
            elTag;
            // @ts-ignore
            const __VLS_308 = __VLS_asFunctionalComponent1(__VLS_307, new __VLS_307({
                type: "warning",
                effect: "plain",
            }));
            const __VLS_309 = __VLS_308({
                type: "warning",
                effect: "plain",
            }, ...__VLS_functionalComponentArgsRest(__VLS_308));
            const { default: __VLS_312 } = __VLS_310.slots;
            // @ts-ignore
            [];
            var __VLS_310;
        }
        __VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({
            ...{ class: "value-text" },
        });
        /** @type {__VLS_StyleScopedClasses['value-text']} */ ;
        (row.value);
        // @ts-ignore
        [];
    }
    // @ts-ignore
    [];
    var __VLS_303;
    // @ts-ignore
    [];
    var __VLS_277;
    // @ts-ignore
    [];
    var __VLS_264;
    // @ts-ignore
    [];
}
// @ts-ignore
[];
const __VLS_export = (await import('vue')).defineComponent({});
export default {};
