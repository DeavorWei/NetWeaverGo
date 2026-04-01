package ui

import (
	"strings"
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
)

func buildTestDevice(ip string) models.DeviceAsset {
	return models.DeviceAsset{
		IP:       ip,
		Port:     22,
		Protocol: "SSH",
		Username: "admin",
		Password: "admin",
	}
}

func TestDeviceServiceAddDevices_ExpandRangeBeforeValidation(t *testing.T) {
	repo := repository.NewMockDeviceRepository()
	svc := NewDeviceServiceWithRepo(repo)

	err := svc.AddDevices([]models.DeviceAsset{buildTestDevice("192.168.58.201-202")})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("find all failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(all))
	}
	if all[0].IP != "192.168.58.201" || all[1].IP != "192.168.58.202" {
		t.Fatalf("unexpected expanded ips: %#v %#v", all[0].IP, all[1].IP)
	}
}

func TestDeviceServiceAddDevices_ExpandRangeWithTilde(t *testing.T) {
	repo := repository.NewMockDeviceRepository()
	svc := NewDeviceServiceWithRepo(repo)

	err := svc.AddDevices([]models.DeviceAsset{buildTestDevice("192.168.58.201~202")})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("find all failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(all))
	}
}

func TestDeviceServiceAddDevices_InvalidRangeFormat(t *testing.T) {
	repo := repository.NewMockDeviceRepository()
	svc := NewDeviceServiceWithRepo(repo)

	err := svc.AddDevices([]models.DeviceAsset{buildTestDevice("192.168.58.201-abc")})
	if err == nil {
		t.Fatal("expected error for invalid range format, got nil")
	}
	if !strings.Contains(err.Error(), "无法识别IP范围格式") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeviceServiceAddDevices_DescendingRangeRejected(t *testing.T) {
	repo := repository.NewMockDeviceRepository()
	svc := NewDeviceServiceWithRepo(repo)

	err := svc.AddDevices([]models.DeviceAsset{buildTestDevice("192.168.58.202-201")})
	if err == nil {
		t.Fatal("expected error for descending range, got nil")
	}
	if !strings.Contains(err.Error(), "起始值") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeviceServiceAddDevices_DuplicateAfterExpansion(t *testing.T) {
	repo := repository.NewMockDeviceRepository()
	svc := NewDeviceServiceWithRepo(repo)

	err := svc.AddDevices([]models.DeviceAsset{
		buildTestDevice("192.168.58.201-202"),
		buildTestDevice("192.168.58.202"),
	})
	if err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
	if !strings.Contains(err.Error(), "重复") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeviceServiceAddDevices_DBConflict(t *testing.T) {
	repo := repository.NewMockDeviceRepository()
	repo.AddDevice(buildTestDevice("192.168.58.202"))
	svc := NewDeviceServiceWithRepo(repo)

	err := svc.AddDevices([]models.DeviceAsset{buildTestDevice("192.168.58.201-202")})
	if err == nil {
		t.Fatal("expected db conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("unexpected error: %v", err)
	}
}
