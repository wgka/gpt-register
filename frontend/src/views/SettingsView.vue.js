import { onMounted, reactive } from 'vue';
import { ElMessage } from 'element-plus';
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
});
async function loadSettings() {
    const response = await fetch('/api/settings');
    if (!response.ok) {
        throw new Error('load settings failed');
    }
    const payload = (await response.json());
    state.app = payload.app;
    state.runtime = payload.runtime;
    state.sections = payload.sections;
}
onMounted(() => {
    loadSettings().catch(() => {
        ElMessage.error('加载配置失败');
    });
});
const __VLS_ctx = {
    ...{},
    ...{},
};
let __VLS_components;
let __VLS_intrinsics;
let __VLS_directives;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "settings-page" },
});
/** @type {__VLS_StyleScopedClasses['settings-page']} */ ;
let __VLS_0;
/** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
elCard;
// @ts-ignore
const __VLS_1 = __VLS_asFunctionalComponent1(__VLS_0, new __VLS_0({
    ...{ class: "page-card" },
    shadow: "never",
}));
const __VLS_2 = __VLS_1({
    ...{ class: "page-card" },
    shadow: "never",
}, ...__VLS_functionalComponentArgsRest(__VLS_1));
/** @type {__VLS_StyleScopedClasses['page-card']} */ ;
const { default: __VLS_5 } = __VLS_3.slots;
{
    const { header: __VLS_6 } = __VLS_3.slots;
    __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
        ...{ class: "page-title" },
    });
    /** @type {__VLS_StyleScopedClasses['page-title']} */ ;
}
let __VLS_7;
/** @ts-ignore @type {typeof __VLS_components.elDescriptions | typeof __VLS_components.ElDescriptions | typeof __VLS_components.elDescriptions | typeof __VLS_components.ElDescriptions} */
elDescriptions;
// @ts-ignore
const __VLS_8 = __VLS_asFunctionalComponent1(__VLS_7, new __VLS_7({
    column: (1),
    border: true,
}));
const __VLS_9 = __VLS_8({
    column: (1),
    border: true,
}, ...__VLS_functionalComponentArgsRest(__VLS_8));
const { default: __VLS_12 } = __VLS_10.slots;
let __VLS_13;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_14 = __VLS_asFunctionalComponent1(__VLS_13, new __VLS_13({
    label: "应用名称",
}));
const __VLS_15 = __VLS_14({
    label: "应用名称",
}, ...__VLS_functionalComponentArgsRest(__VLS_14));
const { default: __VLS_18 } = __VLS_16.slots;
(__VLS_ctx.state.app?.name ?? '-');
// @ts-ignore
[state,];
var __VLS_16;
let __VLS_19;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_20 = __VLS_asFunctionalComponent1(__VLS_19, new __VLS_19({
    label: "版本",
}));
const __VLS_21 = __VLS_20({
    label: "版本",
}, ...__VLS_functionalComponentArgsRest(__VLS_20));
const { default: __VLS_24 } = __VLS_22.slots;
(__VLS_ctx.state.app?.version ?? '-');
// @ts-ignore
[state,];
var __VLS_22;
let __VLS_25;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_26 = __VLS_asFunctionalComponent1(__VLS_25, new __VLS_25({
    label: "调试模式",
}));
const __VLS_27 = __VLS_26({
    label: "调试模式",
}, ...__VLS_functionalComponentArgsRest(__VLS_26));
const { default: __VLS_30 } = __VLS_28.slots;
let __VLS_31;
/** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
elTag;
// @ts-ignore
const __VLS_32 = __VLS_asFunctionalComponent1(__VLS_31, new __VLS_31({
    type: (__VLS_ctx.state.app?.debug ? 'warning' : 'success'),
}));
const __VLS_33 = __VLS_32({
    type: (__VLS_ctx.state.app?.debug ? 'warning' : 'success'),
}, ...__VLS_functionalComponentArgsRest(__VLS_32));
const { default: __VLS_36 } = __VLS_34.slots;
(__VLS_ctx.state.app?.debug ? '开启' : '关闭');
// @ts-ignore
[state, state,];
var __VLS_34;
// @ts-ignore
[];
var __VLS_28;
let __VLS_37;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_38 = __VLS_asFunctionalComponent1(__VLS_37, new __VLS_37({
    label: "监听地址",
}));
const __VLS_39 = __VLS_38({
    label: "监听地址",
}, ...__VLS_functionalComponentArgsRest(__VLS_38));
const { default: __VLS_42 } = __VLS_40.slots;
(__VLS_ctx.state.runtime?.addr ?? '-');
// @ts-ignore
[state,];
var __VLS_40;
let __VLS_43;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_44 = __VLS_asFunctionalComponent1(__VLS_43, new __VLS_43({
    label: "数据库",
}));
const __VLS_45 = __VLS_44({
    label: "数据库",
}, ...__VLS_functionalComponentArgsRest(__VLS_44));
const { default: __VLS_48 } = __VLS_46.slots;
(__VLS_ctx.state.runtime?.database_url ?? '-');
// @ts-ignore
[state,];
var __VLS_46;
let __VLS_49;
/** @ts-ignore @type {typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem | typeof __VLS_components.elDescriptionsItem | typeof __VLS_components.ElDescriptionsItem} */
elDescriptionsItem;
// @ts-ignore
const __VLS_50 = __VLS_asFunctionalComponent1(__VLS_49, new __VLS_49({
    label: "日志文件",
}));
const __VLS_51 = __VLS_50({
    label: "日志文件",
}, ...__VLS_functionalComponentArgsRest(__VLS_50));
const { default: __VLS_54 } = __VLS_52.slots;
(__VLS_ctx.state.runtime?.log_file ?? '-');
// @ts-ignore
[state,];
var __VLS_52;
// @ts-ignore
[];
var __VLS_10;
// @ts-ignore
[];
var __VLS_3;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "sections-grid" },
});
/** @type {__VLS_StyleScopedClasses['sections-grid']} */ ;
for (const [section] of __VLS_vFor((__VLS_ctx.state.sections))) {
    let __VLS_55;
    /** @ts-ignore @type {typeof __VLS_components.elCard | typeof __VLS_components.ElCard | typeof __VLS_components.elCard | typeof __VLS_components.ElCard} */
    elCard;
    // @ts-ignore
    const __VLS_56 = __VLS_asFunctionalComponent1(__VLS_55, new __VLS_55({
        key: (section.category),
        ...{ class: "page-card" },
        shadow: "never",
    }));
    const __VLS_57 = __VLS_56({
        key: (section.category),
        ...{ class: "page-card" },
        shadow: "never",
    }, ...__VLS_functionalComponentArgsRest(__VLS_56));
    /** @type {__VLS_StyleScopedClasses['page-card']} */ ;
    const { default: __VLS_60 } = __VLS_58.slots;
    {
        const { header: __VLS_61 } = __VLS_58.slots;
        __VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
            ...{ class: "section-header" },
        });
        /** @type {__VLS_StyleScopedClasses['section-header']} */ ;
        __VLS_asFunctionalElement1(__VLS_intrinsics.h3, __VLS_intrinsics.h3)({
            ...{ class: "section-title" },
        });
        /** @type {__VLS_StyleScopedClasses['section-title']} */ ;
        (section.title);
        let __VLS_62;
        /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
        elTag;
        // @ts-ignore
        const __VLS_63 = __VLS_asFunctionalComponent1(__VLS_62, new __VLS_62({
            type: "info",
        }));
        const __VLS_64 = __VLS_63({
            type: "info",
        }, ...__VLS_functionalComponentArgsRest(__VLS_63));
        const { default: __VLS_67 } = __VLS_65.slots;
        (section.category);
        // @ts-ignore
        [state,];
        var __VLS_65;
        // @ts-ignore
        [];
    }
    let __VLS_68;
    /** @ts-ignore @type {typeof __VLS_components.elTable | typeof __VLS_components.ElTable | typeof __VLS_components.elTable | typeof __VLS_components.ElTable} */
    elTable;
    // @ts-ignore
    const __VLS_69 = __VLS_asFunctionalComponent1(__VLS_68, new __VLS_68({
        data: (section.items),
        stripe: true,
    }));
    const __VLS_70 = __VLS_69({
        data: (section.items),
        stripe: true,
    }, ...__VLS_functionalComponentArgsRest(__VLS_69));
    const { default: __VLS_73 } = __VLS_71.slots;
    let __VLS_74;
    /** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
    elTableColumn;
    // @ts-ignore
    const __VLS_75 = __VLS_asFunctionalComponent1(__VLS_74, new __VLS_74({
        prop: "name",
        label: "字段",
        minWidth: "180",
    }));
    const __VLS_76 = __VLS_75({
        prop: "name",
        label: "字段",
        minWidth: "180",
    }, ...__VLS_functionalComponentArgsRest(__VLS_75));
    let __VLS_79;
    /** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
    elTableColumn;
    // @ts-ignore
    const __VLS_80 = __VLS_asFunctionalComponent1(__VLS_79, new __VLS_79({
        prop: "db_key",
        label: "数据库键",
        minWidth: "180",
    }));
    const __VLS_81 = __VLS_80({
        prop: "db_key",
        label: "数据库键",
        minWidth: "180",
    }, ...__VLS_functionalComponentArgsRest(__VLS_80));
    let __VLS_84;
    /** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
    elTableColumn;
    // @ts-ignore
    const __VLS_85 = __VLS_asFunctionalComponent1(__VLS_84, new __VLS_84({
        prop: "description",
        label: "说明",
        minWidth: "180",
    }));
    const __VLS_86 = __VLS_85({
        prop: "description",
        label: "说明",
        minWidth: "180",
    }, ...__VLS_functionalComponentArgsRest(__VLS_85));
    let __VLS_89;
    /** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
    elTableColumn;
    // @ts-ignore
    const __VLS_90 = __VLS_asFunctionalComponent1(__VLS_89, new __VLS_89({
        prop: "type",
        label: "类型",
        width: "100",
    }));
    const __VLS_91 = __VLS_90({
        prop: "type",
        label: "类型",
        width: "100",
    }, ...__VLS_functionalComponentArgsRest(__VLS_90));
    let __VLS_94;
    /** @ts-ignore @type {typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn | typeof __VLS_components.elTableColumn | typeof __VLS_components.ElTableColumn} */
    elTableColumn;
    // @ts-ignore
    const __VLS_95 = __VLS_asFunctionalComponent1(__VLS_94, new __VLS_94({
        label: "当前值",
        minWidth: "220",
    }));
    const __VLS_96 = __VLS_95({
        label: "当前值",
        minWidth: "220",
    }, ...__VLS_functionalComponentArgsRest(__VLS_95));
    const { default: __VLS_99 } = __VLS_97.slots;
    {
        const { default: __VLS_100 } = __VLS_97.slots;
        const [{ row }] = __VLS_vSlot(__VLS_100);
        if (row.secret) {
            let __VLS_101;
            /** @ts-ignore @type {typeof __VLS_components.elTag | typeof __VLS_components.ElTag | typeof __VLS_components.elTag | typeof __VLS_components.ElTag} */
            elTag;
            // @ts-ignore
            const __VLS_102 = __VLS_asFunctionalComponent1(__VLS_101, new __VLS_101({
                type: "warning",
                effect: "plain",
            }));
            const __VLS_103 = __VLS_102({
                type: "warning",
                effect: "plain",
            }, ...__VLS_functionalComponentArgsRest(__VLS_102));
            const { default: __VLS_106 } = __VLS_104.slots;
            // @ts-ignore
            [];
            var __VLS_104;
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
    var __VLS_97;
    // @ts-ignore
    [];
    var __VLS_71;
    // @ts-ignore
    [];
    var __VLS_58;
    // @ts-ignore
    [];
}
// @ts-ignore
[];
const __VLS_export = (await import('vue')).defineComponent({});
export default {};
