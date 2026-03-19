package topology

import (
	"sort"
	"strings"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/normalize"
)

// IdentityResolver 负责设备身份归并。
type IdentityResolver struct {
	bySerial  map[string]map[string]struct{}
	byMgmtIP  map[string]map[string]struct{}
	byChassis map[string]map[string]struct{}
	byName    map[string]map[string]struct{}
}

func NewIdentityResolver(devices []models.DiscoveryDevice) *IdentityResolver {
	r := &IdentityResolver{
		bySerial:  make(map[string]map[string]struct{}),
		byMgmtIP:  make(map[string]map[string]struct{}),
		byChassis: make(map[string]map[string]struct{}),
		byName:    make(map[string]map[string]struct{}),
	}

	for _, d := range devices {
		deviceID := strings.TrimSpace(d.DeviceIP)
		if deviceID == "" {
			continue
		}

		addIndex(r.bySerial, normalizeValue(d.SerialNumber), deviceID)
		addIndex(r.byMgmtIP, normalizeValue(chooseNonEmpty(d.MgmtIP, d.DeviceIP)), deviceID)
		addIndex(r.byChassis, normalizeValue(d.ChassisID), deviceID)

		name := chooseNonEmpty(d.NormalizedName, d.DisplayName, d.Hostname)
		addIndex(r.byName, normalizeValue(normalize.NormalizeDeviceName(name)), deviceID)
	}
	return r
}

func addIndex(index map[string]map[string]struct{}, key, deviceID string) {
	if key == "" {
		return
	}
	if _, ok := index[key]; !ok {
		index[key] = make(map[string]struct{})
	}
	index[key][deviceID] = struct{}{}
}

func normalizeValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value)
}

// Resolve 通过显式标识归并设备:
// Serial > MgmtIP > ChassisID > NormalizedName
//
// 规则:
// 1. 选择首个命中的优先级作为主候选。
// 2. 若主候选存在多个设备，直接冲突。
// 3. 若低优先级命中且与主候选不一致，记为冲突（多标识符冲突）。
func (r *IdentityResolver) Resolve(serial, mgmtIP, chassisID, normalizedName string) (deviceID string, conflict bool, candidates []string) {
	groups := []map[string]map[string]struct{}{
		r.bySerial,
		r.byMgmtIP,
		r.byChassis,
		r.byName,
	}
	keys := []string{
		normalizeValue(serial),
		normalizeValue(mgmtIP),
		normalizeValue(chassisID),
		normalizeValue(normalizedName),
	}

	primarySet := make(map[string]struct{})
	seen := make(map[string]struct{})
	ordered := make([]string, 0, 4)
	primaryLevel := -1

	for i, key := range keys {
		if key == "" {
			continue
		}
		candSet, ok := groups[i][key]
		if !ok || len(candSet) == 0 {
			continue
		}
		levelCandidates := setToSortedSlice(candSet)
		if primaryLevel == -1 {
			primaryLevel = i
			for _, c := range levelCandidates {
				primarySet[c] = struct{}{}
			}
		} else if i > primaryLevel {
			for _, c := range levelCandidates {
				if _, ok := primarySet[c]; !ok {
					conflict = true
				}
			}
		}

		for _, c := range levelCandidates {
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			ordered = append(ordered, c)
		}
	}

	if len(primarySet) == 0 {
		return "", false, nil
	}
	if len(primarySet) > 1 {
		conflict = true
	}

	primary := setToSortedSlice(primarySet)
	selected := primary[0]
	return selected, conflict, ordered
}

// ResolveLLDPNeighbor 按优先级归并设备身份。
func (r *IdentityResolver) ResolveLLDPNeighbor(neighbor models.TopologyLLDPNeighbor) (deviceID string, conflict bool, candidates []string) {
	normalizedName := normalize.NormalizeDeviceName(neighbor.NeighborName)
	return r.Resolve("", neighbor.NeighborIP, neighbor.NeighborChassis, normalizedName)
}

func setToSortedSlice(set map[string]struct{}) []string {
	result := make([]string, 0, len(set))
	for deviceID := range set {
		result = append(result, deviceID)
	}
	sort.Strings(result)
	return result
}
