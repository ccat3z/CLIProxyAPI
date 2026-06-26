package config

import (
	"reflect"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	"gopkg.in/yaml.v3"
)

func TestCloneForRuntimeNil(t *testing.T) {
	var cfg *Config
	if got := cfg.CloneForRuntime(); got != nil {
		t.Fatalf("CloneForRuntime() = %#v, want nil", got)
	}
}

func TestCloneForRuntimeDeepCopiesConfig(t *testing.T) {
	cfg := sampleCloneRuntimeConfig()

	clone := cfg.CloneForRuntime()
	if clone == nil {
		t.Fatal("CloneForRuntime() = nil")
	}
	if clone == cfg {
		t.Fatal("CloneForRuntime() returned original pointer")
	}

	mutateOriginalConfig(cfg)

	if clone.APIKeys[0] != "client-key" {
		t.Fatalf("clone.APIKeys[0] = %q, want client-key", clone.APIKeys[0])
	}
	if clone.OpenAICompatibility[0].Models[0].Thinking.Levels[0] != "low" {
		t.Fatalf("clone thinking level = %q, want low", clone.OpenAICompatibility[0].Models[0].Thinking.Levels[0])
	}
	if got := clone.Payload.Default[0].Params["object"].(map[string]any)["key"]; got != "value" {
		t.Fatalf("clone payload object key = %#v, want value", got)
	}
	if got := clone.Payload.Default[0].Params["array"].([]any)[0]; got != "first" {
		t.Fatalf("clone payload array[0] = %#v, want first", got)
	}
	// The yaml.Node stored under Params must be an independent deep copy: the
	// scalar reached via both Content and the Alias anchor stays at its original
	// value after the source node is mutated.
	if got := rawNodeScalar(t, clone.Payload.Default[0].Params["node"].(yaml.Node), "mode"); got != "first" {
		t.Fatalf("clone node raw mode = %q, want first", got)
	}
	if got := rawNodeAliasScalar(t, clone.Payload.Default[0].Params["node"].(yaml.Node)); got != "first" {
		t.Fatalf("clone node alias mode = %q, want first", got)
	}

	clone.APIKeys[0] = "clone-client-key"
	clone.OpenAICompatibility[0].Models[0].Thinking.Levels[0] = "clone-low"
	clone.Payload.Default[0].Params["object"].(map[string]any)["key"] = "clone-value"
	node := clone.Payload.Default[0].Params["node"].(yaml.Node)
	setRawNodeScalar(&node, "mode", "third")
	clone.Payload.Default[0].Params["node"] = node

	if cfg.APIKeys[0] != "mutated-client-key" {
		t.Fatalf("cfg.APIKeys[0] = %q, want mutated-client-key", cfg.APIKeys[0])
	}
	if cfg.OpenAICompatibility[0].Models[0].Thinking.Levels[0] != "mutated-low" {
		t.Fatalf("cfg thinking level = %q, want mutated-low", cfg.OpenAICompatibility[0].Models[0].Thinking.Levels[0])
	}
	if got := cfg.Payload.Default[0].Params["object"].(map[string]any)["key"]; got != "mutated-value" {
		t.Fatalf("cfg payload object key = %#v, want mutated-value", got)
	}
	if got := rawNodeScalar(t, cfg.Payload.Default[0].Params["node"].(yaml.Node), "mode"); got != "second" {
		t.Fatalf("cfg node raw mode = %q, want second", got)
	}
}

func TestCloneForRuntimeDoesNotShareReferenceFields(t *testing.T) {
	cfg := sampleCloneRuntimeConfig()
	clone := cfg.CloneForRuntime()

	assertNoSharedRuntimeReferences(t, reflect.ValueOf(cfg), reflect.ValueOf(clone), "Config")
}

func sampleCloneRuntimeConfig() *Config {
	return &Config{
		SDKConfig: SDKConfig{
			APIKeys: []string{"client-key"},
			Streaming: StreamingConfig{
				KeepAliveSeconds: 3,
				BootstrapRetries: 2,
			},
		},
		OpenAICompatibility: []OpenAICompatibility{{
			Name: "compat",
			Models: []OpenAICompatibilityModel{{
				Name:     "compat-upstream",
				Alias:    "compat-client",
				Thinking: &registry.ThinkingSupport{Levels: []string{"low", "high"}},
			}},
			Headers: map[string]string{"X-Compat": "one"},
		}},
		Payload: PayloadConfig{
			Default: []PayloadRule{{
				Models: []PayloadModelRule{{
					Name:    "model-*",
					Headers: map[string]string{"X-Tier": "gold"},
					Match:   []map[string]any{{"tier": "gold"}},
					Exist:   []string{"$.messages"},
				}},
				Params: map[string]any{
					"object": map[string]any{"key": "value"},
					"array":  []any{"first", map[string]any{"nested": "value"}},
					"node":   sampleRawNode("first"),
				},
			}},
			Filter: []PayloadFilterRule{{
				Models: []PayloadModelRule{{Name: "model-*"}},
				Params: []string{"$.secret"},
			}},
		},
	}
}

