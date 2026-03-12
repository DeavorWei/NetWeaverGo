#!/usr/bin/env node
/**
 * API 迁移脚本
 * 自动将旧 API 调用转换为新命名空间 API
 *
 * 使用方法：
 *   node scripts/migrate-api.js <file-or-directory>
 *
 * 示例：
 *   node scripts/migrate-api.js frontend/src/views/Settings.vue
 *   node scripts/migrate-api.js frontend/src
 */

const fs = require("fs");
const path = require("path");

// API 映射表：旧 API 名称 → [命名空间, 新方法名]
const API_MAPPING = {
  // Device API
  ListDevices: ["DeviceAPI", "listDevices"],
  AddDevice: ["DeviceAPI", "addDevice"],
  UpdateDevice: ["DeviceAPI", "updateDevice"],
  DeleteDevice: ["DeviceAPI", "deleteDevice"],
  SaveDevices: ["DeviceAPI", "saveDevices"],
  GetProtocolDefaultPorts: ["DeviceAPI", "getProtocolDefaultPorts"],
  GetValidProtocols: ["DeviceAPI", "getValidProtocols"],
  // CommandGroup API
  ListCommandGroups: ["CommandGroupAPI", "listCommandGroups"],
  GetCommandGroup: ["CommandGroupAPI", "getCommandGroup"],
  CreateCommandGroup: ["CommandGroupAPI", "createCommandGroup"],
  UpdateCommandGroup: ["CommandGroupAPI", "updateCommandGroup"],
  DeleteCommandGroup: ["CommandGroupAPI", "deleteCommandGroup"],
  DuplicateCommandGroup: ["CommandGroupAPI", "duplicateCommandGroup"],
  ImportCommandGroup: ["CommandGroupAPI", "importCommandGroup"],
  ExportCommandGroup: ["CommandGroupAPI", "exportCommandGroup"],
  GetCommands: ["CommandGroupAPI", "getCommands"],
  SaveCommands: ["CommandGroupAPI", "saveCommands"],
  // Settings API
  LoadSettings: ["SettingsAPI", "loadSettings"],
  SaveSettings: ["SettingsAPI", "saveSettings"],
  EnsureConfig: ["SettingsAPI", "ensureConfig"],
  GetAppInfo: ["SettingsAPI", "getAppInfo"],
  LogInfo: ["SettingsAPI", "logInfo"],
  LogWarn: ["SettingsAPI", "logWarn"],
  LogError: ["SettingsAPI", "logError"],
  // Engine API
  StartEngine: ["EngineAPI", "startEngine"],
  StartEngineWithSelection: ["EngineAPI", "startEngineWithSelection"],
  StartBackup: ["EngineAPI", "startBackup"],
  ResolveSuspend: ["EngineAPI", "resolveSuspend"],
  IsRunning: ["EngineAPI", "isRunning"],
  // TaskGroup API
  ListTaskGroups: ["TaskGroupAPI", "listTaskGroups"],
  GetTaskGroup: ["TaskGroupAPI", "getTaskGroup"],
  CreateTaskGroup: ["TaskGroupAPI", "createTaskGroup"],
  UpdateTaskGroup: ["TaskGroupAPI", "updateTaskGroup"],
  DeleteTaskGroup: ["TaskGroupAPI", "deleteTaskGroup"],
  StartTaskGroup: ["TaskGroupAPI", "startTaskGroup"],
};

// 需要导入的命名空间集合
let requiredNamespaces = new Set();

