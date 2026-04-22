# 黑暗模式设置保存问题修复方案（优化版）

## 问题描述

用户反馈：每次启动应用都是黑暗模式，无论上次设置的是黑暗模式还是明亮模式。

## 问题分析

### 根本原因

1. **初始化时机过晚**：主题初始化在 Vue 组件的 `onMounted` 钩子中执行，导致页面先使用 CSS 默认样式，然后才切换到保存的主题
2. **默认主题与 CSS 不一致**：[`theme.ts:61`](frontend/src/types/theme.ts:61) 中 `DEFAULT_THEME = 'dark'`，但 CSS `:root` 默认是明亮主题

### 现有机制分析

| 组件 | 文件路径 | 作用 |
|------|----------|------|
| 主题存储键名 | [`types/theme.ts:64`](frontend/src/types/theme.ts:64) | `netweaver-theme` |
| 默认主题 | [`types/theme.ts:61`](frontend/src/types/theme.ts:61) | `'dark'` |
| 主题管理 | [`composables/useTheme.ts`](frontend/src/composables/useTheme.ts) | 提供 `setTheme`/`toggleTheme` |
| 主题切换组件 | [`components/common/ThemeSwitch.vue`](frontend/src/components/common/ThemeSwitch.vue) | UI 切换按钮 |
| CSS 变量 | [`styles/themes/_variables.css`](frontend/src/styles/themes/_variables.css) | 主题样式定义 |

## 优化后的修复方案

### 核心思路

采用**极简修复策略**：仅在 `index.html` 中添加早期初始化脚本，其他代码保持不变。

### 修改 1: index.html 添加早期主题初始化

**文件**: [`frontend/index.html`](frontend/index.html)

**修改内容**: 在 `<head>` 的 `<meta>` 标签之后、CSS 链接之前添加内联脚本：

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/x-icon" href="/logo.ico" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>NetWeaverGo</title>
    
    <!-- 早期主题初始化 - 防止页面闪烁 -->
    <script>
      (function() {
        const THEME_KEY = 'netweaver-theme';
        const savedTheme = localStorage.getItem(THEME_KEY);
        
        let theme;
        if (savedTheme === 'light' || savedTheme === 'dark') {
          // 已保存的有效主题
          theme = savedTheme;
        } else {
          // 无保存主题时，检测系统偏好（与 useTheme.ts 的 initTheme 逻辑保持一致）
          const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
          theme = prefersDark ? 'dark' : 'light';
        }
        
        // 立即应用到 DOM，在 CSS 渲染前执行
        const html = document.documentElement;
        html.setAttribute('data-theme', theme);
        if (theme === 'dark') {
          html.classList.add('dark');
        }
      })();
    </script>
    
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link
      href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap"
      rel="stylesheet"
    />
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

**作用**:
- 页面加载时**立即**应用主题，避免闪烁
- 确保 CSS 变量在首次渲染前已正确设置
- 与 `useTheme.ts` 中的 `initTheme()` 逻辑完全一致：优先已保存主题 → 其次系统偏好 → 最后默认黑暗

### 为什么不需要其他修改

| 原方案建议 | 不采纳原因 |
|------------|------------|
| 修改 CSS 选择器 | 当前 CSS 设计合理，`:root` 作为明亮主题兜底是正确的 |
| 将初始化移到模块顶层 | 可能引起 SSR/构建环境问题，`onMounted` 是 Vue 推荐的做法 |
| ~~检测系统偏好~~ | 已在早期脚本中实现，与 `useTheme.ts` 的 `initTheme()` 逻辑对齐 |
| 大量调试日志 | 代码逻辑清晰，不需要额外日志 |

## 实施步骤

1. **修改 `frontend/index.html`**
   - 在 `<head>` 中添加早期初始化脚本
   - 确保脚本在 CSS 链接之前

2. **验证修复效果**
   - 切换到明亮模式，刷新页面，应无闪烁且保持明亮模式
   - 切换到黑暗模式，刷新页面，应无闪烁且保持黑暗模式
   - 清除 localStorage 后刷新，应根据系统偏好显示对应主题（系统暗色→黑暗模式，系统亮色→明亮模式）

## 测试验证方案

### 场景 1: 首次访问（无保存主题）
1. 清除 localStorage：`localStorage.removeItem('netweaver-theme')`
2. 刷新页面
3. **预期**: 跟随系统偏好——系统为暗色时显示黑暗模式，系统为亮色时显示明亮模式（与 `useTheme.ts` 的 `initTheme()` 逻辑一致）

### 场景 2: 切换到明亮并保存
1. 点击主题切换按钮切换到明亮模式
2. 刷新页面
3. **预期**: 页面立即显示明亮模式（无闪烁）

### 场景 3: 切换到黑暗并保存
1. 点击主题切换按钮切换到黑暗模式
2. 刷新页面
3. **预期**: 页面立即显示黑暗模式（无闪烁）

### 场景 4: 跨会话保持
1. 设置明亮模式
2. 关闭浏览器标签页
3. 重新打开应用
4. **预期**: 保持明亮模式

## 调试方法

在浏览器开发者工具控制台中：

```javascript
// 检查当前保存的主题
console.log(localStorage.getItem('netweaver-theme'))

// 手动设置为明亮模式
localStorage.setItem('netweaver-theme', 'light')

// 手动设置为黑暗模式
localStorage.setItem('netweaver-theme', 'dark')

// 清除主题设置（恢复默认）
localStorage.removeItem('netweaver-theme')
```

## 预期效果

- ✅ 页面加载时立即应用正确主题，**无闪烁**
- ✅ 用户切换的主题在下次启动时正确恢复
- ✅ 首次访问跟随系统偏好，无保存主题时与 `useTheme.ts` 的 `initTheme()` 行为一致
- ✅ 代码改动最小化，风险最低

## 方案对比

| 方案 | 代码改动量 | 闪烁问题 | 复杂度 | 推荐度 |
|------|------------|----------|--------|--------|
| 本方案（极简） | 1 个文件，~20 行 | 解决 | 低 | ⭐⭐⭐⭐⭐ |
| 原方案（全面） | 3 个文件，多处修改 | 解决 | 高 | ⭐⭐⭐ |

## 相关文件

- [`frontend/index.html`](frontend/index.html) - 需要修改
- [`frontend/src/composables/useTheme.ts`](frontend/src/composables/useTheme.ts) - 无需修改
- [`frontend/src/types/theme.ts`](frontend/src/types/theme.ts) - 无需修改
- [`frontend/src/styles/themes/_variables.css`](frontend/src/styles/themes/_variables.css) - 无需修改
- [`frontend/src/components/common/ThemeSwitch.vue`](frontend/src/components/common/ThemeSwitch.vue) - 无需修改
