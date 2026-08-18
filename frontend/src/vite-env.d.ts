/// <reference types="vite/client" />

declare module '*.ico' {
    const src: string
    export default src
}

declare module '*.vue' {
    import type {DefineComponent} from 'vue'
    const component: DefineComponent<{}, {}, any>
    export default component
}
