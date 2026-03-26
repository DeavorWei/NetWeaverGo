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
