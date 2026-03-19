// Package main 提供设备凭据迁移工具
// 用于将数据库中的明文密码加密存储
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

func main() {
	// 解析命令行参数
	storageRoot := flag.String("storage", "", "存储根目录路径（默认使用当前配置）")
	dryRun := flag.Bool("dry-run", false, "仅检查不执行迁移")
	flag.Parse()

	// 初始化配置
	if *storageRoot != "" {
		absPath, err := filepath.Abs(*storageRoot)
		if err != nil {
			log.Fatalf("解析存储路径失败: %v", err)
		}
		config.NormalizeStorageRoot(absPath)
	}

	// 初始化数据库
	if err := config.InitDB(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 初始化日志
	logPath := config.GetPathManager().GetAppLogPath()
	if err := logger.InitGlobalLogger(logPath); err != nil {
		log.Printf("警告: 初始化日志失败: %v", err)
	}

	// 获取加密器
	cipher := config.GetCredentialCipher()

	// 查询所有设备
	var devices []models.DeviceAsset
	if err := config.DB.Find(&devices).Error; err != nil {
		log.Fatalf("查询设备失败: %v", err)
	}

	fmt.Printf("共发现 %d 台设备\n", len(devices))

	// 统计
	var (
		emptyCount    int
		encryptedCnt  int
		plaintextCnt  int
		migratedCount int
		errorCount    int
	)

	for _, d := range devices {
		if d.Password == "" {
			emptyCount++
			continue
		}

		// 检查是否已是密文
		if config.IsEncrypted(d.Password) {
			encryptedCnt++
			continue
		}

		plaintextCnt++
		fmt.Printf("设备 %s: 发现明文密码\n", d.IP)

		if *dryRun {
			fmt.Printf("  [DRY-RUN] 将加密密码\n")
			continue
		}

		// 加密密码
		encrypted, err := cipher.Encrypt(d.Password)
		if err != nil {
			log.Printf("  加密设备 %s 失败: %v", d.IP, err)
			errorCount++
			continue
		}

		// 直接更新数据库（跳过 GORM 回调）
		if err := config.DB.Model(&models.DeviceAsset{}).
			Where("id = ?", d.ID).
			Update("password", encrypted).Error; err != nil {
			log.Printf("  更新设备 %s 失败: %v", d.IP, err)
			errorCount++
			continue
		}

		migratedCount++
		fmt.Printf("  已加密\n")
	}

	// 输出统计
	fmt.Println("\n========== 迁移统计 ==========")
	fmt.Printf("总设备数: %d\n", len(devices))
	fmt.Printf("空密码: %d\n", emptyCount)
	fmt.Printf("已加密: %d\n", encryptedCnt)
	fmt.Printf("明文密码: %d\n", plaintextCnt)

	if *dryRun {
		fmt.Println("\n[DRY-RUN 模式] 未执行实际迁移")
	} else {
		fmt.Printf("成功迁移: %d\n", migratedCount)
		fmt.Printf("失败: %d\n", errorCount)
	}

	if errorCount > 0 {
		os.Exit(1)
	}
}
