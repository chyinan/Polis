package recovery

import "testing"

func TestCanAuthorizeDestructiveBoundaryRequiresSemanticClosure(t *testing.T) {
	if CanAuthorizeDestructiveBoundary(false, false) || CanAuthorizeDestructiveBoundary(true, false) || CanAuthorizeDestructiveBoundary(false, true) {
		t.Fatal("destructive boundary was authorized without both package predicates")
	}
	if !CanAuthorizeDestructiveBoundary(true, true) {
		t.Fatal("complete package with semantic closure was not authorized")
	}
}