function migrateFile(filePath) {
  let content = fs.readFileSync(filePath, "utf-8");
  let originalContent = content;
  let hasChanges = false;

  // 创建备份
  const backupPath = `${filePath}.backup-${Date.now()}`;
  try {
    fs.writeFileSync(backupPath, content);
    console.log(`  📦 已备份: ${path.basename(backupPath)}`);
  } catch (err) {
    console.error(`  ⚠️ 备份失败: ${filePath}`, err.message);
    return false;
  }

  // 1. 处理从 api.ts 导入的旧 API
  const oldApiImportRegex =
    /import\s*\{\s*([^}]+)\s*\}\s*from\s*['"]@\/services\/api['"]/g;

  content = content.replace(oldApiImportRegex, (match, imports) => {
    const importList = imports.split(",").map((s) => s.trim());
    const newImports = [];
    const apisToMigrate = [];

    importList.forEach((imp) => {
      const apiName = imp.split(" as ")[0].trim();
      if (API_MAPPING[apiName]) {
        apisToMigrate.push(apiName);
        requiredNamespaces.add(API_MAPPING[apiName][0]);
        hasChanges = true;
      } else {
        newImports.push(imp);
      }
    });

    if (apisToMigrate.length === 0) return match;

    // 转换 API 调用
    apisToMigrate.forEach((oldApi) => {
      const [namespace, newMethod] = API_MAPPING[oldApi];
      // 替换函数调用：OldApi(...) → Namespace.newMethod(...)
      const callRegex = new RegExp(`\\b${oldApi}\\s*\\(`, "g");
      content = content.replace(callRegex, `${namespace}.${newMethod}(`);
    });

    if (newImports.length > 0) {
      return `import { ${newImports.join(", ")} } from '@/services/api'`;
    }
    return ""; // 完全移除导入行
  });

  // 2. 处理直接从 bindings 导入的情况（如 Settings.vue）
  const bindingImportRegex =
    /import\s*\{\s*([^}]+)\s*\}\s*from\s*['"]\.\.\/bindings\/github\.com\/NetWeaverGo\/core\/internal\/ui\/(\w+)service['"]/g;

  content = content.replace(
    bindingImportRegex,
    (match, imports, serviceName) => {
      const importList = imports.split(",").map((s) => s.trim());
      const namespaceMap = {
        deviceservice: "DeviceAPI",
        commandgroupservice: "CommandGroupAPI",
        settingsservice: "SettingsAPI",
        engineservice: "EngineAPI",
        taskgroupservice: "TaskGroupAPI",
      };

      const namespace = namespaceMap[serviceName.toLowerCase()];
      if (!namespace) return match;

      const apisToMigrate = [];
      importList.forEach((imp) => {
        const apiName = imp.split(" as ")[0].trim();
        const mapping = Object.entries(API_MAPPING).find(
          ([old, [ns, newName]]) =>
            ns === namespace && old.toLowerCase() === apiName.toLowerCase(),
        );
        if (mapping) {
          apisToMigrate.push({ old: apiName, new: mapping[1] });
          requiredNamespaces.add(namespace);
          hasChanges = true;
        }
      });

      if (apisToMigrate.length === 0) return match;

      // 替换函数调用
      apisToMigrate.forEach(({ old, new: newMethod }) => {
        const callRegex = new RegExp(`\\b${old}\\s*\\(`, "g");
        content = content.replace(callRegex, `${namespace}.${newMethod}(`);
      });

      return ""; // 移除 binding 导入
    },
  );

  // 3. 添加新的命名空间导入
  if (requiredNamespaces.size > 0) {
    const namespaces = Array.from(requiredNamespaces).sort();
    const newImportLine = `import { ${namespaces.join(
      ", ",
    )} } from '@/services/api'\n`;

    // 查找现有的 api.ts 导入
    const existingImportRegex =
      /import\s*\{[^}]*\}\s*from\s*['"]@\/services\/api['"]\n?/;
    if (!existingImportRegex.test(content)) {
      // 在文件开头添加导入
      content = newImportLine + content;
    } else {
      // 合并到现有导入
      content = content.replace(existingImportRegex, (match) => {
        const existingMatch = match.match(/\{([^}]+)\}/);
        if (!existingMatch) return newImportLine;
        const existing = existingMatch[1];
        const allImports = [
          ...new Set([
            ...existing.split(",").map((s) => s.trim()),
            ...namespaces,
          ]),
        ].sort();
        return `import { ${allImports.join(", ")} } from '@/services/api'\n`;
      });
    }
  }

  // 4. 清理空行
  content = content.replace(/\n{3,}/g, "\n\n");

  if (hasChanges) {
    fs.writeFileSync(filePath, content, "utf-8");
    console.log(`✅ 已迁移: ${filePath}`);
    return true;
  }
  return false;
}

function processPath(targetPath) {
  const stats = fs.statSync(targetPath);

  if (stats.isFile() && /\.(ts|vue)$/.test(targetPath)) {
    // 重置命名空间集合
    requiredNamespaces = new Set();
    return migrateFile(targetPath) ? 1 : 0;
  }

  if (stats.isDirectory()) {
    let count = 0;
    const files = fs.readdirSync(targetPath);
    files.forEach((file) => {
      const fullPath = path.join(targetPath, file);
      count += processPath(fullPath);
    });
    return count;
  }

  return 0;
}

// 主程序
const target = process.argv[2] || "frontend/src";
const absolutePath = path.resolve(target);

if (!fs.existsSync(absolutePath)) {
  console.error(`❌ 路径不存在: ${absolutePath}`);
  process.exit(1);
}

console.log(`🚀 开始迁移 API: ${absolutePath}\n`);
const migratedCount = processPath(absolutePath);
console.log(`\n✨ 完成! 共迁移 ${migratedCount} 个文件`);
