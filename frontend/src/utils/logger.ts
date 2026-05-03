/**
 * 前端日志工具类
 * @description 提供统一的日志记录接口，自动发送到后端持久化
 */

import { FrontendLogEntry as BindingFrontendLogEntry } from '@/bindings/github.com/NetWeaverGo/core/internal/logger/models'

/** 日志级别 */
export type LogLevel = 'error' | 'warn' | 'info' | 'debug'

/** 前端日志条目 (与后端 FrontendLogEntry 结构匹配) */
export interface FrontendLogEntry {
  /** 日志级别: error, warn, info, debug */
  level: LogLevel
  /** 日志消息 */
  message: string
  /** ISO 8601 时间戳 */
  timestamp: string
  /** 来源模块 */
  module: string
  /** 堆栈信息 (错误时提供) */
  stack?: string
  /** 当前页面 URL */
  url?: string
  /** 用户代理 */
  userAgent?: string
}

/**
 * 将前端日志条目转换为绑定类型
 */
function toBindingEntry(entry: FrontendLogEntry): BindingFrontendLogEntry {
  const bindingEntry = new BindingFrontendLogEntry()
  bindingEntry.level = entry.level
  bindingEntry.message = entry.message
  bindingEntry.timestamp = entry.timestamp
  bindingEntry.module = entry.module
  if (entry.stack !== undefined) {
    bindingEntry.stack = entry.stack
  }
  if (entry.url !== undefined) {
    bindingEntry.url = entry.url
  }
  if (entry.userAgent !== undefined) {
    bindingEntry.userAgent = entry.userAgent
  }
  return bindingEntry
}

/** Logger 配置 */
export interface LoggerConfig {
  /** 是否启用后端日志 (默认 true) */
  enableBackendLog: boolean
  /** 是否启用控制台输出 (默认 true) */
  enableConsole: boolean
  /** 批量发送缓冲区大小 (默认 50) */
  batchSize: number
  /** 批量发送间隔 (毫秒, 默认 2000) */
  batchInterval: number
  /** 是否捕获全局错误 (默认 true) */
  captureGlobalErrors: boolean
}

/** 默认配置 */
const defaultConfig: LoggerConfig = {
  enableBackendLog: true,
  enableConsole: true,
  batchSize: 50,
  batchInterval: 2000,
  captureGlobalErrors: true,
}

/**
 * 前端日志记录器类
 * @description 提供日志缓冲、批量发送、全局错误捕获等功能
 */
export class FrontendLogger {
  private config: LoggerConfig
  private buffer: FrontendLogEntry[] = []
  private batchTimer: ReturnType<typeof setTimeout> | null = null
  private flushPromise: Promise<void> | null = null
  private isShuttingDown = false

  constructor(config: Partial<LoggerConfig> = {}) {
    this.config = { ...defaultConfig, ...config }
  }

  /**
   * 配置 Logger
   */
  configure(newConfig: Partial<LoggerConfig>): void {
    this.config = { ...this.config, ...newConfig }
  }

  /**
   * 获取当前配置
   */
  getConfig(): Readonly<LoggerConfig> {
    return { ...this.config }
  }

  /**
   * 信息日志
   */
  info(message: string, module = 'App'): void {
    this.log('info', message, module)
  }

  /**
   * 警告日志
   */
  warn(message: string, module = 'App'): void {
    this.log('warn', message, module)
  }

  /**
   * 错误日志
   * @param message 错误消息
   * @param module 来源模块
   * @param err 错误对象 (可选)
   */
  error(message: string, module = 'App', err?: Error | unknown): void {
    const stack = this.extractStack(err)
    this.log('error', message, module, stack)
  }

  /**
   * 调试日志
   */
  debug(message: string, module = 'App'): void {
    this.log('debug', message, module)
  }

