// pattern: Functional Core
package fixture

const (
	PeerContractV1 = "api-pagination@1"
	PeerContractV2 = "api-pagination@2"
	PeerBackendV1  = "package backend\nconst Endpoint = \"GET /items\"\nfunc FetchItems() string { return \"items\" }\n"
	PeerBackendV2  = "package backend\nconst Endpoint = \"GET /items?cursor=...\"\nfunc FetchItems() string { return \"items,next_cursor\" }\n"
	PeerFrontendV1 = "package frontend\nfunc ConsumeItems(body string) string { return \"items\" }\n// contract-v1\n"
	PeerFrontendV2 = "package frontend\nfunc ConsumeItems(body string) string { return \"items,next_cursor\" }\n"
)

func ValidatePeerFrontendCandidate(content string) bool {
	return CandidateCriteriaPassed(CheckPeerFrontendCandidate(content, PeerContractSpec{Schema: `{"items":[],"next_cursor":""}`}))
}
