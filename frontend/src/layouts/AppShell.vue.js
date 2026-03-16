import { computed } from 'vue';
import { useRoute } from 'vue-router';
const route = useRoute();
const heading = computed(() => {
    switch (route.name) {
        case 'accounts':
            return {
                title: '账号管理',
            };
        case 'settings':
            return {
                title: '系统设置页',
            };
        default:
            return {
                title: '注册执行台',
            };
    }
});
const activePath = computed(() => route.path);
const __VLS_ctx = {
    ...{},
    ...{},
};
let __VLS_components;
let __VLS_intrinsics;
let __VLS_directives;
/** @type {__VLS_StyleScopedClasses['menu']} */ ;
/** @type {__VLS_StyleScopedClasses['menu']} */ ;
/** @type {__VLS_StyleScopedClasses['el-menu-item']} */ ;
/** @type {__VLS_StyleScopedClasses['shell']} */ ;
/** @type {__VLS_StyleScopedClasses['shell__aside']} */ ;
/** @type {__VLS_StyleScopedClasses['shell__header']} */ ;
/** @type {__VLS_StyleScopedClasses['shell__heading']} */ ;
/** @type {__VLS_StyleScopedClasses['shell__main']} */ ;
let __VLS_0;
/** @ts-ignore @type {typeof __VLS_components.elContainer | typeof __VLS_components.ElContainer | typeof __VLS_components.elContainer | typeof __VLS_components.ElContainer} */
elContainer;
// @ts-ignore
const __VLS_1 = __VLS_asFunctionalComponent1(__VLS_0, new __VLS_0({
    ...{ class: "shell" },
}));
const __VLS_2 = __VLS_1({
    ...{ class: "shell" },
}, ...__VLS_functionalComponentArgsRest(__VLS_1));
var __VLS_5 = {};
/** @type {__VLS_StyleScopedClasses['shell']} */ ;
const { default: __VLS_6 } = __VLS_3.slots;
let __VLS_7;
/** @ts-ignore @type {typeof __VLS_components.elAside | typeof __VLS_components.ElAside | typeof __VLS_components.elAside | typeof __VLS_components.ElAside} */
elAside;
// @ts-ignore
const __VLS_8 = __VLS_asFunctionalComponent1(__VLS_7, new __VLS_7({
    ...{ class: "shell__aside" },
    width: "240px",
}));
const __VLS_9 = __VLS_8({
    ...{ class: "shell__aside" },
    width: "240px",
}, ...__VLS_functionalComponentArgsRest(__VLS_8));
/** @type {__VLS_StyleScopedClasses['shell__aside']} */ ;
const { default: __VLS_12 } = __VLS_10.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "brand" },
});
/** @type {__VLS_StyleScopedClasses['brand']} */ ;
__VLS_asFunctionalElement1(__VLS_intrinsics.h1, __VLS_intrinsics.h1)({
    ...{ class: "brand__title" },
});
/** @type {__VLS_StyleScopedClasses['brand__title']} */ ;
let __VLS_13;
/** @ts-ignore @type {typeof __VLS_components.elMenu | typeof __VLS_components.ElMenu | typeof __VLS_components.elMenu | typeof __VLS_components.ElMenu} */
elMenu;
// @ts-ignore
const __VLS_14 = __VLS_asFunctionalComponent1(__VLS_13, new __VLS_13({
    defaultActive: (__VLS_ctx.activePath),
    ...{ class: "menu" },
    router: true,
}));
const __VLS_15 = __VLS_14({
    defaultActive: (__VLS_ctx.activePath),
    ...{ class: "menu" },
    router: true,
}, ...__VLS_functionalComponentArgsRest(__VLS_14));
/** @type {__VLS_StyleScopedClasses['menu']} */ ;
const { default: __VLS_18 } = __VLS_16.slots;
let __VLS_19;
/** @ts-ignore @type {typeof __VLS_components.elMenuItem | typeof __VLS_components.ElMenuItem | typeof __VLS_components.elMenuItem | typeof __VLS_components.ElMenuItem} */
elMenuItem;
// @ts-ignore
const __VLS_20 = __VLS_asFunctionalComponent1(__VLS_19, new __VLS_19({
    index: "/",
}));
const __VLS_21 = __VLS_20({
    index: "/",
}, ...__VLS_functionalComponentArgsRest(__VLS_20));
const { default: __VLS_24 } = __VLS_22.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
// @ts-ignore
[activePath,];
var __VLS_22;
let __VLS_25;
/** @ts-ignore @type {typeof __VLS_components.elMenuItem | typeof __VLS_components.ElMenuItem | typeof __VLS_components.elMenuItem | typeof __VLS_components.ElMenuItem} */
elMenuItem;
// @ts-ignore
const __VLS_26 = __VLS_asFunctionalComponent1(__VLS_25, new __VLS_25({
    index: "/accounts",
}));
const __VLS_27 = __VLS_26({
    index: "/accounts",
}, ...__VLS_functionalComponentArgsRest(__VLS_26));
const { default: __VLS_30 } = __VLS_28.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
// @ts-ignore
[];
var __VLS_28;
let __VLS_31;
/** @ts-ignore @type {typeof __VLS_components.elMenuItem | typeof __VLS_components.ElMenuItem | typeof __VLS_components.elMenuItem | typeof __VLS_components.ElMenuItem} */
elMenuItem;
// @ts-ignore
const __VLS_32 = __VLS_asFunctionalComponent1(__VLS_31, new __VLS_31({
    index: "/settings",
}));
const __VLS_33 = __VLS_32({
    index: "/settings",
}, ...__VLS_functionalComponentArgsRest(__VLS_32));
const { default: __VLS_36 } = __VLS_34.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.span, __VLS_intrinsics.span)({});
// @ts-ignore
[];
var __VLS_34;
// @ts-ignore
[];
var __VLS_16;
// @ts-ignore
[];
var __VLS_10;
let __VLS_37;
/** @ts-ignore @type {typeof __VLS_components.elContainer | typeof __VLS_components.ElContainer | typeof __VLS_components.elContainer | typeof __VLS_components.ElContainer} */
elContainer;
// @ts-ignore
const __VLS_38 = __VLS_asFunctionalComponent1(__VLS_37, new __VLS_37({}));
const __VLS_39 = __VLS_38({}, ...__VLS_functionalComponentArgsRest(__VLS_38));
const { default: __VLS_42 } = __VLS_40.slots;
let __VLS_43;
/** @ts-ignore @type {typeof __VLS_components.elHeader | typeof __VLS_components.ElHeader | typeof __VLS_components.elHeader | typeof __VLS_components.ElHeader} */
elHeader;
// @ts-ignore
const __VLS_44 = __VLS_asFunctionalComponent1(__VLS_43, new __VLS_43({
    ...{ class: "shell__header" },
}));
const __VLS_45 = __VLS_44({
    ...{ class: "shell__header" },
}, ...__VLS_functionalComponentArgsRest(__VLS_44));
/** @type {__VLS_StyleScopedClasses['shell__header']} */ ;
const { default: __VLS_48 } = __VLS_46.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({});
__VLS_asFunctionalElement1(__VLS_intrinsics.h2, __VLS_intrinsics.h2)({
    ...{ class: "shell__heading" },
});
/** @type {__VLS_StyleScopedClasses['shell__heading']} */ ;
(__VLS_ctx.heading.title);
// @ts-ignore
[heading,];
var __VLS_46;
let __VLS_49;
/** @ts-ignore @type {typeof __VLS_components.elMain | typeof __VLS_components.ElMain | typeof __VLS_components.elMain | typeof __VLS_components.ElMain} */
elMain;
// @ts-ignore
const __VLS_50 = __VLS_asFunctionalComponent1(__VLS_49, new __VLS_49({
    ...{ class: "shell__main" },
}));
const __VLS_51 = __VLS_50({
    ...{ class: "shell__main" },
}, ...__VLS_functionalComponentArgsRest(__VLS_50));
/** @type {__VLS_StyleScopedClasses['shell__main']} */ ;
const { default: __VLS_54 } = __VLS_52.slots;
__VLS_asFunctionalElement1(__VLS_intrinsics.div, __VLS_intrinsics.div)({
    ...{ class: "shell__content" },
});
/** @type {__VLS_StyleScopedClasses['shell__content']} */ ;
let __VLS_55;
/** @ts-ignore @type {typeof __VLS_components.RouterView} */
RouterView;
// @ts-ignore
const __VLS_56 = __VLS_asFunctionalComponent1(__VLS_55, new __VLS_55({}));
const __VLS_57 = __VLS_56({}, ...__VLS_functionalComponentArgsRest(__VLS_56));
// @ts-ignore
[];
var __VLS_52;
// @ts-ignore
[];
var __VLS_40;
// @ts-ignore
[];
var __VLS_3;
// @ts-ignore
[];
const __VLS_export = (await import('vue')).defineComponent({});
export default {};
