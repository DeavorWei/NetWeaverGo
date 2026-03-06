/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './src/**/*.{vue,js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        // 深色系主色调
        'bg-dark':    '#0f1117',
        'bg-panel':   '#161b27',
        'bg-card':    '#1a2236',
        'bg-hover':   '#1e2a3f',
        'border':     '#2a3a5c',
        'accent':     '#3b82f6',
        'accent-glow':'#60a5fa',
        'success':    '#22c55e',
        'warning':    '#f59e0b',
        'error':      '#ef4444',
        'text-primary':'#e2e8f0',
        'text-secondary':'#94a3b8',
        'text-muted':  '#64748b',
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
