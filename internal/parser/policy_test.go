package parser

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/parser/xmlcfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigPolicyExecutor(t *testing.T) {
	node := &xmlcfg.ParseNode{
		ConfigPolicy: &xmlcfg.ConfigParsePolicy{
			Fields: []xmlcfg.FieldNode{
				{
					Name:  "sysName",
					Regex: `(?i)sysname\s+(?P<sysName>\S+)`,
				},
				{
					Name:  "version",
					Regex: `(?i)VRP\s+\(R\)\s+software,\s+Version\s+(?P<version>\S+)`,
					ToUpper: &xmlcfg.ToUpperNode{
						Name: "version",
					},
				},
			},
		},
	}

	echo := `
Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.170 (V200R019C00SPC500)
Copyright (C) 2000-2019 HUAWEI TECH CO., LTD.
sysname Spine-01
`

	exec := &ConfigPolicyExecutor{}
	rows, err := exec.Execute(echo, node)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	assert.Equal(t, "Spine-01", rows[0]["sysName"])
	assert.Equal(t, "5.170", rows[0]["version"])
}

func TestTableLinePolicyExecutor(t *testing.T) {
	node := &xmlcfg.ParseNode{
		TableLinePolicy: &xmlcfg.TableLineParsePolicy{
			Regex: `(?i)^(?P<mac>[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4})\s+(?P<vlan>\d+)\s+(?P<port>\S+)\s+(?P<type>\S+)`,
			Fields: []xmlcfg.FieldNode{
				{
					Name: "type",
					ToUpper: &xmlcfg.ToUpperNode{
						Name: "type",
					},
				},
			},
		},
	}

	echo := `
-------------------------------------------------------------------------------
MAC Address    VLAN/VSI/BD   Learned-From        Type
-------------------------------------------------------------------------------
00e0-fc12-3456 10            GE1/0/1             dynamic
00e0-fc12-789a 20            GE1/0/2             static
-------------------------------------------------------------------------------
`

	exec := &TableLinePolicyExecutor{}
	rows, err := exec.Execute(echo, node)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, "00e0-fc12-3456", rows[0]["mac"])
	assert.Equal(t, "10", rows[0]["vlan"])
	assert.Equal(t, "GE1/0/1", rows[0]["port"])
	assert.Equal(t, "DYNAMIC", rows[0]["type"])

	assert.Equal(t, "00e0-fc12-789a", rows[1]["mac"])
	assert.Equal(t, "20", rows[1]["vlan"])
	assert.Equal(t, "GE1/0/2", rows[1]["port"])
	assert.Equal(t, "STATIC", rows[1]["type"])
}