func mutateOriginalConfig(cfg *Config) {
	cfg.APIKeys[0] = "mutated-client-key"
	cfg.OpenAICompatibility[0].Models[0].Thinking.Levels[0] = "mutated-low"
	cfg.Payload.Default[0].Params["object"].(map[string]any)["key"] = "mutated-value"
	node := cfg.Payload.Default[0].Params["node"].(yaml.Node)
	setRawNodeScalar(&node, "mode", "second")
	cfg.Payload.Default[0].Params["node"] = node
}

// sampleRawNode builds a yaml.Node mapping that references the same scalar
// (modeValue) both directly under "mode" and via a YAML alias under
// "mode-alias". This exercises the cyclic/aliased node handling in
// deepCopyNodeSeen through the generic map[string]any clone path.
func sampleRawNode(mode string) yaml.Node {
	modeValue := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: mode, Anchor: "modeAnchor"}
	return yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "enabled"},
			{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "mode"},
			modeValue,
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "mode-alias"},
			{Kind: yaml.AliasNode, Alias: modeValue},
		},
	}
}

func rawNodeScalar(t *testing.T, node yaml.Node, key string) string {
	t.Helper()
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i] != nil && node.Content[i].Value == key && node.Content[i+1] != nil {
			return node.Content[i+1].Value
		}
	}
	t.Fatalf("raw node missing key %q", key)
	return ""
}

// rawNodeAliasScalar resolves the value reached through the YAML alias node
// (the "mode-alias" entry), verifying the alias target was preserved.
func rawNodeAliasScalar(t *testing.T, node yaml.Node) string {
	t.Helper()
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i] != nil && node.Content[i].Value == "mode-alias" && node.Content[i+1] != nil {
			alias := node.Content[i+1].Alias
			if alias == nil {
				t.Fatalf("mode-alias entry has no Alias target")
			}
			return alias.Value
		}
	}
	t.Fatalf("raw node missing alias entry")
	return ""
}

func setRawNodeScalar(node *yaml.Node, key, value string) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i] != nil && node.Content[i].Value == key && node.Content[i+1] != nil {
			node.Content[i+1].Value = value
			return
		}
	}
}

func assertNoSharedRuntimeReferences(t *testing.T, original, clone reflect.Value, path string) {
	t.Helper()
	if !original.IsValid() || !clone.IsValid() {
		return
	}
	if original.Kind() == reflect.Interface {
		if original.IsNil() || clone.IsNil() {
			return
		}
		assertNoSharedRuntimeReferences(t, original.Elem(), clone.Elem(), path)
		return
	}
	if original.Kind() != clone.Kind() {
		t.Fatalf("%s kind mismatch: %s != %s", path, original.Kind(), clone.Kind())
	}

	switch original.Kind() {
	case reflect.Pointer:
		if original.IsNil() || clone.IsNil() {
			return
		}
		if original.Pointer() == clone.Pointer() {
			t.Fatalf("%s shares pointer %x", path, original.Pointer())
		}
		assertNoSharedRuntimeReferences(t, original.Elem(), clone.Elem(), path+"->"+original.Type().Elem().String())
	case reflect.Map:
		if original.IsNil() || clone.IsNil() {
			return
		}
		if original.Pointer() == clone.Pointer() {
			t.Fatalf("%s shares map pointer %x", path, original.Pointer())
		}
		iter := original.MapRange()
		for iter.Next() {
			key := iter.Key()
			assertNoSharedRuntimeReferences(t, iter.Value(), clone.MapIndex(key), path+"["+keyForPath(key)+"]")
		}
	case reflect.Slice:
		if original.IsNil() || clone.IsNil() {
			return
		}
		if original.Pointer() == clone.Pointer() {
			t.Fatalf("%s shares slice pointer %x", path, original.Pointer())
		}
		for i := 0; i < original.Len(); i++ {
			assertNoSharedRuntimeReferences(t, original.Index(i), clone.Index(i), path+"[]")
		}
	case reflect.Struct:
		for i := 0; i < original.NumField(); i++ {
			field := original.Type().Field(i)
			assertNoSharedRuntimeReferences(t, original.Field(i), clone.Field(i), path+"."+field.Name)
		}
	}
}

func keyForPath(key reflect.Value) string {
	if key.Kind() == reflect.String {
		return key.String()
	}
	return key.Type().String()
}
