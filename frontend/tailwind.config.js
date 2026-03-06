/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './src/**/*.{vue,js,ts,jsx,tsx}',
  ],
  darkMode: 'class', // 启用基于 class 的暗黑模式切换
  theme: {
    extend: {
      colors: {
        // 使用 CSS 变量以支持主题切换
        'bg-dark':    'var(--bg-dark)',
        'bg-panel':   'var(--bg-panel)',
        'bg-card':    'var(--bg-card)',
        'bg-hover':   'var(--bg-hover)',
        'border':     'var(--border)',
        'accent':     'var(--accent)',
        'accent-glow':'var(--accent-glow)',
        'success':    'var(--success)',
        'warning':    'var(--warning)',
        'error':      'var(--error)',
        'text-primary':'var(--text-primary)',
        'text-secondary':'var(--text-secondary)',
        'text-muted':  'var(--text-muted)',
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', '"Courier New"', 'monospace'],
      },
      boxShadow: {
        'card':  '0 4px 24px rgba(0,0,0,0.4)',
        'glow':  '0 0 20px rgba(59,130,246,0.3)',
        'inner-glow': 'inset 0 1px 0 rgba(255,255,255,0.05)',
      },
      animation: {
        'fade-in':    'fadeIn 0.3s ease-out',
        'slide-in':   'slideIn 0.3s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4,0,0.6,1) infinite',
      },
      keyframes: {
        fadeIn:  { from: { opacity: '0' }, to: { opacity: '1' } },
        slideIn: { from: { opacity: '0', transform: 'translateY(8px)' }, to: { opacity: '1', transform: 'translateY(0)' } },
      },
    },
  },
  plugins: [],
}