  /**
   * 核心日志方法
   */
  private log(level: LogLevel, message: string, module: string, stack?: string): void {
    const entry = this.createEntry(level, message, module, stack)

    // 控制台输出
    this.writeToConsole(entry)

    // error 级别立即发送，其他级别进入缓冲队列
    if (level === 'error') {
      this.sendToBackend(entry)
    } else {
      this.addToBatch(entry)
    }
  }

  /**
   * 创建日志条目
   */
  private createEntry(level: LogLevel, message: string, module: string, stack?: string): FrontendLogEntry {
    return {
      level,
      message,
      timestamp: new Date().toISOString(),
      module,
      stack,
      url: window.location.href,
      userAgent: navigator.userAgent,
    }
  }

  /**
   * 提取错误堆栈
   */
  private extractStack(err: Error | unknown): string | undefined {
    if (err instanceof Error) {
      return err.stack
    }
    return undefined
  }

  /**
   * 输出到控制台
   */
  private writeToConsole(entry: FrontendLogEntry): void {
    if (!this.config.enableConsole) return

    const prefix = `[${entry.timestamp}] [${entry.level.toUpperCase()}] [${entry.module}]`
    const method = entry.level === 'error' ? 'error' :
                   entry.level === 'warn' ? 'warn' :
                   entry.level === 'debug' ? 'debug' : 'log'

    console[method](prefix, entry.message)
    if (entry.stack) {
      console[method]('Stack:', entry.stack)
    }
  }

  /**
   * 添加到批量缓冲区
   */
  private addToBatch(entry: FrontendLogEntry): void {
    if (!this.config.enableBackendLog) return

    this.buffer.push(entry)

    // 达到批次大小时立即发送
    if (this.buffer.length >= this.config.batchSize) {
      this.flushBatch()
    } else {
      // 启动定时器
      this.scheduleBatchFlush()
    }
  }

  /**
   * 安排批量刷新
   */
  private scheduleBatchFlush(): void {
    if (this.batchTimer === null) {
      this.batchTimer = setTimeout(() => {
        this.flushBatch()
      }, this.config.batchInterval)
    }
  }

  /**
   * 刷新批量缓冲区
   */
  private flushBatch(): void {
    if (this.batchTimer !== null) {
      clearTimeout(this.batchTimer)
      this.batchTimer = null
    }

    if (this.buffer.length === 0) return

    const entries = [...this.buffer]
    this.buffer = []

    this.sendBatchToBackend(entries)
  }

  /**
   * 发送单条日志到后端
   */
  private async sendToBackend(entry: FrontendLogEntry): Promise<void> {
    if (!this.config.enableBackendLog) return

    try {
      // 动态导入 Wails 绑定
      const { Log } = await import(
        '@/bindings/github.com/NetWeaverGo/core/internal/ui/frontendlogservice'
      )
      await Log(toBindingEntry(entry))
    } catch {
      // 后端调用失败，仅输出到控制台
      if (this.config.enableConsole) {
        console.error('[Logger] 后端日志发送失败:', entry.message)
      }
    }
  }

  /**
   * 批量发送日志到后端
   */
  private async sendBatchToBackend(entries: FrontendLogEntry[]): Promise<void> {
    if (!this.config.enableBackendLog || entries.length === 0) return

    try {
      // 动态导入 Wails 绑定
      const { LogBatch } = await import(
        '@/bindings/github.com/NetWeaverGo/core/internal/ui/frontendlogservice'
      )
      await LogBatch(entries.map(toBindingEntry))
    } catch {
      // 后端调用失败，仅输出到控制台
      if (this.config.enableConsole) {
        console.error('[Logger] 批量日志发送失败，条数:', entries.length)
      }
    }
  }

  /**
   * 强制刷新缓冲区 (包括后端)
   */
  async flush(): Promise<void> {
    // 先刷新前端缓冲区
    this.flushBatch()

    // 等待当前刷新完成
    if (this.flushPromise) {
      await this.flushPromise
    }

    // 刷新后端缓冲区
    if (this.config.enableBackendLog) {
      try {
        const { Flush } = await import(
          '@/bindings/github.com/NetWeaverGo/core/internal/ui/frontendlogservice'
        )
        await Flush()
      } catch {
        // 忽略刷新失败
      }
    }
  }

