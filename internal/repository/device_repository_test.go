package repository

import (
	"fmt"
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
)

func TestMockDeviceRepository_FindAll(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1", Protocol: "SSH"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.2", Protocol: "SNMP"})

	devices, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll failed: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(devices))
	}
}

func TestMockDeviceRepository_FindByID(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1", Protocol: "SSH"})

	// 存在的设备
	device, err := repo.FindByID(1)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if device.IP != "192.168.1.1" {
		t.Errorf("Expected IP 192.168.1.1, got %s", device.IP)
	}

	// 不存在的设备
	_, err = repo.FindByID(999)
	if err != gorm.ErrRecordNotFound {
		t.Errorf("Expected ErrRecordNotFound, got %v", err)
	}
}

func TestMockDeviceRepository_FindByIP(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1", Protocol: "SSH"})

	// 存在的设备
	device, err := repo.FindByIP("192.168.1.1")
	if err != nil {
		t.Fatalf("FindByIP failed: %v", err)
	}
	if device.IP != "192.168.1.1" {
		t.Errorf("Expected IP 192.168.1.1, got %s", device.IP)
	}

	// 不存在的设备
	_, err = repo.FindByIP("192.168.1.999")
	if err != gorm.ErrRecordNotFound {
		t.Errorf("Expected ErrRecordNotFound, got %v", err)
	}
}

func TestMockDeviceRepository_Create(t *testing.T) {
	repo := NewMockDeviceRepository()

	device := &models.DeviceAsset{IP: "192.168.1.1", Protocol: "SSH"}
	err := repo.Create(device)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if device.ID == 0 {
		t.Error("Expected device ID to be set")
	}

	// 验证创建成功
	found, err := repo.FindByID(device.ID)
	if err != nil {
		t.Fatalf("FindByID after Create failed: %v", err)
	}
	if found.IP != "192.168.1.1" {
		t.Errorf("Expected IP 192.168.1.1, got %s", found.IP)
	}
}

func TestMockDeviceRepository_Update(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1", Protocol: "SSH"})

	// 更新设备
	device, _ := repo.FindByID(1)
	device.Username = "admin"
	err := repo.Update(device)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// 验证更新成功
	found, err := repo.FindByID(1)
	if err != nil {
		t.Fatalf("FindByID after Update failed: %v", err)
	}
	if found.Username != "admin" {
		t.Errorf("Expected Username admin, got %s", found.Username)
	}
}

func TestMockDeviceRepository_Delete(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1", Protocol: "SSH"})

	err := repo.Delete(1)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 验证删除成功
	_, err = repo.FindByID(1)
	if err != gorm.ErrRecordNotFound {
		t.Errorf("Expected ErrRecordNotFound after delete, got %v", err)
	}
}

func TestMockDeviceRepository_Query(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1", Protocol: "SSH", Group: "group1"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.2", Protocol: "SSH", Group: "group1"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.3", Protocol: "SNMP", Group: "group2"})

	// 分页查询
	result, err := repo.Query(DeviceQueryOptions{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result.Data) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(result.Data))
	}
	if result.Total != 3 {
		t.Errorf("Expected total 3, got %d", result.Total)
	}
	if result.TotalPages != 2 {
		t.Errorf("Expected 2 total pages, got %d", result.TotalPages)
	}

	// 搜索查询
	result, err = repo.Query(DeviceQueryOptions{SearchQuery: "192.168.1.1"})
	if err != nil {
		t.Fatalf("Query with search failed: %v", err)
	}
	if len(result.Data) != 1 {
		t.Errorf("Expected 1 device, got %d", len(result.Data))
	}
	if result.Total != 1 {
		t.Errorf("Expected total 1, got %d", result.Total)
	}
}

func TestMockDeviceRepository_ExistsByIP(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1", Protocol: "SSH"})

	// 存在的 IP
	exists, err := repo.ExistsByIP("192.168.1.1")
	if err != nil {
		t.Fatalf("ExistsByIP failed: %v", err)
	}
	if !exists {
		t.Error("Expected IP to exist")
	}

	// 不存在的 IP
	exists, err = repo.ExistsByIP("192.168.1.999")
	if err != nil {
		t.Fatalf("ExistsByIP failed: %v", err)
	}
	if exists {
		t.Error("Expected IP not to exist")
	}
}

func TestMockDeviceRepository_ErrorHandling(t *testing.T) {
	repo := NewMockDeviceRepository()

	// 设置错误
	testErr := fmt.Errorf("test error")
	repo.SetQueryError(testErr)

	// 验证错误返回
	_, err := repo.FindAll()
	if err != testErr {
		t.Errorf("Expected test error, got %v", err)
	}

	// 重置
	repo.Reset()
	_, err = repo.FindAll()
	if err != nil {
		t.Errorf("Expected no error after reset, got %v", err)
	}
}

