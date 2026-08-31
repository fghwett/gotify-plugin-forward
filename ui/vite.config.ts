import { defineConfig } from 'vite'
import solid from 'vite-plugin-solid'

export default defineConfig({
  // 相对路径加载资源，适配 gotify 动态分配的插件路径前缀
  base: './',
  plugins: [solid()],
  build: {
    outDir: '../app/web',
    emptyOutDir: true,
  },
})
