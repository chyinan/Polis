// pattern: Functional Core
package recovery

func CanAuthorizeDestructiveBoundary(recoveryPackageComplete, packageSemanticClosurePassed bool) bool {
	return recoveryPackageComplete && packageSemanticClosurePassed
}
