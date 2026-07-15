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

// urnTypeSet is the membership set for the full type registry.
var urnTypeSet = func() map[string]bool {
	m := map[string]bool{}
	for _, t := range urnTypes() {
		m[t] = true
	}
	return m
}()

// nodeURNTypeSet — URN types that identify a node.
var nodeURNTypeSet = func() map[string]bool {
	m := map[string]bool{"node": true}
	for _, t := range NodeTypes {
		m[t] = true
	}
	for _, t := range NodeRoles {
		m[t] = true
	}
	for _, t := range NodeParts {
		m[t] = true
	}
	return m
}()

// ownerNamespacedSet — types whose owner/author namespace may be an @<handle>.
var ownerNamespacedSet = func() map[string]bool {
	m := map[string]bool{"app": true, "agent": true, "memory": true, "edge": true}
	for k := range nodeURNTypeSet {
		m[k] = true
	}
	return m
}()