  /**
   * 初始化全局错误捕获
   */
  setupGlobalErrorCapture(): void {
    if (!this.config.captureGlobalErrors) return

    // 捕获 JavaScript 运行时错误
    window.onerror = (message, _source, _lineno, _colno, err) => {
      // stack 信息会在 error 方法内部通过 extractStack 提取
      this.error(String(message), 'GlobalError', err || new Error(String(message)))
      return false
    }

    // 捕获未处理的 Promise 拒绝
    window.addEventListener('unhandledrejection', (event) => {
      const err = event.reason instanceof Error
        ? event.reason
        : new Error(String(event.reason))
      this.error(`Unhandled Promise Rejection: ${err.message}`, 'Promise', err)
    })

    // 页面卸载前刷新日志
    window.addEventListener('beforeunload', () => {
      this.handleShutdown()
    })

    // 页面隐藏时刷新日志 (移动端场景)
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'hidden') {
        this.handleShutdown()
      }
    })
  }

  /**
   * 处理关闭逻辑
   */
  private handleShutdown(): void {
    if (this.isShuttingDown) return
    this.isShuttingDown = true

    // 同步刷新缓冲区 (beforeunload 中不能使用异步操作)
    this.flushBatch()

    // 尝试使用 sendBeacon 发送 (如果可用)
    this.trySendBeacon()
  }

  /**
   * 尝试使用 sendBeacon 发送剩余日志
   */
  private trySendBeacon(): void {
    // Wails 环境下 sendBeacon 可能不可用，这里仅作为备用方案
    // 实际日志已在 flushBatch 中处理
  }

  /**
   * 创建模块专用 Logger
   */
  createModuleLogger(module: string): ModuleLogger {
    return new ModuleLogger(this, module)
  }
}

/**
 * 模块专用 Logger
 */
export class ModuleLogger {
  private logger: FrontendLogger
  private module: string

  constructor(logger: FrontendLogger, module: string) {
    this.logger = logger
    this.module = module
  }

  info(message: string): void {
    this.logger.info(message, this.module)
  }

  warn(message: string): void {
    this.logger.warn(message, this.module)
  }

  error(message: string, err?: Error | unknown): void {
    this.logger.error(message, this.module, err)
  }

  debug(message: string): void {
    this.logger.debug(message, this.module)
  }
}

// ==================== 单例实例 ====================

/** 全局 Logger 实例 */
let globalLogger: FrontendLogger | null = null

/**
 * 获取全局 Logger 实例
 */
export function getLogger(): FrontendLogger {
  if (!globalLogger) {
    globalLogger = new FrontendLogger()
  }
  return globalLogger
}

/**
 * 配置全局 Logger
 */
export function configureLogger(config: Partial<LoggerConfig>): void {
  const logger = getLogger()
  logger.configure(config)
}

/**
 * 初始化全局错误捕获
 */
export function setupGlobalErrorCapture(): void {
  const logger = getLogger()
  logger.setupGlobalErrorCapture()
}

/**
 * 创建模块专用 Logger
 */
export function createModuleLogger(module: string): ModuleLogger {
  return getLogger().createModuleLogger(module)
}

// ==================== 便捷导出 ====================

/** 默认导出：全局 Logger 实例的便捷方法 */
const logger = {
  info: (message: string, module?: string) => getLogger().info(message, module),
  warn: (message: string, module?: string) => getLogger().warn(message, module),
  error: (message: string, module?: string, err?: Error | unknown) => getLogger().error(message, module, err),
  debug: (message: string, module?: string) => getLogger().debug(message, module),
  flush: () => getLogger().flush(),
  configure: (config: Partial<LoggerConfig>) => configureLogger(config),
  setupGlobalErrorCapture: () => setupGlobalErrorCapture(),
  createModuleLogger: (module: string) => createModuleLogger(module),
}

export default logger
