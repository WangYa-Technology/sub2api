package service

import (
	"testing"

	pluginv1 "github.com/Wei-Shaw/sub2api/pkg/pluginapi/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluatePluginCompatibility(t *testing.T) {
	manifest := testPluginManifest(nil)
	host := PluginHostInfo{Version: "0.1.179", BuildType: "release"}

	result := EvaluatePluginCompatibility(manifest, host)
	require.True(t, result.Compatible)
	assert.True(t, result.Tested)
	assert.Equal(t, "compatible", result.Status)

	manifest.Requires.TestedSub2APIVersions = []string{"0.1.178"}
	result = EvaluatePluginCompatibility(manifest, host)
	require.True(t, result.Compatible)
	assert.False(t, result.Tested)
	assert.Equal(t, "untested", result.Status)

	manifest.Requires.Sub2API = ">=0.2.0 <0.3.0"
	result = EvaluatePluginCompatibility(manifest, host)
	assert.False(t, result.Compatible)
	assert.Equal(t, "incompatible", result.Status)
}

func TestEvaluatePluginCompatibilityRejectsProtocolMismatch(t *testing.T) {
	manifest := testPluginManifest(nil)
	manifest.Requires.PluginProtocol = pluginv1.ProtocolVersion + 1

	result := EvaluatePluginCompatibility(manifest, PluginHostInfo{Version: "0.1.179"})

	assert.False(t, result.Compatible)
	assert.Equal(t, "incompatible", result.Status)
}

func TestMatchesSemverRange(t *testing.T) {
	assert.True(t, matchesSemverRange("0.1.179", ">=0.1.170, <0.2.0"))
	assert.True(t, matchesSemverRange("v1.2.3", "=1.2.3"))
	assert.False(t, matchesSemverRange("0.1.169", ">=0.1.170 <0.2.0"))
	assert.False(t, matchesSemverRange("dev", ">=0.1.0"))
	assert.False(t, matchesSemverRange("0.1.179", "^0.1.0"))
}

func TestHCAIPluginVersionRanges(t *testing.T) {
	for _, tt := range []struct {
		version    string
		rangeExpr  string
		compatible bool
	}{
		{"0.2.7-hcai", ">=0.2.7 <0.3.0", true},
		{"v0.2.7-hcai.1", ">=0.2.7 <0.3.0", true},
		{"0.2.7-hcai.12+build.1", "=0.2.7", true},
		{"0.2.6-hcai.9", ">=0.2.7 <0.3.0", false},
		{"0.3.0-hcai", ">=0.2.7 <0.3.0", false},
		{"0.2.7-hcai.1", ">0.2.7", false},
		{"0.2.7-rc.1", ">=0.2.7", false},
		{"0.2.7-hcai.rc.1", ">=0.2.7", false},
		{"0.2.7-hcai.1.beta", ">=0.2.7", false},
		{"0.2.7-hcai-custom", ">=0.2.7", false},
		{"0.2.7-hcai.01", ">=0.2.7", false},
		{"0.2.7-hcai.1", ">=0.2.7-hcai.2", false},
		{"0.2.7-hcai.2", ">=0.2.7-hcai.2 <0.3.0", true},
		{"0.2.7", ">=0.2.7 <0.3.0", true},
	} {
		t.Run(tt.version+"/"+tt.rangeExpr, func(t *testing.T) {
			assert.Equal(t, tt.compatible, matchesSemverRange(tt.version, tt.rangeExpr))
		})
	}
}

func TestHCAICompatibilityPreservesTestedAndProtocolChecks(t *testing.T) {
	manifest := testPluginManifest(nil)
	manifest.Requires.Sub2API = ">=0.2.7 <0.3.0"
	manifest.Requires.TestedSub2APIVersions = []string{"0.2.7"}
	host := PluginHostInfo{Version: "0.2.7-hcai.1", BuildType: "release"}
	result := EvaluatePluginCompatibility(manifest, host)
	require.True(t, result.Compatible)
	assert.False(t, result.Tested)
	assert.Equal(t, "untested", result.Status)
	assert.Equal(t, host.Version, result.CurrentSub2API)

	manifest.Requires.TestedSub2APIVersions = []string{"v0.2.7-hcai.1"}
	result = EvaluatePluginCompatibility(manifest, host)
	assert.True(t, result.Tested)
	assert.Equal(t, "compatible", result.Status)

	for _, field := range []*int{&manifest.Requires.PluginProtocol, &manifest.Requires.TransportAPI, &manifest.Requires.UIBridge} {
		*field++
		result = EvaluatePluginCompatibility(manifest, host)
		assert.False(t, result.Compatible)
		assert.Equal(t, "incompatible", result.Status)
		*field--
	}
}
