// pattern: Imperative Shell
package research

import "context"

type SearchBackend interface {
	Search(context.Context, string) ([]SearchCandidate, error)
}
