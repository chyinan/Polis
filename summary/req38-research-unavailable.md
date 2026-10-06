# REQ-38 research/search/fetch unavailable control plane

Slice286 adds Schema118 ResearchOperation records and fake-only product surface @19. Search and HTTPS fetch requests are bounded, normalized and persisted as `unavailable/research_backend_unavailable` with zero provider egress. The surface does not call web, MCP, HTTP or browser backends; actual source policy, retrieval and evidence binding remain future qualified work.
