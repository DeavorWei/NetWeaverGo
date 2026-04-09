package taskexec

import (
	"fmt"

	"github.com/google/uuid"
)

func newPrefixedID(prefix string) string {
	return fmt.Sprintf("%s%s", prefix, uuid.NewString())
}

func newRunID() string {
	return newPrefixedID("run_")
}

func newStageID() string {
	return newPrefixedID("stage_")
}

func newUnitID() string {
	return newPrefixedID("unit_")
}

func newEventID() string {
	return newPrefixedID("event_")
}

func newArtifactID() string {
	return newPrefixedID("artifact_")
}

func newEdgeID() string {
	return newPrefixedID("edge_")
}

func newDefinitionID() string {
	return newPrefixedID("definition_")
}

// newNodeUUID 生成全局唯一的节点UUID
// 阶段3架构演进：用于替代DeviceIP作为拓扑节点的主键
func newNodeUUID() string {
	return newPrefixedID("node_")
}