func TestMockDeviceRepository_FindByIPs(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1", Protocol: "SSH"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.2", Protocol: "SNMP"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.3", Protocol: "SSH"})

	// 查询多个 IP
	devices, err := repo.FindByIPs([]string{"192.168.1.1", "192.168.1.3"})
	if err != nil {
		t.Fatalf("FindByIPs failed: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(devices))
	}

	// 查询不存在的 IP
	devices, err = repo.FindByIPs([]string{"192.168.1.999"})
	if err != nil {
		t.Fatalf("FindByIPs failed: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("Expected 0 devices, got %d", len(devices))
	}

	// 空列表查询
	devices, err = repo.FindByIPs([]string{})
	if err != nil {
		t.Fatalf("FindByIPs failed: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("Expected 0 devices for empty list, got %d", len(devices))
	}
}

func TestMockDeviceRepository_Count(t *testing.T) {
	repo := NewMockDeviceRepository()

	// 空仓库
	count, err := repo.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	// 添加设备
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.2"})

	count, err = repo.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}

func TestMockDeviceRepository_CreateBatch(t *testing.T) {
	repo := NewMockDeviceRepository()

	devices := []models.DeviceAsset{
		{IP: "192.168.1.1", Protocol: "SSH"},
		{IP: "192.168.1.2", Protocol: "SNMP"},
	}

	err := repo.CreateBatch(devices)
	if err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	// 验证创建成功
	count, _ := repo.Count()
	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}

	// 空列表
	err = repo.CreateBatch([]models.DeviceAsset{})
	if err != nil {
		t.Fatalf("CreateBatch with empty list failed: %v", err)
	}
}

func TestMockDeviceRepository_UpdateBatch(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1", Protocol: "SSH"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.2", Protocol: "SNMP"})

	// 更新多个设备
	devices := []models.DeviceAsset{
		{ID: 1, IP: "192.168.1.1", Username: "admin1"},
		{ID: 2, IP: "192.168.1.2", Username: "admin2"},
	}

	err := repo.UpdateBatch(devices)
	if err != nil {
		t.Fatalf("UpdateBatch failed: %v", err)
	}

	// 验证更新成功
	device1, _ := repo.FindByID(1)
	if device1.Username != "admin1" {
		t.Errorf("Expected Username admin1, got %s", device1.Username)
	}

	device2, _ := repo.FindByID(2)
	if device2.Username != "admin2" {
		t.Errorf("Expected Username admin2, got %s", device2.Username)
	}

	// 更新不存在的设备
	err = repo.UpdateBatch([]models.DeviceAsset{{ID: 999, IP: "192.168.1.999"}})
	if err == nil {
		t.Error("Expected error for non-existent device")
	}
}

func TestMockDeviceRepository_DeleteBatch(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.2"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.3"})

	// 批量删除
	err := repo.DeleteBatch([]uint{1, 2})
	if err != nil {
		t.Fatalf("DeleteBatch failed: %v", err)
	}

	// 验证删除成功
	count, _ := repo.Count()
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	// 删除不存在的设备（应该返回错误）
	err = repo.DeleteBatch([]uint{999})
	if err == nil {
		t.Error("Expected error for non-existent devices")
	}

	// 空列表删除
	err = repo.DeleteBatch([]uint{})
	if err != nil {
		t.Fatalf("DeleteBatch with empty list failed: %v", err)
	}
}

func TestMockDeviceRepository_GetDistinctGroups(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1", Group: "group1"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.2", Group: "group1"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.3", Group: "group2"})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.4", Group: ""}) // 空分组

	groups, err := repo.GetDistinctGroups()
	if err != nil {
		t.Fatalf("GetDistinctGroups failed: %v", err)
	}
	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}

	// 验证排序
	if groups[0] != "group1" || groups[1] != "group2" {
		t.Errorf("Groups not sorted correctly: %v", groups)
	}
}

func TestMockDeviceRepository_GetDistinctTags(t *testing.T) {
	repo := NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.1", Tags: []string{"tag1", "tag2"}})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.2", Tags: []string{"tag2", "tag3"}})
	repo.AddDevice(models.DeviceAsset{IP: "192.168.1.3", Tags: []string{}}) // 空标签

	tags, err := repo.GetDistinctTags()
	if err != nil {
		t.Fatalf("GetDistinctTags failed: %v", err)
	}
	if len(tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(tags))
	}

	// 验证排序
	if tags[0] != "tag1" || tags[1] != "tag2" || tags[2] != "tag3" {
		t.Errorf("Tags not sorted correctly: %v", tags)
	}
}

func TestMockDeviceRepository_Transaction(t *testing.T) {
	repo := NewMockDeviceRepository()

	// WithTx 返回自身
	txRepo := repo.WithTx(nil)
	if txRepo != repo {
		t.Error("WithTx should return self for Mock")
	}

	// BeginTx 返回 nil
	tx := repo.BeginTx()
	if tx != nil {
		t.Error("BeginTx should return nil for Mock")
	}
}
