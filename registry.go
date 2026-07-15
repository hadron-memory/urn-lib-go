package urn

// The type registry (locked, v1-parity). Ported from hadron-server src/lib/urn.ts.

// CoreTypes are the core URN types.
var CoreTypes = []string{
	"agent", "app", "app-user", "ai-config", "asset", "edge",
	"license", "memory", "node", "org", "platform", "reference",
	"session", "subscription", "usage", "user",
}

// NodeTypes are the node types.
var NodeTypes = []string{
	"abstract", "partial", "parent", "plan", "prompt", "record", "task", "review",
}

// NodeRoles are the node roles.
var NodeRoles = []string{
	"chat", "chat-message", "config", "conversation", "event", "goal", "stage",
}

// NodeParts are the node parts.
var NodeParts = []string{"condition", "data"}

// RoleMarkers are the memory-role markers (slug validation reserves their prefixes).
var RoleMarkers = []string{"system", "app-mem", "app-user", "group-mem", "priv", "anon"}

// TypeMarkers are type-marker words that may appear inside path-segments.
var TypeMarkers = map[string]bool{"app": true, "agent": true, "memory": true}

func urnTypes() []string {
	out := make([]string, 0, len(CoreTypes)+len(NodeTypes)+len(NodeRoles)+len(NodeParts))
	out = append(out, CoreTypes...)
	out = append(out, NodeTypes...)
	out = append(out, NodeRoles...)
	out = append(out, NodeParts...)
	return out
}

var roleMarkerSet = func() map[string]bool {
	m := make(map[string]bool, len(RoleMarkers))
	for _, r := range RoleMarkers {
		m[r] = true
	}
	return m
}()

// reservedSlugSet is the FR-019 reserved-word set: the type registry, "loc"
// (reserved as a slug though deprecated as a type), and the role markers —
// all lowercased.
var reservedSlugSet = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range urnTypes() {
		m[w] = true
	}
	m["loc"] = true
	for _, r := range RoleMarkers {
		m[r] = true
	}
	return m
}()
